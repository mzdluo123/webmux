package relay

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/xtaci/smux"
)

const streamCloseTimeout = protocol.ControlLease

func (s *connection) accept(p *physical, role protocol.Role) error {
	for {
		stream, err := p.mux.AcceptStream()
		if err != nil {
			return protocol.Error{Code: protocol.Unavailable}
		}
		// Backpressure bounds in-flight headers; it never closes a healthy
		// physical connection because completed streams are still cleaning up.
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case s.headerSlots <- struct{}{}:
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.serveStream(p, role, stream)
			}()
		}
	}
}

func (s *connection) serveStream(p *physical, role protocol.Role, stream *smux.Stream) {
	defer func() {
		s.closeStream(p, stream)
		s.unregisterHTTP(stream)
	}()
	readingHeader := true
	defer func() {
		if readingHeader {
			<-s.headerSlots
		}
	}()
	if err := stream.SetDeadline(time.Now().Add(protocol.StreamOpenTimeout)); err != nil {
		return
	}
	var header protocol.StreamHeader
	if err := protocol.ReadFrame(stream, &header, protocol.MaxStreamHeaderBytes); err != nil {
		return
	}
	if err := header.Validate(s.scope, role); err != nil {
		_ = streamResult(stream, header.RequestID, protocolCode(err))
		return
	}
	<-s.headerSlots
	readingHeader = false
	if header.Kind == protocol.StreamControl {
		s.mu.Lock()
		if p != s.control || s.controlClaimed || s.denied || s.ctx.Err() != nil {
			s.mu.Unlock()
			_ = streamResult(stream, header.RequestID, protocol.Conflict)
			return
		}
		s.controlClaimed = true
		s.mu.Unlock()
		if err := streamResult(stream, header.RequestID, ""); err != nil {
			s.stop(protocol.Error{Code: protocol.Unavailable})
			return
		}
		if err := stream.SetDeadline(time.Time{}); err != nil {
			s.stop(err)
			return
		}
		rpc := newControlRPC(s, stream)
		rpc.start()
		if !s.publishReady(rpc) {
			return
		}
		<-s.ctx.Done()
		return
	}
	if !s.registerHTTP(p, role, stream) {
		_ = streamResult(stream, header.RequestID, protocol.Unavailable)
		return
	}
	// mTLS identifies this otherwise ordinary loopback request at the
	// dedicated WebMux listener. No header can manufacture that provenance.
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: protocol.StreamOpenTimeout}, Config: s.client.config.LoopbackTLSConfig}
	dialCtx, cancel := context.WithTimeout(s.ctx, protocol.StreamOpenTimeout)
	conn, err := dialer.DialContext(dialCtx, "tcp", s.client.config.LoopbackAddress)
	cancel()
	if err != nil {
		if s.ctx.Err() == nil {
			_ = stream.SetWriteDeadline(time.Now().Add(protocol.StreamOpenTimeout))
		}
		_ = streamResult(stream, header.RequestID, protocol.UpstreamFailed)
		return
	}
	defer func() { _ = conn.Close() }()
	s.mu.Lock()
	_, attached := s.pool(role).physicals[p]
	if !attached || s.denied || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.httpStreams[stream] = httpStream{physical: p, conn: conn}
	s.mu.Unlock()
	if err := stream.SetWriteDeadline(time.Now().Add(protocol.StreamOpenTimeout)); err != nil {
		return
	}
	stop := context.AfterFunc(s.ctx, func() {
		// Interrupt reads/writes without enqueueing a potentially blocked FIN.
		abortConn(conn)
		_ = stream.SetDeadline(time.Now())
	})
	defer stop()
	if s.ctx.Err() != nil {
		return
	}
	if err := streamResult(stream, header.RequestID, ""); err != nil {
		return
	}
	if s.ctx.Err() != nil {
		return
	}
	if err := stream.SetDeadline(time.Time{}); err != nil {
		return
	}
	// A request is one raw HTTP/1.1 stream, including streaming bodies and
	// upgrade bytes. Never parse/rewrite a target or buffer a complete body.
	copyDuplex(stream, conn)
}

func (s *connection) registerHTTP(p *physical, role protocol.Role, stream *smux.Stream) bool {
	// Publication is checked under the same lock as access denial and the map.
	s.client.mu.Lock()
	active := s.client.active == s
	s.client.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	_, attached := s.pool(role).physicals[p]
	if !active || !attached || s.denied || s.ctx.Err() != nil {
		return false
	}
	s.httpStreams[stream] = httpStream{physical: p}
	return true
}

func (s *connection) unregisterHTTP(stream *smux.Stream) {
	s.mu.Lock()
	delete(s.httpStreams, stream)
	s.mu.Unlock()
}

func (s *connection) closeHTTP() {
	s.mu.Lock()
	s.denied = true
	type access struct {
		stream *smux.Stream
		conn   net.Conn
	}
	streams := make([]access, 0, len(s.httpStreams))
	for stream, state := range s.httpStreams {
		streams = append(streams, access{stream: stream, conn: state.conn})
	}
	s.mu.Unlock()
	for _, access := range streams {
		if access.conn != nil {
			abortConn(access.conn)
		}
		_ = access.stream.SetDeadline(time.Now())
	}
}

func abortConn(conn net.Conn) {
	if secure, ok := conn.(*tls.Conn); ok {
		_ = secure.NetConn().Close()
		return
	}
	_ = conn.Close()
}

// smux Close/CloseWrite have an internal FIN wait independent of stream write
// deadlines. Allow the same progress budget as data IO before closing the
// physical socket; a queued FIN behind rate-limited data is not a 1s failure.
func (s *connection) closeStream(p *physical, stream *smux.Stream) {
	done := make(chan struct{})
	go func() { _ = stream.Close(); close(done) }()
	timer := time.NewTimer(streamCloseTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-timer.C:
		slog.Warn("relay stream FIN timed out; closing data socket")
		p.close()
	case <-s.ctx.Done():
		p.close()
	}
	<-done
}

func streamResult(stream *smux.Stream, id protocol.Token, code protocol.Code) error {
	result := protocol.StreamResult{Version: protocol.V1, RequestID: id}
	if code != "" {
		result.Error = &protocol.Error{Code: code}
	}
	return protocol.WriteFrame(stream, result, protocol.MaxStreamHeaderBytes)
}

func protocolCode(err error) protocol.Code {
	if p, ok := err.(protocol.Error); ok {
		return p.Code
	}
	return protocol.Invalid
}

// idleWriter bounds a stalled write, not the lifespan or idle read of a terminal.
type idleWriter struct{ conn net.Conn }

func (w idleWriter) Write(data []byte) (int, error) {
	if err := w.conn.SetWriteDeadline(time.Now().Add(protocol.ControlLease)); err != nil {
		return 0, err
	}
	return w.conn.Write(data)
}

func copyDuplex(stream *smux.Stream, conn net.Conn) {
	// Each direction has one owner; both are joined before returning.
	done := make(chan struct{})
	go func() {
		_, err := io.Copy(idleWriter{conn: conn}, stream)
		if err == nil {
			if half, ok := conn.(interface{ CloseWrite() error }); ok {
				err = half.CloseWrite()
			}
		}
		if err != nil {
			abortConn(conn)
			_ = stream.SetDeadline(time.Now())
		}
		close(done)
	}()
	_, err := io.Copy(idleWriter{conn: stream}, conn)
	// Do not call smux CloseWrite inline: a queued FIN can ignore stream
	// deadlines. The peer's one-request stream is closed after response EOF.
	if err != nil {
		abortConn(conn)
		_ = stream.SetDeadline(time.Now())
	}
	// At response EOF, the request has completed (including upgrade closure).
	// Interrupt any unfinished request body rather than leak a reader.
	_ = stream.SetReadDeadline(time.Now())
	_ = conn.SetDeadline(time.Now())
	<-done
}
