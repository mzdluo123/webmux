package codexrollout

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/basic.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestReadMirrorPrivacyAndPositions(t *testing.T) {
	b := fixture(t)
	var items []Item
	var end int64
	result, err := Read(bytes.NewReader(b), func(r Record) error {
		if r.Offset != end || r.End <= r.Offset || len(r.Hash) != 64 {
			t.Fatal("invalid position", r.Offset, r.End)
		}
		end = r.End
		items = append(items, r.Items...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Consumed != int64(len(b)) || result.Pending || result.Metadata.ID != "00000000-0000-4000-8000-000000000001" || result.Model != "synthetic-model" {
		t.Fatal(result)
	}
	if len(items) != 6 {
		t.Fatalf("got %d items", len(items))
	}
	for _, i := range items {
		if strings.Contains(i.Text, "PRIVATE_SENTINEL") {
			t.Fatal("reasoning escaped")
		}
	}
	if items[0].Text != "你好 hello\n  next line" || items[3].CallID != "call-1" {
		t.Fatal(items)
	}
}
func TestIncompleteAndCorrupt(t *testing.T) {
	b := fixture(t)
	tail := []byte(`{"type":"response_item"`)
	result, err := Read(bytes.NewReader(append(append([]byte{}, b...), tail...)), nil)
	if err != nil || !result.Pending || result.Consumed != int64(len(b)) {
		t.Fatal(result, err)
	}
	result, err = Read(bytes.NewReader(append(append([]byte{}, b...), append(tail, '\n')...)), nil)
	if !errors.Is(err, ErrFormat) || result.Consumed != int64(len(b)) {
		t.Fatal(result, err)
	}
}
func TestVersionUnknownAndLimits(t *testing.T) {
	b := fixture(t)
	for _, version := range []string{"0.76.0", "9.99.0"} {
		_, err := Read(strings.NewReader(strings.Replace(string(b), "0.77.0", version, 1)), nil)
		if !errors.Is(err, ErrVersion) {
			t.Fatal(err)
		}
	}
	_, err := Read(strings.NewReader(strings.Repeat("x", MaxRecord+1)), nil)
	if !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	var diagnostics []Diagnostic
	_, err = Read(strings.NewReader(string(b)+"{\"type\":\"future\",\"payload\":{}}\n"), func(r Record) error { diagnostics = append(diagnostics, r.Diagnostics...); return nil })
	if err != nil || diagnostics[len(diagnostics)-1].Category != "unknown_record" {
		t.Fatal(diagnostics, err)
	}
}
func TestCallbackFailureDoesNotConsume(t *testing.T) {
	b := fixture(t)
	stop := errors.New("transaction failed")
	r, err := Read(bytes.NewReader(b), func(Record) error { return stop })
	if !errors.Is(err, stop) || r.Consumed != 0 {
		t.Fatal(r, err)
	}
}

func TestMaterializedForkAndResume(t *testing.T) {
	tests := []struct {
		name, id string
		count    int
	}{{"fork", "00000000-0000-4000-8000-000000000002", 3}, {"resume", "00000000-0000-4000-8000-000000000001", 8}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + tt.name + ".jsonl")
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			result, err := Read(bytes.NewReader(data), func(r Record) error { count += len(r.Items); return nil })
			if err != nil || result.Metadata.ID != tt.id || count != tt.count {
				t.Fatal(result, count, err)
			}
		})
	}
}

func TestStructuredOutputImagesAndLargeText(t *testing.T) {
	b := fixture(t)
	output := `{"type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":[{"type":"input_text","text":"错误 error\n  intact"},{"type":"input_image","image_url":"data:image/png;base64,SYNTHETIC_SECRET"}]}}` + "\n"
	var last Record
	_, err := Read(strings.NewReader(string(b)+output), func(r Record) error { last = r; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Items) != 1 || last.Items[0].Text != "错误 error\n  intact" || len(last.Items[0].Attachments) != 1 || strings.Contains(last.Items[0].Attachments[0].Reference, "SECRET") {
		t.Fatal(last)
	}
	large := strings.Repeat("中英\n  line\n", 100000)
	raw, err := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []map[string]string{{"type": "input_text", "text": large}}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Read(bytes.NewReader(append(append(append([]byte{}, b...), raw...), '\n')), func(r Record) error { last = r; return nil })
	if err != nil || last.Items[0].Text != large {
		t.Fatal("large message changed", err)
	}
}

func TestTotalInputLimitIncludesPartialTail(t *testing.T) {
	b := fixture(t)
	line := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + strings.Repeat("x", 3<<20) + `"}]}}` + "\n"
	readers := []io.Reader{bytes.NewReader(b)}
	for range 22 {
		readers = append(readers, strings.NewReader(line))
	}
	_, err := Read(io.MultiReader(readers...), nil)
	if !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}
