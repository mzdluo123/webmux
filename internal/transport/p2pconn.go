package transport

import (
	"context"
	"fmt"

	"github.com/rainchan/webmux/internal/hub"
	"github.com/rainchan/webmux/internal/streamproto"
)

type p2pStreamConn struct {
	c streamproto.StreamConn
}

var _ hub.MessageConn = (*p2pStreamConn)(nil)

// NewP2PStreamConn adapts a direct stream to the core message boundary.
func NewP2PStreamConn(c streamproto.StreamConn) hub.MessageConn {
	if c == nil {
		return nil
	}
	return &p2pStreamConn{c: c}
}

func (p *p2pStreamConn) Read(ctx context.Context) (hub.MessageType, []byte, error) {
	typ, data, err := p.c.Read(ctx)
	if err != nil {
		return 0, nil, err
	}
	switch typ {
	case streamproto.MessageText:
		return hub.MessageText, data, nil
	case streamproto.MessageBinary:
		return hub.MessageBinary, data, nil
	default:
		return 0, nil, fmt.Errorf("unsupported p2p message type %v", typ)
	}
}

func (p *p2pStreamConn) Write(ctx context.Context, typ hub.MessageType, data []byte) error {
	var p2pType streamproto.MessageType
	switch typ {
	case hub.MessageText:
		p2pType = streamproto.MessageText
	case hub.MessageBinary:
		p2pType = streamproto.MessageBinary
	default:
		return fmt.Errorf("unsupported message type %v", typ)
	}
	return p.c.Write(ctx, p2pType, data)
}

func (p *p2pStreamConn) Close(code hub.StatusCode, reason string) error {
	return p.c.Close(uint16(code), reason)
}

func (p *p2pStreamConn) CloseNow() error {
	return p.c.CloseNow()
}

func (p *p2pStreamConn) SetReadLimit(n int64) {
	p.c.SetReadLimit(n)
}

func (p *p2pStreamConn) CloseRead(ctx context.Context) context.Context {
	return p.c.CloseRead(ctx)
}
