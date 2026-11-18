package codexrollout

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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

func TestDiscoverySmallHeadersDoNotConsumeBodySizedBudget(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sessions")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	header := bytes.SplitN(fixture(t), []byte{'\n'}, 2)[0]
	data := append(append(append([]byte{}, header...), '\n'), []byte(strings.Repeat("x", 8<<10))...)
	for i := range 200 {
		path := filepath.Join(dir, strconv.Itoa(i)+".jsonl")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := discover(context.Background(), root, discoveryLimits{MaxDiscoveryFiles, 128 << 10, 1000})
	if err != nil || result.Truncated || len(result.Files) != 200 {
		t.Fatal(len(result.Files), result.Truncated, err)
	}
	for _, file := range result.Files {
		if file.Problem != "" || file.Metadata.ID == "" {
			t.Fatal(file)
		}
	}
}

func TestInspectRetainsVerifiedIdentityForUnsupportedFormats(t *testing.T) {
	header := string(bytes.SplitN(fixture(t), []byte{'\n'}, 2)[0])
	tests := []struct {
		name, header string
		want         error
	}{
		{"old-writer", strings.Replace(header, "0.77.0", "0.76.0", 1), ErrVersion},
		{"history-base", strings.Replace(header, `"cli_version":`, `"history_base":{"session_id":"parent"},"cli_version":`, 1), ErrLineage},
		{"fork-reference", strings.Replace(header, `"cli_version":`, `"forked_from_id":"parent","cli_version":`, 1), ErrLineage},
		{"subagent-reference", strings.Replace(header, `"cli_version":`, `"subagent_history_start_ordinal":0,"cli_version":`, 1), ErrLineage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "sessions")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "misleading-name.jsonl")
			if err := os.WriteFile(path, []byte(tt.header+"\ninvalid body\n"), 0600); err != nil {
				t.Fatal(err)
			}
			metadata, err := Inspect(context.Background(), path)
			if !errors.Is(err, tt.want) || metadata.ID != "00000000-0000-4000-8000-000000000001" {
				t.Fatalf("inspect lost verified identity: %+v %v", metadata, err)
			}
			found, err := Discover(context.Background(), root)
			if err != nil || len(found.Files) != 1 || found.Files[0].Metadata.ID != metadata.ID || found.Files[0].Problem != "unreadable_or_unsupported" {
				t.Fatalf("discovery lost candidate or problem: %+v %v", found, err)
			}
		})
	}
}

func TestInspectNeverInfersIdentityFromFilename(t *testing.T) {
	for _, header := range []string{
		`{"type":"session_meta","payload":{"cwd":"/synthetic","cli_version":"0.169.0"}}`,
		`{"type":"response_item","payload":{"id":"filename-id","cwd":"/synthetic","cli_version":"0.169.0"}}`,
		`{"type":"session_meta","payload":{"id":17,"cwd":"/synthetic","cli_version":"0.169.0"}}`,
		`{"type":"session_meta","payload":{"id":"filename-id","cwd":"/synthetic","cli_version":"0.169.0"}`,
	} {
		path := filepath.Join(t.TempDir(), "rollout-filename-id.jsonl")
		if err := os.WriteFile(path, []byte(header+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if metadata, err := Inspect(context.Background(), path); !errors.Is(err, ErrFormat) || metadata.ID != "" {
			t.Fatalf("unverified header became identity: %+v %v", metadata, err)
		}
	}
}
