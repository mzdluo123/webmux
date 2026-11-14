// Package streamproto defines transport-neutral message streams and RPC
// envelopes. Implementations may use WebSocket, WebRTC or a local connection.
package streamproto

import "context"

type MessageType int

const (
	MessageText MessageType = iota + 1
	MessageBinary
)

type StreamConn interface {
	Read(context.Context) (MessageType, []byte, error)
	Write(context.Context, MessageType, []byte) error
	Close(code uint16, reason string) error
	CloseNow() error
	SetReadLimit(int64)
	CloseRead(context.Context) context.Context
}

type StreamHandler func(context.Context, string, StreamConn)

type RPCRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

type RPCResponse struct {
	Status     int
	StatusText string
	Headers    map[string]string
	Body       []byte
}

type RPCHandler func(context.Context, RPCRequest) RPCResponse
