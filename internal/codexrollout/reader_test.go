package codexrollout

import (
	"bytes"
	"errors"
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
