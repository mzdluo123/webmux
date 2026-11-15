package historysource

// ContextReference identifies an acknowledged native record by native identity,
// role and exact transmitted digest. Text equality by itself is never evidence.
// A nil UserText marks a history-only native seed; a non-nil value is the user's
// separate current request from an inline dispatch.
type ContextReference struct {
	NativeID, Role, TextHash, Policy string
	RecordID, Key                    string
	UserText                         *string
	Originals                        []RecordRef
}
