package historysource

// Snapshot fixes source identity, record hashes and a native read boundary.
type Snapshot struct {
	ID                                                                         string
	Source                                                                     Source
	Cursor, Generation, Fingerprint, FormatVersion, ParserVersion, ContentHash string
	RecordCount                                                                int
	Gaps                                                                       []Gap
	Records                                                                    []RecordRef
}

// RecordRef is immutable selection/coverage metadata, never a body mirror.
type RecordRef struct {
	ID, Hash, Kind string
	Position       int
}

type PageRequest struct {
	After      int
	TextOffset int
	Limit      int
	MaxBytes   int
}

type Page struct {
	Snapshot   Snapshot
	Records    []Record
	Next       int
	TextOffset int
	More       bool
}
