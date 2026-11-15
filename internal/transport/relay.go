package transport

import "context"

// The key is private: only the dedicated, mutually authenticated loopback
// listener may establish this transport identity. HTTP headers never do.
type relayTransportKey struct{}

func TrustedRelayContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, relayTransportKey{}, true)
}

func IsTrustedRelay(ctx context.Context) bool {
	trusted, _ := ctx.Value(relayTransportKey{}).(bool)
	return trusted
}

type relayOwnerKey struct{}

// RelayOwnerContext is set only after owner cookie authentication.
func RelayOwnerContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, relayOwnerKey{}, true)
}

func IsRelayOwner(ctx context.Context) bool {
	owner, _ := ctx.Value(relayOwnerKey{}).(bool)
	return owner
}
