// Package historyimport reads and freezes native history for the current process.
package historyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/rainchan/webmux/internal/historysource"
)

type DiscoverFunc func(context.Context, string) ([]historysource.Source, error)
type Identity struct{ Agent, NativeID, Root string }
type Options struct {
	Readers     map[string]historysource.ReadFunc
	Discoverers map[string]DiscoverFunc
	Roots       map[string]string
	Resolve     func(context.Context, string) (Identity, error)
}
type Service struct {
	readers         map[string]historysource.ReadFunc
	discoverers     map[string]DiscoverFunc
	roots           map[string]string
	resolveIdentity func(context.Context, string) (Identity, error)
	scopes          *scopeCache
}

func New(o Options) *Service {
	return &Service{readers: o.Readers, discoverers: o.Discoverers, roots: o.Roots, resolveIdentity: o.Resolve, scopes: newScopeCache()}
}
func (s *Service) Discover(ctx context.Context, agent, root string) ([]historysource.Source, error) {
	if s == nil {
		return nil, historysource.ErrUnavailable
	}
	if root == "" {
		root = s.roots[agent]
	}
	fn := s.discoverers[agent]
	if fn == nil {
		return nil, historysource.ErrFormat
	}
	return fn(ctx, root)
}
func (s *Service) RuntimeSource(ctx context.Context, id string) (historysource.Source, error) {
	var empty historysource.Source
	if s == nil || s.resolveIdentity == nil {
		return empty, historysource.ErrUnavailable
	}
	identity, err := s.resolveIdentity(ctx, id)
	if err != nil {
		return empty, err
	}
	if identity.NativeID == "" {
		return empty, historysource.ErrIdentity
	}
	root := identity.Root

	if root == "" {
		return empty, historysource.ErrUnavailable
	}
	found, err := s.Discover(ctx, identity.Agent, root)
	if err != nil {
		return empty, err
	}
	matches := 0
	var result historysource.Source
	for _, candidate := range found {
		if candidate.Agent == identity.Agent && candidate.NativeID == identity.NativeID {
			result = candidate
			matches++
		}
	}
	if matches != 1 {
		return empty, historysource.ErrIdentity
	}
	return result, nil
}
func (s *Service) FreezeRuntime(ctx context.Context, id string) (Scope, error) {
	source, err := s.RuntimeSource(ctx, id)
	if err != nil {
		return Scope{}, err
	}
	return s.FreezeScope(ctx, source)
}
func (s *Service) Document(ctx context.Context, id string) (historysource.Document, error) {
	return s.ScopeDocument(ctx, id)
}
func (s *Service) Snapshot(ctx context.Context, id string) (historysource.Snapshot, error) {
	scope, err := s.Scope(id)
	return scope.Snapshot, err
}
func (s *Service) document(ctx context.Context, frozen historysource.Snapshot) (historysource.Document, error) {
	var empty historysource.Document
	doc, err := s.readNative(ctx, historysource.Request{Source: frozen.Source, Cursor: frozen.Cursor})
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
	doc.RecordRefs = frozen.Records
	return doc, nil
}
func (s *Service) readNative(ctx context.Context, req historysource.Request) (historysource.Document, error) {
	var empty historysource.Document
	if s == nil {
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

func documentHash(doc historysource.Document) (string, error) {
	// Pending belongs to the unfrozen suffix, not the frozen content identity.
	doc.Pending = false
	doc.RecordRefs = nil
	return hashValue(doc)
}

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
