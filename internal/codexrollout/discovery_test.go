package codexrollout

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoveryReturnsBoundedPartialMetadata(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sessions")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	header := bytes.SplitN(fixture(t), []byte{'\n'}, 2)[0]
	for _, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, name+".jsonl"), append(append([]byte{}, header...), '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := discover(context.Background(), root, discoveryLimits{2, MaxDiscoveryBytes, 100})
	if err != nil || !out.Truncated || len(out.Files) != 2 || out.Files[0].Metadata.ID == "" {
		t.Fatal(out, err)
	}
	out, err = discover(context.Background(), root, discoveryLimits{10, 32, 100})
	if err != nil || !out.Truncated || len(out.Files) != 1 || out.Files[0].Problem != "metadata_limit" {
		t.Fatal(out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Discover(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDiscoveryDoesNotReadConversationBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	header := bytes.SplitN(fixture(t), []byte{'\n'}, 2)[0]
	data := append(append(append([]byte{}, header...), '\n'), []byte(strings.Repeat("not json body", MaxMetadata))...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if metadata, err := Inspect(context.Background(), path); err != nil || metadata.ID == "" {
		t.Fatal(metadata, err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", MaxMetadata+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(context.Background(), path); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}
