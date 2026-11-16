package historyimport

import (
	"context"
	"crypto/rand"
	"sync"
	"time"

	"github.com/rainchan/webmux/internal/historysource"
)

const (
	scopeLifetime = 5 * time.Minute
	maxScopes     = 32
	maxScopeBytes = 64 << 20
)

// Scope fixes a native source and its complete record boundary for one process.
type Scope struct {
	ID        string
	Snapshot  historysource.Snapshot
	ExpiresAt time.Time
}

type scopeEntry struct {
	pins  int
	scope Scope
	bytes int
}

type scopeCache struct {
	mu       sync.Mutex
	entries  map[string]scopeEntry
	bytes    int
	instance string
	now      func() time.Time
}

func newScopeCache() *scopeCache {
	return &scopeCache{entries: make(map[string]scopeEntry), instance: rand.Text(), now: time.Now}
}

func (c *scopeCache) clean(now time.Time) {
	for id, e := range c.entries {
		if e.pins == 0 && !now.Before(e.scope.ExpiresAt) {
			c.bytes -= e.bytes
			delete(c.entries, id)
		}
	}
}

func (c *scopeCache) put(snapshot historysource.Snapshot) (Scope, error) {
	// Account for strings, backing slices and conservative struct overhead.
	size := 1024 + len(snapshot.Source.Agent) + len(snapshot.Source.Path) + len(snapshot.Source.NativeID) + len(snapshot.Source.Branch)
	size += len(snapshot.Cursor) + len(snapshot.Generation) + len(snapshot.Fingerprint) + len(snapshot.FormatVersion) + len(snapshot.ParserVersion) + len(snapshot.ContentHash)
	for _, record := range snapshot.Records {
		size += 128 + len(record.ID) + len(record.Hash) + len(record.Kind)
	}
	for _, gap := range snapshot.Gaps {
		size += 64 + len(gap.RecordID) + len(gap.Category)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	c.clean(now)
	id := c.instance + "_" + rand.Text()
	size += len(id)
	if len(c.entries) >= maxScopes || size > maxScopeBytes-c.bytes {
		return Scope{}, historysource.ErrLimit
	}
	snapshot.ID = id
	scope := Scope{ID: id, Snapshot: snapshot, ExpiresAt: now.Add(scopeLifetime)}
	stored := scope
	stored.Snapshot.Records = append([]historysource.RecordRef(nil), snapshot.Records...)
	stored.Snapshot.Gaps = append([]historysource.Gap(nil), snapshot.Gaps...)
	c.entries[id] = scopeEntry{scope: stored, bytes: size}
	c.bytes += size
	return scope, nil
}

func (c *scopeCache) get(id string) (Scope, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clean(c.now())
	e, ok := c.entries[id]
	if !ok || !c.now().Before(e.scope.ExpiresAt) {
		return Scope{}, historysource.ErrExpired
	}
	// Consumers receive their own manifest slices.
	scope := e.scope
	scope.Snapshot.Records = append([]historysource.RecordRef(nil), scope.Snapshot.Records...)
	scope.Snapshot.Gaps = append([]historysource.Gap(nil), scope.Snapshot.Gaps...)
	return scope, nil
}

// FreezeScope reads an exact, authorized native identity supplied by the caller.
// The cache retains source metadata and record hashes; bodies stay in the source.
func (s *Service) FreezeScope(ctx context.Context, source historysource.Source) (Scope, error) {
	doc, err := s.readNative(ctx, historysource.Request{Source: source})
	if err != nil {
		return Scope{}, err
	}
	hash, err := documentHash(doc)
	if err != nil {
		return Scope{}, err
	}
	snapshot := historysource.Snapshot{Source: doc.Source, Cursor: doc.Cursor, Generation: doc.Generation, Fingerprint: doc.Fingerprint, FormatVersion: doc.FormatVersion, ParserVersion: doc.ParserVersion, ContentHash: hash, RecordCount: len(doc.Records), Gaps: doc.Gaps}
	snapshot.Records = make([]historysource.RecordRef, 0, len(doc.Records))
	for i, record := range doc.Records {
		hash, err := hashValue(record)
		if err != nil {
			return Scope{}, err
		}
		snapshot.Records = append(snapshot.Records, historysource.RecordRef{ID: record.ID, Hash: hash, Kind: record.Kind, Position: i + 1})
	}
	return s.scopes.put(snapshot)
}

// ScopeDocument verifies the original cursor and content before returning bodies.
func (s *Service) ScopeDocument(ctx context.Context, id string) (historysource.Document, error) {
	var empty historysource.Document
	if s == nil {
		return empty, historysource.ErrUnavailable
	}
	scope, err := s.scopes.get(id)
	if err != nil {
		return empty, err
	}
	return s.document(ctx, scope.Snapshot)
}

func (s *Service) Scope(id string) (Scope, error) {
	if s == nil {
		return Scope{}, historysource.ErrExpired
	}
	return s.scopes.get(id)
}

// ReserveScopeBytes accounts for the preview text that shares a scope lifetime.
func (s *Service) ReserveScopeBytes(id string, bytes int) error {
	c := s.scopes
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clean(c.now())
	e, ok := c.entries[id]
	if !ok {
		return historysource.ErrExpired
	}
	if bytes < 0 || bytes > maxScopeBytes-c.bytes {
		return historysource.ErrLimit
	}
	e.bytes += bytes
	c.bytes += bytes
	c.entries[id] = e
	return nil
}
func (s *Service) ReleaseScope(id string) {
	c := s.scopes
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[id]; ok {
		c.bytes -= e.bytes
		delete(c.entries, id)
	}
}

func (s *Service) PinScope(id string) error {
	c := s.scopes
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clean(c.now())
	e, ok := c.entries[id]
	if !ok || !c.now().Before(e.scope.ExpiresAt) {
		return historysource.ErrExpired
	}
	e.pins++
	c.entries[id] = e
	return nil
}
func (s *Service) UnpinScope(id string) {
	c := s.scopes
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[id]
	if ok && e.pins > 0 {
		e.pins--
		c.entries[id] = e
	}
	c.clean(c.now())
}
