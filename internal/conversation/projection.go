package conversation

// HistoricalMessage is the portable projection used by selection and handoff.
// Exportable=false is explicit for control events and unsupported private kinds;
// callers must never reinterpret those records as user instructions.
type HistoricalMessage struct {
	ActivityID, ConversationID, ProviderThreadID, RunID string
	NativeID, ParentID, CallID, NativeCallID, ToolName  string
	Position, RunOrdinal                                int64
	Kind, Role, Status, Text, Source                    string
	Exportable                                          bool
	ExclusionReason                                     string
	Attachments                                         []ActivityAttachment
}

// Candidate contains a bounded mechanical projection and its transport cost.
type Candidate struct {
	Message               HistoricalMessage
	ByteLength, JSONBytes int64
}
