package relay

import (
	"bufio"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
)

// Relay-side slots are reused as soon as response body and local Close finish.
func TestColdAssetChurnHasNoSpuriousRejection(t *testing.T) {
	id := newTestIdentity(t)
	local := tlsTestServer(t, id, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "2")
		_, _ = io.WriteString(w, "ok")
	}), true)
	h := startPoolConnector(t, id, local.Listener.Addr().String(), nil)
	go func() {
		for {
			var m protocol.ControlMessage
			if protocol.ReadFrame(h.management, &m, protocol.MaxControlBytes) != nil {
				return
			}
			if m.Type == protocol.ControlPing {
				m.Type = protocol.ControlPong
				if protocol.WriteFrame(h.management, m, protocol.MaxControlBytes) != nil {
					return
				}
			}
		}
	}()
	var rejected atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 32 {
				stream, result := openNodeStream(t, h.bulk, protocol.StreamHTTP, h.bulk.scope)
				if result.Error != nil {
					rejected.Add(1)
					_ = stream.Close()
					continue
				}
				_, err := io.WriteString(stream, "GET /assets/test.js HTTP/1.1\r\nHost: original.example\r\nConnection: close\r\n\r\n")
				if err != nil {
					t.Error(err)
					return
				}
				response, err := http.ReadResponse(bufio.NewReader(stream), nil)
				if err != nil {
					t.Error(err)
					return
				}
				_, err = io.Copy(io.Discard, response.Body)
				if err != nil {
					t.Error(err)
					return
				}
				_ = response.Body.Close()
				_ = stream.Close()
			}
		}()
	}
	wg.Wait()
	t.Logf("requests=1024 relay_concurrency=32 rejected=%d", rejected.Load())
	if rejected.Load() != 0 {
		t.Fatalf("valid slot reuse rejected with unavailable: %d", rejected.Load())
	}
}
