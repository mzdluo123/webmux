package historysource

// Binding associates a durable provider identity with a read-only native source.
// An empty path requires discovery by the exact native session ID; cwd and titles
// are never identity evidence. Fork prefixes are separate immutable snapshots.
type Binding struct {
	ConversationID, ProviderThreadID string
	Root                             string
	Source                           Source
}
