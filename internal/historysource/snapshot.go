package historysource

// Snapshot describes one frozen native range. Reference snapshots own no body;
// retained snapshots own one immutable export, never a continuously updated log.
type Snapshot struct {
	ID                                                                         string
	Source                                                                     Source
	Cursor, Fingerprint, FormatVersion, ParserVersion, ContentHash, ExportHash string
	RecordCount                                                                int
	Gaps                                                                       []Gap
	Records                                                                    []RecordRef
	Retained                                                                   bool
}

// RecordRef is immutable selection/coverage metadata, never a body mirror.
type RecordRef struct {
	ID, Hash string
	Position int
}

// Export is materialized only for an explicitly retained snapshot. Attachments
// are keyed by their original safe reference, with bytes captured at retention.
type Export struct {
	Records     []Record
	Attachments map[string][]byte
}

type SnapshotData struct {
	Snapshot Snapshot
	Export   *Export
}

type PageRequest struct {
	After      int
	TextOffset int
	Limit      int
	MaxBytes   int
}

type Page struct {
	Records    []Record
	Next       int
	TextOffset int
	More       bool
}
