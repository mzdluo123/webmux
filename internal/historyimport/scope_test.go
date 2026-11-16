package historyimport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/historysource"
)

func scopeReader(text *string) historysource.ReadFunc {
	return func(_ context.Context, req historysource.Request) (historysource.Document, error) {
		body := *text
		if req.Cursor != "" {
			var end int
			if _, err := fmt.Sscan(req.Cursor, &end); err != nil || len(body) < end {
				return historysource.Document{}, historysource.ErrChanged
			}
			body = body[:end]
		}
		return historysource.Document{Source: req.Source, Cursor: fmt.Sprint(len(body)), Fingerprint: "file-1", Generation: "1", FormatVersion: "v1", ParserVersion: "v1", Records: []historysource.Record{{ID: "1", Kind: "prose", Role: "user", Text: body}}}, nil
	}
}

func TestNativeScopeFrozenBoundary(t *testing.T) {
	text := "原始内容"
	svc := New(Options{Readers: map[string]historysource.ReadFunc{"test": scopeReader(&text)}})
	ctx := context.Background()
	scope, err := svc.FreezeScope(ctx, historysource.Source{Agent: "test", NativeID: "native-1", Path: "/native"})
	if err != nil {
		t.Fatal(err)
	}
	text += " appended"
	doc, err := svc.ScopeDocument(ctx, scope.ID)
	if err != nil || doc.Records[0].Text != "原始内容" {
		t.Fatalf("frozen read = %+v, %v", doc, err)
	}
	text = strings.Replace(text, "原", "新", 1)
	if _, err := svc.ScopeDocument(ctx, scope.ID); !errors.Is(err, historysource.ErrChanged) {
		t.Fatalf("changed source = %v", err)
	}
	other := New(Options{Readers: map[string]historysource.ReadFunc{"test": scopeReader(&text)}})
	if _, err := other.Scope(scope.ID); !errors.Is(err, historysource.ErrExpired) {
		t.Fatalf("prior instance = %v", err)
	}
}

func TestNativeScopeLimitsAndExpiry(t *testing.T) {
	c := newScopeCache()
	now := time.Now()
	c.now = func() time.Time { return now }
	for range maxScopes {
		if _, err := c.put(historysource.Snapshot{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.put(historysource.Snapshot{}); !errors.Is(err, historysource.ErrLimit) {
		t.Fatalf("full cache = %v", err)
	}
	now = now.Add(scopeLifetime)
	if _, err := c.put(historysource.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	if len(c.entries) != 1 {
		t.Fatalf("live scopes = %d", len(c.entries))
	}
	c.bytes = maxScopeBytes
	if _, err := c.put(historysource.Snapshot{}); !errors.Is(err, historysource.ErrLimit) {
		t.Fatalf("byte limit = %v", err)
	}
}

func TestNativeScopeConcurrentCapacity(t *testing.T) {
	c := newScopeCache()
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			_, err := c.put(historysource.Snapshot{})
			if err != nil && !errors.Is(err, historysource.ErrLimit) {
				t.Errorf("put: %v", err)
			}
		})
	}
	wg.Wait()
	if len(c.entries) != maxScopes {
		t.Fatalf("scope count = %d", len(c.entries))
	}
}

func TestScopePageUnicodeContinuation(t *testing.T) {
	text := "甲🙂乙"
	svc := New(Options{Readers: map[string]historysource.ReadFunc{"test": scopeReader(&text)}})
	scope, err := svc.FreezeScope(context.Background(), historysource.Source{Agent: "test", NativeID: "native", Path: "/native"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.Page(context.Background(), scope.ID, historysource.PageRequest{MaxBytes: 4})
	if err != nil || first.Records[0].Text != "甲" || first.TextOffset != 3 {
		t.Fatalf("first %+v %v", first, err)
	}
	second, err := svc.Page(context.Background(), scope.ID, historysource.PageRequest{After: first.Next, TextOffset: first.TextOffset, MaxBytes: 4})
	if err != nil || second.Records[0].Text != "🙂" || second.TextOffset != 7 {
		t.Fatalf("second %+v %v", second, err)
	}
	if _, err := svc.Page(context.Background(), scope.ID, historysource.PageRequest{TextOffset: 4, MaxBytes: 4}); !errors.Is(err, historysource.ErrFormat) {
		t.Fatalf("split rune offset: %v", err)
	}
}
func TestPinnedScopeKeepsItsMemoryReservation(t *testing.T) {
	svc := New(Options{})
	now := time.Now()
	svc.scopes.now = func() time.Time { return now }
	scope, err := svc.scopes.put(historysource.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.PinScope(scope.ID); err != nil {
		t.Fatal(err)
	}
	before := svc.scopes.bytes
	now = now.Add(scopeLifetime)
	if _, err := svc.Scope(scope.ID); !errors.Is(err, historysource.ErrExpired) {
		t.Fatalf("expired scope %v", err)
	}
	if svc.scopes.bytes != before || len(svc.scopes.entries) != 1 {
		t.Fatal("in-flight reservation was released")
	}
	svc.UnpinScope(scope.ID)
	if svc.scopes.bytes != 0 || len(svc.scopes.entries) != 0 {
		t.Fatal("expired reservation was retained after completion")
	}
}
