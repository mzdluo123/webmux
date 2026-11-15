// Package historyimport orchestrates native reads and frozen exports. Despite
// its historical name, it never imports live source bodies into Activity.
package historyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/rainchan/webmux/internal/historysource"
)

type Store interface {
	HistoryEnabled() bool
	SaveFrozen(context.Context, historysource.SnapshotData, string) (historysource.Snapshot, error)
	Frozen(context.Context, string) (historysource.SnapshotData, error)
	RetainFrozen(context.Context, string, string) error
	ReleaseFrozen(context.Context, string, string) error
	NativeBindings(context.Context, string) ([]historysource.Binding, error)
	BindNativeSource(context.Context, string, historysource.Source) error
	PrefixSnapshots(context.Context, string) ([]string, error)
	SourceAllowed(context.Context, historysource.Source) error
	DispatchSourceScope(context.Context, string) (historysource.DispatchScope, error)
	RecordSourceContext(context.Context, historysource.DispatchScope, historysource.ContextState) error
	NativeContextReferences(context.Context, historysource.Source) ([]historysource.ContextReference, error)
	ImportFrozenSource(context.Context, string, string) (historysource.Binding, error)
}

type DiscoverFunc func(context.Context, string) ([]historysource.Source, error)

type Options struct {
	Store       Store
	Readers     map[string]historysource.ReadFunc
	Discoverers map[string]DiscoverFunc
	Roots       map[string]string
}

type Service struct {
	store       Store
	readers     map[string]historysource.ReadFunc
	discoverers map[string]DiscoverFunc
	roots       map[string]string
}

func New(o Options) *Service {
	s := &Service{store: o.Store, readers: make(map[string]historysource.ReadFunc, len(o.Readers)), discoverers: make(map[string]DiscoverFunc, len(o.Discoverers)), roots: make(map[string]string, len(o.Roots))}
	for name, reader := range o.Readers {
		if reader != nil {
			s.readers[name] = reader
		}
	}
	for name, discover := range o.Discoverers {
		if discover != nil {
			s.discoverers[name] = discover
		}
	}
	for name, root := range o.Roots {
		s.roots[name] = root
	}
	return s
}

func (s *Service) enabled() bool { return s != nil && s.store != nil && s.store.HistoryEnabled() }

func (s *Service) Discover(ctx context.Context, agent, root string) ([]historysource.Source, error) {
	if !s.enabled() {
		return nil, historysource.ErrUnavailable
	}
	discover := s.discoverers[agent]
	if discover == nil {
		return nil, historysource.ErrFormat
	}
	return discover(ctx, root)
}

func (s *Service) read(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	var empty historysource.Document
	if !s.enabled() {
		return empty, historysource.ErrUnavailable
	}
	reader := s.readers[req.Source.Agent]
	if reader == nil {
		return empty, historysource.ErrFormat
	}
	if req.Source.NativeID == "" || req.Source.Path == "" {
		return empty, historysource.ErrIdentity
	}
	doc, err := reader(ctx, req)
	if err != nil {
		return empty, err
	}
	if doc.Source.Agent != req.Source.Agent || doc.Source.NativeID != req.Source.NativeID || doc.Source.Path != req.Source.Path || (req.Source.Branch != "" && doc.Source.Branch != req.Source.Branch) {
		return empty, historysource.ErrIdentity
	}
	if len(doc.Records) > historysource.MaxRecords || doc.ParserVersion == "" || doc.Fingerprint == "" || doc.Cursor == "" {
		return empty, historysource.ErrFormat
	}
	ids := make(map[string]bool, len(doc.Records))
	total := 0
	prefix := doc.Source.Agent + ":" + doc.Source.NativeID + ":" + doc.Generation + ":"
	for index := range doc.Records {
		record := &doc.Records[index]
		if record.ID == "" || ids[record.ID] {
			return empty, historysource.ErrFormat
		}
		ids[record.ID] = true
		total += len(record.Text)
		if total > historysource.MaxBytes {
			return empty, historysource.ErrLimit
		}
		record.ID = sourceRecordID(prefix, record.ID)
		if record.ParentID != "" {
			record.ParentID = sourceRecordID(prefix, record.ParentID)
		}
		if record.CallID != "" {
			record.NativeCallID = record.CallID
			record.CallID = prefix + record.CallID
		}
	}
	return doc, nil
}

func sourceRecordID(prefix, id string) string {
	sum := sha256.Sum256([]byte(prefix + id))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Freeze(ctx context.Context, source historysource.Source, owner string, retain bool) (historysource.Snapshot, error) {
	var empty historysource.Snapshot
	if owner == "" {
		return empty, historysource.ErrIdentity
	}
	if !s.enabled() {
		return empty, historysource.ErrUnavailable
	}
	if err := s.store.SourceAllowed(ctx, source); err != nil {
		return empty, err
	}
	doc, err := s.read(ctx, historysource.Request{Source: source})
	if err != nil {
		return empty, err
	}
	return s.freezeDocument(ctx, doc, owner, retain)
}

func (s *Service) freezeDocument(ctx context.Context, doc historysource.Document, owner string, retain bool) (historysource.Snapshot, error) {
	var empty historysource.Snapshot
	hash, err := documentHash(doc)
	if err != nil {
		return empty, err
	}
	snapshot := historysource.Snapshot{Source: doc.Source, Cursor: doc.Cursor, Generation: doc.Generation, Fingerprint: doc.Fingerprint, FormatVersion: doc.FormatVersion, ParserVersion: doc.ParserVersion, ContentHash: hash, RecordCount: len(doc.Records), Gaps: doc.Gaps, Retained: retain}
	snapshot.Records = make([]historysource.RecordRef, 0, len(doc.Records))
	for position, record := range doc.Records {
		hash, err := hashValue(record)
		if err != nil {
			return empty, err
		}
		snapshot.Records = append(snapshot.Records, historysource.RecordRef{ID: record.ID, Hash: hash, Kind: record.Kind, Position: position + 1})
	}
	if doc.RecordRefs != nil {
		snapshot.References = doc.References
	} else {
		snapshot.References, err = s.contextReferences(ctx, doc)
		if err != nil {
			return empty, err
		}
	}
	data := historysource.SnapshotData{Snapshot: snapshot}
	if retain {
		export, err := retainExport(ctx, doc)
		if err != nil {
			return empty, err
		}
		snapshot.ExportHash, err = hashValue(export)
		if err != nil {
			return empty, err
		}
		data.Export = &export
	}
	snapshot.ID, err = snapshotHash(snapshot)
	if err != nil {
		return empty, err
	}
	data.Snapshot = snapshot
	return s.store.SaveFrozen(ctx, data, owner)
}

func (s *Service) Snapshot(ctx context.Context, id string) (historysource.Snapshot, error) {
	if !s.enabled() {
		return historysource.Snapshot{}, historysource.ErrUnavailable
	}
	data, err := s.store.Frozen(ctx, id)
	return data.Snapshot, err
}

func (s *Service) Document(ctx context.Context, id string) (historysource.Document, error) {
	var empty historysource.Document
	if !s.enabled() {
		return empty, historysource.ErrUnavailable
	}
	data, err := s.store.Frozen(ctx, id)
	if err != nil {
		return empty, err
	}
	return s.document(ctx, data)
}

func (s *Service) document(ctx context.Context, data historysource.SnapshotData) (historysource.Document, error) {
	var empty historysource.Document
	frozen := data.Snapshot
	if frozen.Retained {
		if data.Export == nil {
			return empty, historysource.ErrUnavailable
		}
		hash, err := hashValue(data.Export)
		if err != nil || hash != frozen.ExportHash {
			return empty, historysource.ErrUnavailable
		}
		return historysource.Document{Source: frozen.Source, Cursor: frozen.Cursor, Generation: frozen.Generation, Fingerprint: frozen.Fingerprint, FormatVersion: frozen.FormatVersion, ParserVersion: frozen.ParserVersion, Records: data.Export.Records, RecordRefs: frozen.Records, References: frozen.References, Gaps: frozen.Gaps}, nil
	}
	doc, err := s.read(ctx, historysource.Request{Source: frozen.Source, Cursor: frozen.Cursor})
	if err != nil {
		return empty, err
	}
	hash, err := documentHash(doc)
	if err != nil {
		return empty, err
	}
	if doc.Fingerprint != frozen.Fingerprint || doc.ParserVersion != frozen.ParserVersion || doc.FormatVersion != frozen.FormatVersion || doc.Cursor != frozen.Cursor || hash != frozen.ContentHash {
		return empty, historysource.ErrChanged
	}
	doc.RecordRefs, doc.References = frozen.Records, frozen.References
	return doc, nil
}

func (s *Service) Retain(ctx context.Context, id, owner string) error {
	if !s.enabled() {
		return historysource.ErrUnavailable
	}
	return s.store.RetainFrozen(ctx, id, owner)
}

func (s *Service) Release(ctx context.Context, id, owner string) error {
	if !s.enabled() {
		return historysource.ErrUnavailable
	}
	return s.store.ReleaseFrozen(ctx, id, owner)
}

func documentHash(doc historysource.Document) (string, error) {
	// Pending belongs to the unfrozen suffix, not the frozen content identity.
	doc.Pending = false
	doc.RecordRefs, doc.References = nil, nil
	return hashValue(doc)
}

func snapshotHash(snapshot historysource.Snapshot) (string, error) { return hashValue(snapshot) }

func hashValue(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode frozen history identity: %w", err)
	}
	if len(data) > historysource.MaxBytes*2 {
		return "", historysource.ErrLimit
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
