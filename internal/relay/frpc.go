package relay

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/frphelper"
)

const frpcConfig = `serverAddr = "127.0.0.1"
serverPort = %d
loginFailExit = true
log.to = "console"
log.level = "info"
transport.protocol = "websocket"
transport.tls.enable = false
transport.tcpMux = false
transport.poolCount = 0
transport.heartbeatInterval = 10
transport.heartbeatTimeout = 30
transport.dialServerTimeout = 5
auth.method = "oidc"
auth.additionalScopes = ["HeartBeats", "NewWorkConns"]
auth.oidc.tokenSource.type = "file"
auth.oidc.tokenSource.file.path = %q
[[proxies]]
name = %q
type = "tcp"
localIP = "127.0.0.1"
localPort = %d
remotePort = %d
`

var configureFRPCParentDeath func(*exec.Cmd)

type frpRuntime struct {
	access      *frpAccess
	originUntil time.Time
	bridge      *frpBridge
	cancel      context.CancelFunc
	bridgeLn    *frpListener
	originLn    *frpListener
	bridgeHTTP  *http.Server
	originHTTP  *http.Server
	exited      chan struct{}
	tokenFile   string
}

func (s *connection) startFRP(ctx context.Context, dir, executable string, key *ecdsa.PrivateKey, grant protocol.AllocationGrant) (_ *frpRuntime, err error) {
	if !time.Now().Before(time.Unix(grant.ExpiresAt, 0)) {
		return nil, protocol.Error{Code: protocol.Expired}
	}
	ctx, cancel := context.WithCancel(ctx)
	r := &frpRuntime{access: &frpAccess{ctx: ctx}, cancel: cancel}
	defer func() {
		if err != nil {
			r.close()
		}
	}()
	secure, expires, err := originTLS(s.scope, key, grant, r.access)
	if err != nil {
		return nil, err
	}
	r.originUntil = expires
	r.access.grant.Store(time.Unix(grant.ExpiresAt, 0).UnixNano())
	r.bridgeLn, err = listenFRP(frpBridgeSockets)
	if err != nil {
		return nil, protocol.Error{Code: protocol.Unavailable}
	}
	r.originLn, err = listenFRP(frpBridgeSockets)
	if err != nil {
		return nil, protocol.Error{Code: protocol.Unavailable}
	}
	token := filepath.Join(dir, "token")
	if err := replaceFRPToken(token, grant.JWT); err != nil {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	r.tokenFile = token
	configPath := filepath.Join(dir, "frpc.toml")
	bridgePort := r.bridgeLn.Addr().(*net.TCPAddr).Port
	originPort := r.originLn.Addr().(*net.TCPAddr).Port
	cfg := fmt.Sprintf(frpcConfig, bridgePort, token, grant.ProxyName, originPort, grant.RemotePort)
	if err := os.WriteFile(configPath, []byte(cfg), 0600); err != nil {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	r.bridge = newFRPBridge(s.client, &s.assignment.Trust, grant, r.access, r.bridgeLn.Addr().String())
	server := func(handler http.Handler) *http.Server {
		return &http.Server{Handler: handler, BaseContext: func(net.Listener) context.Context { return ctx },
			ReadHeaderTimeout: protocol.AuthTimeout, IdleTimeout: protocol.ControlLease, MaxHeaderBytes: 32 << 10,
			ErrorLog: log.New(io.Discard, "", 0)}
	}
	r.bridgeHTTP = server(r.bridgeLn.track(r.bridge))
	r.originHTTP = server(originHandler(s.assignment.Binding.PublicURL, s.callbacks.Origin, r.access, expires))
	r.originHTTP.ConnContext = originConnContext
	r.originHTTP.TLSConfig = secure
	r.originHTTP.SetKeepAlivesEnabled(false)
	go func() { _ = r.bridgeHTTP.Serve(r.bridgeLn) }()
	go func() { _ = r.originHTTP.Serve(tls.NewListener(r.originLn, secure)) }()

	if err := frphelper.Verify(executable); err != nil {
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	cmd := exec.CommandContext(ctx, executable, "-c", configPath)
	cmd.Dir = dir
	cmd.Env = []string{}
	// Keep stdout on a parent-owned pipe and discard its contents. Darwin
	// lacks a parent-death signal; if WebMux crashes the closed bridge forces
	// stock frpc to reconnect, and its next console log exits on SIGPIPE.
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if configureFRPCParentDeath != nil {
		configureFRPCParentDeath(cmd)
	}
	// Linux's parent-death signal is tied to the spawning thread. Keep that
	// thread alive until Wait has reaped the child.
	started := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		if configureFRPCParentDeath != nil {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
		}
		defer close(exited)
		err := cmd.Start()
		started <- err
		if err == nil {
			_ = cmd.Wait()
		}
	}()
	if err := <-started; err != nil {
		<-exited
		return nil, protocol.Error{Code: protocol.InvalidConfig}
	}
	r.exited = exited
	return r, nil
}

func (r *frpRuntime) close() {
	r.access.end()
	r.cancel()
	if r.originLn != nil {
		_ = r.originLn.Close()
	}
	if r.bridgeLn != nil {
		_ = r.bridgeLn.Close()
	}
	if r.originHTTP != nil {
		_ = r.originHTTP.Close()
	}
	if r.bridgeHTTP != nil {
		_ = r.bridgeHTTP.Close()
		r.bridgeLn.work.Wait()
		r.bridge.transport.CloseIdleConnections()
	}
	if r.exited != nil {
		<-r.exited
	}
	if r.tokenFile != "" {
		if err := os.Remove(r.tokenFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("relay could not remove the retired frpc token file")
		}
	}
}

func replaceFRPToken(path, token string) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), "token-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			if cleanup := os.Remove(tmp); cleanup != nil && !errors.Is(cleanup, os.ErrNotExist) {
				err = errors.Join(err, cleanup)
			}
		}
	}()
	_, err = io.WriteString(f, token)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}
