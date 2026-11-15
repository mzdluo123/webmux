package historysource

import (
	"crypto/sha256"
	"encoding/hex"
)

// ContextState distinguishes a native context instance and its last observed
// compaction. It is evidence for reusing dispatch facts, never a copy of history.
type ContextState struct {
	Key, NativeID, HeadID, HeadHash string
	Compacted                       bool
}

func (s Snapshot) Context() ContextState {
	out := ContextState{NativeID: s.Source.NativeID}
	if s.Source.NativeID == "" || s.Generation == "" {
		return out
	}
	var compaction string
	for _, record := range s.Records {
		if record.Kind == "compaction" {
			compaction = record.Hash
			out.Compacted = true
		}
		out.HeadID, out.HeadHash = record.ID, record.Hash
	}
	sum := sha256.Sum256([]byte(s.Source.Agent + "\x00" + s.Source.NativeID + "\x00" + s.Generation + "\x00" + compaction))
	out.Key = hex.EncodeToString(sum[:])
	return out
}

type DispatchScope struct {
	ExecutionID, ConversationID, ProviderThreadID, NativeID, BeforeContextKey string
}
