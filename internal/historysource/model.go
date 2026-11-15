// Package historysource defines read-only native history values. It owns no
// runtime agents, SQL, mutable target state, or long-lived body storage.
package historysource

import (
	"context"
	"errors"
)

var (
	ErrUnavailable = errors.New("native history source unavailable")
	ErrChanged     = errors.New("native history source changed")
	ErrFormat      = errors.New("unsupported native history format")
	ErrLimit       = errors.New("native history resource limit")
	ErrIdentity    = errors.New("native history identity mismatch")
)

const (
	MaxBytes       = 64 << 20
	MaxRecordBytes = 4 << 20
	MaxRecords     = 100000
)

// Source identifies a native session, never an inferred match by title or cwd.
// Branch is an explicit native leaf when the format contains a message graph.
type Source struct {
	Agent, Path, NativeID, Branch, ProviderThreadID string
}

// Request.Cursor is an opaque adapter-owned frozen boundary. Empty selects the
// current complete prefix. Readers must not substitute a newer boundary.
type Request struct {
	Source Source
	Cursor string
}

type Attachment struct {
	Reference, MimeType, Name string
	Size                      int64
}

type Record struct {
	ID, NativeID, ParentID, Kind, Role, Text, CallID, NativeCallID, ToolName, Status string
	Offset, End                                                                      int64
	ByteLength                                                                       int64
	Attachments                                                                      []Attachment
}

type Gap struct {
	RecordID, Category string
	Offset             int64
}

// Document is a bounded, transient read result, not a live database mirror.
// Fingerprint identifies the source instance; Cursor selects its frozen range.
// ParserVersion is independent of the CLI, connector and injection versions.
type Document struct {
	Source                                                        Source
	FormatVersion, ParserVersion, Generation, Fingerprint, Cursor string
	Records                                                       []Record
	RecordRefs                                                    []RecordRef
	References                                                    []ContextReference
	Gaps                                                          []Gap
	Pending                                                       bool
}

// ReadFunc is the only capability passed to native-history orchestration.
// Implementations must not launch an agent, mutate a source, or return reasoning,
// credentials, encrypted state, or unverified attachment bodies.
type ReadFunc func(context.Context, Request) (Document, error)
