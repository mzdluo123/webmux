package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// saveHistory exports only synthetic sessions, and only when a harness explicitly
// supplies its isolated HOME. Never discover or write a developer's history.
// The private JSON store remains authoritative for TUI/ACP leases and recovery.
func (s *store) saveHistory(ss *session) error {
	home := os.Getenv("FAKEAGENT_HISTORY_HOME")
	if home == "" {
		return nil
	}
	if !filepath.IsAbs(home) || home != os.Getenv("HOME") {
		return fmt.Errorf("fake history requires an absolute isolated HOME")
	}
	var path string
	var transcript bytes.Buffer
	enc := json.NewEncoder(&transcript)
	const timestamp = "2025-12-01T00:00:00Z"
	switch s.profile {
	case "omp":
		path = filepath.Join(home, ".omp", "agent", "sessions", "fake-project", ss.ID+".jsonl")
		if err := enc.Encode(map[string]any{"type": "session", "version": 3, "id": ss.ID, "cwd": ss.Cwd, "timestamp": timestamp}); err != nil {
			return err
		}
	case "codex":
		path = filepath.Join(home, ".codex", "sessions", "rollout-"+ss.ID+".jsonl")
		if err := enc.Encode(map[string]any{"type": "session_meta", "timestamp": timestamp, "payload": map[string]any{"id": ss.ID, "cwd": ss.Cwd, "timestamp": timestamp, "cli_version": "0.77.0", "originator": "synthetic-fixture"}}); err != nil {
			return err
		}
	case "claudecode":
		path = filepath.Join(home, ".claude", "projects", "fake-project", ss.ID+".jsonl")
	default:
		// Other fake profiles do not claim native history format coverage.
		return nil
	}
	parent := ""
	for _, m := range ss.Messages {
		role := m.Role
		if role == "agent" {
			role = "assistant"
		}
		var entry any
		switch s.profile {
		case "omp":
			content := []map[string]any{{"type": "text", "text": m.Text}}
			for _, block := range m.Blocks {
				if block.Type == "image" {
					content = append(content, map[string]any{"type": "image", "mimeType": block.MimeType})
				}
			}
			entry = map[string]any{"type": "message", "id": m.ID, "parentId": parent, "message": map[string]any{"role": role, "content": content}}
		case "codex":
			kind := "input_text"
			if role == "assistant" {
				kind = "output_text"
			}
			entry = map[string]any{"type": "response_item", "timestamp": timestamp, "payload": map[string]any{"type": "message", "id": m.ID, "role": role, "content": []map[string]any{{"type": kind, "text": m.Text}}}}
		case "claudecode":
			entry = map[string]any{"type": role, "uuid": m.ID, "parentUuid": parent, "sessionId": ss.ID, "cwd": ss.Cwd, "timestamp": timestamp, "message": map[string]any{"role": role, "content": m.Text}}
		}
		if err := enc.Encode(entry); err != nil {
			return err
		}
		parent = m.ID
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	previous, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// A save may add turns or update config/title, but must never replace an
	// already frozen prefix or its inode. Reject mutations instead of hiding them.
	if !bytes.HasPrefix(transcript.Bytes(), previous) {
		return fmt.Errorf("fake history changed persisted prefix: %s", ss.ID)
	}
	if len(previous) == transcript.Len() {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(transcript.Bytes()[len(previous):])
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
