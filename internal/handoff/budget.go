// Package handoff coordinates portable provider context without owning PTYs or
// encoding client-visible JSON.
package handoff

import (
	"encoding/json"
)

const DefaultTokenCap = 16000
const MaxHistoryBytes = 64000

type Attachment struct {
	Name, MimeType, Reference, Kind string
	Size                            int64
}

type BudgetInput struct {
	TokenCap                           int64
	UserText                           string
	Attachments                        []Attachment
	Used, Window, CompactionThreshold  int64
	Measured                           bool
	Inline                             bool
	MaxPromptBytes, MaxPromptWireBytes int64
}

type Budget struct {
	Window, NativeUsed, Current, Reserve, History, Used int64
	Source                                              string
	OverBudget                                          bool
}

// These closed types contain only strings and finite structural fields, so
// encoding/json cannot fail. The result is used solely for internal accounting.
func jsonBytes[T ~string](v T) int64 {
	body, _ := json.Marshal(v)
	return int64(len(body))
}

func ComputeBudget(in BudgetInput) Budget {
	window := in.Window
	if window <= 0 {
		window = 128000
	}
	if in.CompactionThreshold > 0 {
		window = min(window, in.CompactionThreshold)
	}
	cap := in.TokenCap
	if cap == 0 {
		cap = DefaultTokenCap
	}
	cap = min(cap, MaxHistoryBytes)
	current := jsonBytes(in.UserText)
	wire := current
	for _, a := range in.Attachments {
		if a.Kind == "image" {
			current += 8192
		} else {
			current += 4096
		}
		if a.Size < 0 {
			wire = in.MaxPromptWireBytes + 1
		} else if a.Kind == "image" {
			wire += (a.Size+2)/3*4 + jsonBytes(a.Reference) + jsonBytes(a.Name) + jsonBytes(a.MimeType) + 1024
		} else {
			wire += 6*a.Size + jsonBytes(a.Reference) + jsonBytes(a.Name) + jsonBytes(a.MimeType) + 1024
		}
	}
	reserve := max(int64(16000), window/4+min(window%4, int64(1)))
	used := max(int64(0), in.Used)
	history := int64(0)
	if used < window && current < window-used && reserve < window-used-current {
		history = min(cap, window-used-current-reserve)
	}
	over := used >= window || current >= window-used || reserve >= window-used-current
	if (in.MaxPromptBytes > 0 && int64(len(in.UserText)) > in.MaxPromptBytes) || (in.MaxPromptWireBytes > 0 && wire > in.MaxPromptWireBytes) {
		history, over = 0, true
	}
	if in.Inline {
		separator := int64(len("\n\nUser message:\n"))
		// Capabilities are not known for a fresh preview. Leave room for owned
		// upload paths when an attachment must degrade to inline text.
		separator += int64(len(in.Attachments)) * 1024
		if in.MaxPromptBytes <= 0 || int64(len(in.UserText))+separator > in.MaxPromptBytes {
			history = 0
			over = true
		} else {
			history = min(history, in.MaxPromptBytes-int64(len(in.UserText))-separator)
		}
		if in.MaxPromptWireBytes <= 0 || wire > in.MaxPromptWireBytes {
			history = 0
			over = true
		} else {
			history = min(history, in.MaxPromptWireBytes-wire)
		}
	}
	source := "estimated"
	if in.Measured {
		source = "provider"
	}
	return Budget{Window: window, NativeUsed: used, Current: current, Reserve: reserve, History: max(int64(0), history), Source: source, OverBudget: over}
}
