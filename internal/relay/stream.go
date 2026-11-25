package relay

import (
	"errors"
	"time"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/xtaci/smux"
)

func (s *connection) acceptControl() error {
	claimed := false
	for {
		stream, err := s.control.mux.AcceptStream()
		if err != nil {
			return protocol.Error{Code: protocol.Unavailable}
		}
		if !s.openControl(stream, claimed) {
			_ = stream.Close()
			continue
		}
		claimed = true
	}
}

func (s *connection) openControl(stream *smux.Stream, claimed bool) bool {
	if err := stream.SetDeadline(time.Now().Add(protocol.StreamOpenTimeout)); err != nil {
		return false
	}
	var header protocol.StreamHeader
	if err := protocol.ReadFrame(stream, &header, protocol.MaxStreamHeaderBytes); err != nil {
		return false
	}
	if err := header.Validate(s.scope, protocol.RoleControl); err != nil {
		_ = streamResult(stream, header.RequestID, protocolCode(err))
		return false
	}
	if claimed {
		_ = streamResult(stream, header.RequestID, protocol.Conflict)
		return false
	}
	if streamResult(stream, header.RequestID, "") != nil || stream.SetDeadline(time.Time{}) != nil {
		return false
	}
	rpc := newControlRPC(s, stream)
	rpc.start()
	return s.publishReady(rpc)
}

func streamResult(stream *smux.Stream, id protocol.Token, code protocol.Code) error {
	result := protocol.StreamResult{Version: protocol.V1, RequestID: id}
	if code != "" {
		result.Error = &protocol.Error{Code: code}
	}
	return protocol.WriteFrame(stream, result, protocol.MaxStreamHeaderBytes)
}

func protocolCode(err error) protocol.Code {
	var p protocol.Error
	if errors.As(err, &p) {
		return p.Code
	}
	return protocol.Invalid
}
