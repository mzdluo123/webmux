//go:build windows

package traysys

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")

	// CLSID_NetworkListManager {DCB00C01-570F-4A9B-8D69-199FDBA5723B}
	clsidNetworkListManager = windows.GUID{
		Data1: 0xdcb00c01,
		Data2: 0x570f,
		Data3: 0x4a9b,
		Data4: [8]byte{0x8d, 0x69, 0x19, 0x9f, 0xdb, 0xa5, 0x72, 0x3b},
	}
	// IID_INetworkListManager {DCB00000-570F-4A9B-8D69-199FDBA5723B}
	iidINetworkListManager = windows.GUID{
		Data1: 0xdcb00000,
		Data2: 0x570f,
		Data3: 0x4a9b,
		Data4: [8]byte{0x8d, 0x69, 0x19, 0x9f, 0xdb, 0xa5, 0x72, 0x3b},
	}
)

const (
	clsctxAll        = 23 // CLSCTX_INPROC_SERVER | CLSCTX_INPROC_HANDLER | CLSCTX_LOCAL_SERVER | CLSCTX_REMOTE_SERVER
	rpcEChangedMode  = 0x80010106
	nlmCatPublic     = 0
	nlmCatPrivate    = 1
	nlmCatDomainAuth = 2
)

// comObject represents a C COM interface pointer whose first word points to
// its method table.
type comObject struct {
	vtbl *[32]uintptr
}

// AdapterCategories maps each active Windows network interface's FriendlyName
// (matching net.Interface.Name) to its Network List Manager category:
// "public", "private", or "domain". It requires no administrator rights and
// spawns no subprocesses.
func AdapterCategories() map[string]string {
	byGUID, err := nlmCategoriesByAdapterGUID()
	if err != nil || len(byGUID) == 0 {
		return nil
	}
	names := adapterFriendlyNames()
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]string, len(byGUID))
	for guid, cat := range byGUID {
		if name := names[guid]; name != "" {
			out[name] = cat
		}
	}
	return out
}

func nlmCategoriesByAdapterGUID() (map[windows.GUID]string, error) {
	hr, _, _ := procCoInitializeEx.Call(0, windows.COINIT_MULTITHREADED)
	switch uint32(hr) {
	case 0, 1: // S_OK, S_FALSE
		defer func() { _, _, _ = procCoUninitialize.Call() }()
	case rpcEChangedMode:
		// Thread already initialized COM in apartment-threaded mode; NLM supports both.
	default:
		return nil, syscall.Errno(hr)
	}

	var mgr *comObject
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidNetworkListManager)),
		0,
		clsctxAll,
		uintptr(unsafe.Pointer(&iidINetworkListManager)),
		uintptr(unsafe.Pointer(&mgr)),
	)
	if int32(hr) < 0 || mgr == nil {
		return nil, errors.New("CoCreateInstance INetworkListManager failed")
	}
	defer mgr.release()

	// INetworkListManager::GetNetworkConnections is slot 9 (IUnknown 0..2, IDispatch 3..6,
	// GetNetworks=7, GetNetwork=8, GetNetworkConnections=9).
	var enumConn *comObject
	hr = mgr.call1(9, uintptr(unsafe.Pointer(&enumConn)))
	if int32(hr) < 0 || enumConn == nil {
		return nil, errors.New("GetNetworkConnections failed")
	}
	defer enumConn.release()

	out := map[windows.GUID]string{}
	for range 256 {
		// IEnumNetworkConnections::Next is slot 8 (_NewEnum=7, Next=8).
		var conn *comObject
		var fetched uint32
		hr = enumConn.call3(8, 1, uintptr(unsafe.Pointer(&conn)), uintptr(unsafe.Pointer(&fetched)))
		if hr != 0 || fetched == 0 || conn == nil {
			conn.release()
			break
		}
		cat, guid, ok := inspectNetworkConnection(conn)
		conn.release()
		if ok && cat != "" {
			// If an adapter has multiple connections, prefer the stricter Public classification.
			if prev, exists := out[guid]; !exists || cat == "public" || prev == "" {
				out[guid] = cat
			}
		}
	}
	return out, nil
}

func inspectNetworkConnection(conn *comObject) (string, windows.GUID, bool) {
	// INetworkConnection::GetAdapterId is slot 12 (GetNetwork=7, IsConnectedToInternet=8,
	// IsConnected=9, GetConnectivity=10, GetConnectionId=11, GetAdapterId=12).
	var adapterID windows.GUID
	if int32(conn.call1(12, uintptr(unsafe.Pointer(&adapterID)))) < 0 {
		return "", windows.GUID{}, false
	}
	// INetworkConnection::GetNetwork is slot 7.
	var netObj *comObject
	if int32(conn.call1(7, uintptr(unsafe.Pointer(&netObj)))) < 0 || netObj == nil {
		return "", windows.GUID{}, false
	}
	defer netObj.release()

	// INetwork::GetCategory is slot 18 (IUnknown 0..2, IDispatch 3..6,
	// GetName=7..GetConnectivity=17, GetCategory=18).
	var rawCat int32
	if int32(netObj.call1(18, uintptr(unsafe.Pointer(&rawCat)))) < 0 {
		return "", windows.GUID{}, false
	}
	switch rawCat {
	case nlmCatPublic:
		return "public", adapterID, true
	case nlmCatPrivate:
		return "private", adapterID, true
	case nlmCatDomainAuth:
		return "domain", adapterID, true
	default:
		return "", adapterID, false
	}
}

func (o *comObject) release() {
	if o == nil || o.vtbl == nil {
		return
	}
	_, _, _ = syscall.SyscallN(o.vtbl[2], uintptr(unsafe.Pointer(o)))
}

func (o *comObject) call1(index int, a1 uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[index], uintptr(unsafe.Pointer(o)), a1)
	return r
}

func (o *comObject) call3(index int, a1, a2, a3 uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[index], uintptr(unsafe.Pointer(o)), a1, a2, a3)
	return r
}

func adapterFriendlyNames() map[windows.GUID]string {
	const flags = windows.GAA_FLAG_SKIP_ANYCAST | windows.GAA_FLAG_SKIP_MULTICAST | windows.GAA_FLAG_SKIP_DNS_SERVER
	size := uint32(16 * 1024)
	var buf []byte
	for range 4 {
		buf = make([]byte, size)
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			return nil
		}
	}
	if len(buf) == 0 {
		return nil
	}
	out := map[windows.GUID]string{}
	for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); aa != nil; aa = aa.Next {
		if aa.AdapterName == nil || aa.FriendlyName == nil {
			continue
		}
		guidStr := windows.BytePtrToString(aa.AdapterName)
		guid, err := windows.GUIDFromString(guidStr)
		if err != nil {
			continue
		}
		name := windows.UTF16PtrToString(aa.FriendlyName)
		if name != "" {
			out[guid] = name
		}
	}
	return out
}
