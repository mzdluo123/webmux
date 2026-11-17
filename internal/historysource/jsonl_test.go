package historysource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONLPartialRecordBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native.jsonl")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", MaxRecordBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadJSONL(context.Background(), Request{Source: Source{Path: path}}, nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("unterminated record bypassed record budget: %v", err)
	}
}

func TestJSONLCancelledEmptySource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native.jsonl")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadJSONL(ctx, Request{Source: Source{Path: path}}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("empty source ignored cancellation: %v", err)
	}
}
