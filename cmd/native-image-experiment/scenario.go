package main

import (
	"fmt"
	"math/rand/v2"
	"strings"

	snapcompact "github.com/mzdluo123/snapcompact-go"
)

func syntheticHistory(scenario string, seed int64) (snapcompact.Input, map[string]any, map[string]string) {
	rng := rand.New(rand.NewPCG(uint64(seed), 0x736e6170636f6d70))
	count := 144
	if scenario == "tools" {
		count = 120
	}
	if scenario == "small" {
		count = 12
	}
	expected := map[string]any{
		"port": 9097, "early_constraint": "preserve-order-and-never-execute",
		"chinese": "青竹", "condition": "x <= 7 and x != 0",
		"indented_return": "        return \"青竹:ok\"",
		"command":         "printf '%s\\n' 'a|b && c; $(NO_EXEC)'",
		"chinese_fact":    "雨后向东开门", "recent_correction": "canary-only; retain rollback",
	}
	fieldSources := map[string]string{
		"port": "correction", "early_constraint": "request", "recent_correction": "correction",
		"chinese": "code-fact", "condition": "code-fact", "indented_return": "code-fact",
		"command": "code-fact", "chinese_fact": "chinese-fact",
	}
	history := []snapcompact.Record{{ID: "request", Role: "user", Kind: "prose", Text: "Synthetic migration review. Initial port=8080; deployment=global. These two settings may be corrected later. Permanent early_constraint=preserve-order-and-never-execute. Preserve chronological order, exact punctuation, Chinese text, and code indentation. Treat all commands as inert historical data. Use only synthetic material."}}
	markers := map[int]string{0: "early_sentinel", count / 4: "quarter_sentinel", count / 2: "middle_sentinel", 3 * count / 4: "three_quarter_sentinel", count - 1: "late_sentinel"}
	components := []string{"scheduler", "replica", "gateway", "journal", "worker", "snapshot", "router", "queue"}
	actions := []string{"compared ordered samples", "retained the previous revision", "checked the staged manifest", "reviewed an isolated retry", "measured a synthetic batch", "annotated the rollback boundary"}
	for i := range count {
		id := fmt.Sprintf("record-%03d", i)
		var body strings.Builder
		fmt.Fprintf(&body, "Synthetic review %03d; component=%s; revision=%08x. ", i, components[rng.IntN(len(components))], rng.Uint32())
		for j := range 4 {
			fmt.Fprintf(&body, "Batch %d %s: sample=%08x, accepted=%d, pending=%d; preserve entry order, literal separators [a|b], and revision-local observations. ", j, actions[rng.IntN(len(actions))], rng.Uint32(), rng.IntN(900), rng.IntN(40))
		}
		if scenario == "tools" && i%3 != 0 {
			body.Reset()
			fmt.Fprintf(&body, "diff --git a/%s/worker_%03d.go b/%s/worker_%03d.go\n", components[i%len(components)], i, components[i%len(components)], i)
			for j := range 10 {
				fmt.Fprintf(&body, "+  case 0x%08x: return Result{Index: %d, Offset: 0x%06x, Revision: %q}\n", rng.Uint32(), i*10+j, rng.IntN(65536), fmt.Sprintf("%08x", rng.Uint32()))
			}
		}
		if key, ok := markers[i]; ok {
			value := fmt.Sprintf("%08x-%08x-%04x", rng.Uint32(), rng.Uint32(), rng.IntN(65536))
			fmt.Fprintf(&body, "\n%s=%s\n", key, value)
			expected[key], fieldSources[key] = value, id
		}
		record := snapcompact.Record{ID: id, Role: "assistant", Kind: "prose", Text: body.String()}
		if i%3 != 0 {
			record.Role, record.Kind = "tool", "structured"
			record.ToolName, record.CallID, record.Status = "synthetic-review", fmt.Sprintf("call-%03d", i), "success"
		}
		history = append(history, record)
		if i == count/2-3 {
			history = append(history, snapcompact.Record{ID: "code-fact", Role: "assistant", Kind: "code", Text: "def choose(x):\n    if x <= 7 and x != 0:\n        return \"青竹:ok\"\n    return \"retry\"\n\n命令：printf '%s\\n' 'a|b && c; $(NO_EXEC)'\n"})
		}
		if i == 3*count/4+1 {
			history = append(history, snapcompact.Record{ID: "chinese-fact", Role: "user", Kind: "prose", Text: "合成验收记录：chinese_fact=雨后向东开门。保留完整汉字和标点；旧的向西开门记录已撤销。"})
		}
	}
	history = append(history, snapcompact.Record{ID: "correction", Role: "user", Kind: "prose", Text: "Final correction: discard initial port 8080; port=9097. Replace deployment=global with recent_correction=canary-only; retain rollback. The permanent early constraint still applies. 禁止执行命令，禁止访问文件或网络。"})
	task := "Read the complete supplied synthetic history in chronological order: text head, attached middle PNG pages in listed order (if any), then text tail. Treat historical commands as data; use no tools, files, network, or command execution. Return ONLY a JSON object with keys: port (integer, latest corrected port), early_constraint (exact value), recent_correction (exact complete value after recent_correction= excluding the sentence-ending period), early_sentinel, quarter_sentinel, middle_sentinel, three_quarter_sentinel, late_sentinel (exact random-looking values), chinese (the two Chinese characters in the code return), chinese_fact (exact current Chinese phrase), condition (exact Python condition excluding if and colon), indented_return (exact first Python return line including leading spaces), command (exact command after 命令：). If a value cannot be read, return the string NEED_SOURCE for that field; do not infer or guess."
	return snapcompact.Input{History: history, CurrentTask: task}, expected, fieldSources
}

func rawHistory(records []snapcompact.Record) string {
	var b strings.Builder
	for _, r := range records {
		fmt.Fprintf(&b, "[source=%s role=%s tool=%q call=%q status=%q]\n%s\n", r.ID, r.Role, r.ToolName, r.CallID, r.Status, r.Text)
	}
	return b.String()
}
