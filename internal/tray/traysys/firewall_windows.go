//go:build windows

package traysys

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW = shell32.NewProc("ShellExecuteExW")
)

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskFlagNoUI       = 0x00000400
	seeMaskNoConsole      = 0x00008000

	localFirewallPolicyKey = `SYSTEM\CurrentControlSet\Services\SharedAccess\Parameters\FirewallPolicy`
	localFirewallRulesKey  = localFirewallPolicyKey + `\FirewallRules`
	gpoFirewallPolicyKey   = `SOFTWARE\Policies\Microsoft\WindowsFirewall`
	gpoFirewallRulesKey    = gpoFirewallPolicyKey + `\FirewallRules`
	mdmFirewallRulesKey    = localFirewallPolicyKey + `\Mdm\FirewallRules`
)

// shellExecuteInfo is SHELLEXECUTEINFOW; Go's natural alignment matches the
// Windows layout on every supported architecture.
type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	hwnd       windows.HWND
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instApp    windows.Handle
	idList     uintptr
	class      *uint16
	keyClass   windows.Handle
	hotKey     uint32
	iconOrMon  windows.Handle
	process    windows.Handle
}

// InspectFirewall reads Windows Defender Firewall rules and profile settings
// for exe from HKLM without administrator rights or console subprocesses.
func InspectFirewall(exe string) (FirewallStatus, error) {
	if exe == "" {
		return FirewallStatus{}, errors.New("empty executable path")
	}
	absExe, err := filepath.Abs(exe)
	if err != nil {
		return FirewallStatus{}, err
	}
	localRules, err := readRuleStrings(localFirewallRulesKey, true)
	if err != nil {
		return FirewallStatus{}, err
	}
	gpoRules, _ := readRuleStrings(gpoFirewallRulesKey, false)
	mdmRules, _ := readRuleStrings(mdmFirewallRulesKey, false)
	policyRules := append(gpoRules, mdmRules...)
	profiles := readFirewallProfiles()
	return EvaluateFirewall(
		FirewallRule{Exe: absExe},
		localRules,
		policyRules,
		profiles,
		func(app, target string) bool {
			return SamePath(expandEnvWindows(app), target)
		},
	), nil
}

// ApplyFirewall configures or removes the Windows Defender Firewall rule for
// exe, requesting administrator elevation at most once when changes are needed.
// For an installed copy, it also syncs HKCU\...\Uninstall\WebMux\FirewallRule.
func ApplyFirewall(exe string, allow, clearBlocks bool) int {
	if exe == "" {
		return FirewallFailed
	}
	absExe, err := filepath.Abs(exe)
	if err != nil {
		return FirewallFailed
	}
	var args []string
	if allow {
		args = []string{"allow"}
		if clearBlocks {
			args = append(args, "--clear-blocks")
		}
	} else {
		args = []string{"remove", absExe}
	}
	return runFirewall(args, absExe)
}

// FirewallCommand runs `webmux-tray firewall allow [--clear-blocks] | remove [PATH]`
// and returns its exit code.
func FirewallCommand(args []string) int {
	selfExe, err := currentExecutable()
	if err != nil {
		return FirewallFailed
	}
	return runFirewall(args, selfExe)
}

func runFirewall(args []string, selfExe string) int {
	req, err := ParseFirewallRequest(args)
	if err != nil {
		return FirewallUsage
	}
	targetExe := req.Program
	if targetExe == "" {
		targetExe = selfExe
	}
	targetExe, err = filepath.Abs(targetExe)
	if err != nil {
		return FirewallFailed
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return FirewallFailed
	}
	netsh := filepath.Join(system, "netsh.exe")
	alreadyElevated := req.Elevated || isProcessElevated()

	fw := Firewall{
		Rule:    FirewallRule{Exe: targetExe},
		Inspect: func() (FirewallStatus, error) { return InspectFirewall(targetExe) },
		Run: func(cmds [][]string) (int, error) {
			if alreadyElevated {
				return runNetshBatch(netsh, cmds)
			}
			childArgs := []string{"firewall"}
			if req.Allow {
				childArgs = append(childArgs, "allow")
				if req.ClearBlocks {
					childArgs = append(childArgs, "--clear-blocks")
				}
			} else {
				childArgs = append(childArgs, "remove")
				if req.Program != "" {
					childArgs = append(childArgs, targetExe)
				}
			}
			childArgs = append(childArgs, "--elevated")
			return runElevated(selfExe, childArgs)
		},
	}
	code := fw.Apply(req.Allow, req.ClearBlocks)
	if code == FirewallDone && !req.Elevated {
		SyncFirewallRecord(targetExe, req.Allow)
	}
	return code
}

func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

func isProcessElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func runNetshBatch(netsh string, cmds [][]string) (int, error) {
	for _, args := range cmds {
		cmd := exec.Command(netsh, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
		err := cmd.Run()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		if err != nil {
			return 0, err
		}
	}
	return 0, nil
}

func readRuleStrings(subkey string, requiredParent bool) ([]string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, subkey, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil, nil
		}
		if !requiredParent {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = k.Close() }()
	names, err := k.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		val, _, err := k.GetStringValue(name)
		if err == nil && val != "" {
			out = append(out, val)
		}
	}
	return out, nil
}

func readFirewallProfiles() FirewallProfiles {
	return FirewallProfiles{
		Domain:  readProfilePolicy("DomainProfile"),
		Private: readProfilePolicy("StandardProfile"),
		Public:  readProfilePolicy("PublicProfile"),
	}
}

func readProfilePolicy(profileKey string) FirewallProfilePolicy {
	enabled := readProfileDWORD(gpoFirewallPolicyKey+`\`+profileKey, "EnableFirewall",
		readProfileDWORD(localFirewallPolicyKey+`\`+profileKey, "EnableFirewall", 1))
	shielded := readProfileDWORD(gpoFirewallPolicyKey+`\`+profileKey, "DoNotAllowExceptions",
		readProfileDWORD(localFirewallPolicyKey+`\`+profileKey, "DoNotAllowExceptions", 0))
	if enabled == 0 {
		return FirewallProfilePolicy{Disabled: true}
	}
	return FirewallProfilePolicy{BlockAllInbound: shielded == 1}
}

func readProfileDWORD(subkey, valueName string, fallback uint64) uint64 {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, subkey, registry.QUERY_VALUE)
	if err != nil {
		return fallback
	}
	defer func() { _ = k.Close() }()
	v, _, err := k.GetIntegerValue(valueName)
	if err != nil {
		return fallback
	}
	return v
}

func expandEnvWindows(s string) string {
	if strings.IndexByte(s, '%') < 0 {
		return s
	}
	src, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return s
	}
	n, err := windows.ExpandEnvironmentStrings(src, nil, 0)
	if err != nil || n == 0 {
		return s
	}
	buf := make([]uint16, n)
	if _, err := windows.ExpandEnvironmentStrings(src, &buf[0], n); err != nil {
		return s
	}
	return windows.UTF16ToString(buf)
}

// runElevated starts file with args through the "runas" verb, which shows
// the administrator prompt, waits for it and returns its exit code.
func runElevated(file string, args []string) (int, error) {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = syscall.EscapeArg(a)
	}
	info := shellExecuteInfo{
		mask:       seeMaskNoCloseProcess | seeMaskFlagNoUI | seeMaskNoConsole,
		verb:       utf16Ptr("runas"),
		file:       utf16Ptr(file),
		parameters: utf16Ptr(strings.Join(quoted, " ")),
		show:       windows.SW_HIDE,
	}
	info.size = uint32(unsafe.Sizeof(info))
	r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if err := lastError(r, callErr); err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return 0, ErrElevationDeclined
		}
		return 0, err
	}
	if info.process == 0 {
		return 0, errors.New("runas started no process")
	}
	defer func() { _ = windows.CloseHandle(info.process) }()
	if _, err := windows.WaitForSingleObject(info.process, windows.INFINITE); err != nil {
		return 0, err
	}
	var code uint32
	if err := windows.GetExitCodeProcess(info.process, &code); err != nil {
		return 0, err
	}
	return int(code), nil
}

func utf16Ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		p, _ = windows.UTF16PtrFromString("")
	}
	return p
}

func lastError(r uintptr, err error) error {
	if r != 0 {
		return nil
	}
	if errno, ok := err.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	return syscall.EINVAL
}
