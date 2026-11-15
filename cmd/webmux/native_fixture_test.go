//go:build linux || darwin

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/rainchan/webmux/internal/historysource"
)

// appendMCPNativeHistory writes actual omp schema-3 graph entries. The native
// store stays outside WebMux's database, including when a later turn is appended.
func appendMCPNativeHistory(t *testing.T, source historysource.Source, first, count int, text func(int) string) {
	t.Helper()
	file, err := os.OpenFile(source.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	if first == 0 {
		if err := encoder.Encode(map[string]any{"type": "session", "version": 3, "id": source.NativeID}); err != nil {
			t.Fatal(err)
		}
	}
	for index := first; index < first+count; index++ {
		parent := ""
		if index > 0 {
			parent = "entry-" + strconv.Itoa(index-1)
		}
		role := "assistant"
		if index%2 == 0 {
			role = "user"
		}
		entry := map[string]any{"type": "message", "id": "entry-" + strconv.Itoa(index), "parentId": parent,
			"message": map[string]any{"role": role, "content": []map[string]string{{"type": "text", "text": text(index)}}}}
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
}
