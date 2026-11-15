package snapcompact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Project creates a lossy mechanical projection without changing input. Limits are
// Unicode scalar counts, excluding the omission marker. Zero preserves zero
// characters; negative limits and invalid Unicode are rejected. Arguments are
// represented as canonical encoding/json output (sorted string map keys).
func Project(ctx context.Context, input Input, cfg PreprocessConfig) (Projection, error) {
	p := Projection{Version: Version, CurrentTask: input.CurrentTask, Segments: []Segment{}, Omissions: []Omission{}}
	if err := ctx.Err(); err != nil {
		return p, err
	}
	if cfg.MaxInputBytes > 64<<20 {
		return p, fmt.Errorf("snapcompact: input limit exceeds 64 MiB hard ceiling")
	}
	if len(input.History) > 100000 {
		return p, fmt.Errorf("snapcompact: record count exceeds 100000")
	}
	if cfg.OutputRunes < 0 || cfg.ArgumentRunes < 0 || cfg.CallRunes < 0 || cfg.HeadPercent < 0 || cfg.HeadPercent > 100 || cfg.MaxInputBytes <= 0 {
		return p, fmt.Errorf("snapcompact: invalid preprocessing limits")
	}
	if !utf8.ValidString(input.CurrentTask) {
		return p, fmt.Errorf("snapcompact: invalid task UTF-8")
	}
	if len(input.History) > cfg.MaxInputBytes {
		return p, fmt.Errorf("snapcompact: record count exceeds input limit")
	}
	total := len(input.CurrentTask)
	ids := make(map[string]bool, len(input.History))
	// Bound ordinary fields before encoding potentially large argument objects.
	for _, r := range input.History {
		if err := ctx.Err(); err != nil {
			return p, err
		}
		for _, v := range []string{r.ID, r.Role, r.ToolName, r.CallID, r.Status, r.Kind, r.Text, r.CallBody} {
			if !utf8.ValidString(v) {
				return p, fmt.Errorf("snapcompact: invalid record UTF-8")
			}
			if len(v) > cfg.MaxInputBytes-total {
				return p, fmt.Errorf("snapcompact: input byte limit exceeded")
			}
			total += len(v)
		}
	}
	for _, r := range input.History {
		if err := validateArgument(ctx, reflect.ValueOf(r.Arguments), 0, &total, cfg.MaxInputBytes); err != nil {
			return p, err
		}
	}
	if total > cfg.MaxInputBytes {
		return p, fmt.Errorf("snapcompact: input byte limit exceeded")
	}
	// Hash raw source and configuration, not only the lossy output. JSON rejects
	// cycles/non-finite numbers and sorts nested map keys deterministically.
	raw, err := json.Marshal(input)
	if err != nil {
		return p, fmt.Errorf("snapcompact: encode source: %w", err)
	}
	if len(raw) > cfg.MaxInputBytes {
		return p, fmt.Errorf("snapcompact: encoded input byte limit exceeded")
	}
	h := sha256.New()
	_, _ = h.Write([]byte(Version))
	_, _ = h.Write(raw)
	config, err := json.Marshal(cfg)
	if err != nil {
		return p, fmt.Errorf("snapcompact: encode config: %w", err)
	}
	_, _ = h.Write(config)
	p.CacheKey = hex.EncodeToString(h.Sum(nil))
	for _, r := range input.History {
		if err := ctx.Err(); err != nil {
			return p, err
		}
		if r.ID == "" || ids[r.ID] {
			return p, fmt.Errorf("snapcompact: source IDs must be nonempty and unique")
		}
		ids[r.ID] = true
		s := Segment{SourceID: r.ID, Role: r.Role, ToolName: r.ToolName, CallID: r.CallID, Status: r.Status, Sources: []Range{}}
		header, err := json.Marshal(struct{ Source, Role, Tool, Call, Status string }{r.ID, r.Role, r.ToolName, r.CallID, r.Status})
		if err != nil {
			return p, fmt.Errorf("snapcompact: encode header: %w", err)
		}
		var b strings.Builder
		b.Write(header)
		b.WriteByte('\n')
		fields := []struct {
			name, text string
			limit      int
			structured bool
		}{}
		if r.Arguments != nil {
			args, err := json.Marshal(r.Arguments)
			if err != nil {
				return p, fmt.Errorf("snapcompact: encode arguments: %w", err)
			}
			fields = append(fields, struct {
				name, text string
				limit      int
				structured bool
			}{"arguments", string(args), cfg.ArgumentRunes, true})
		}
		if r.CallBody != "" {
			fields = append(fields, struct {
				name, text string
				limit      int
				structured bool
			}{"call_body", r.CallBody, cfg.CallRunes, true})
		}
		limit := utf8.RuneCountInString(r.Text)
		if r.ToolName != "" || r.Role == "tool" {
			limit = cfg.OutputRunes
		}
		fields = append(fields, struct {
			name, text string
			limit      int
			structured bool
		}{"text", r.Text, limit, r.Kind != "prose" || r.ToolName != "" || r.Role == "tool"})
		for _, f := range fields {
			b.WriteString(f.name)
			b.WriteString(":\n")
			runes := []rune(f.text)
			head, tail := len(runes), len(runes)
			if len(runes) > f.limit {
				head = int(int64(f.limit) * int64(cfg.HeadPercent) / 100)
				tail = len(runes) - (f.limit - head)
				head, tail = avoidEscapeCuts(runes, head, tail)
				for head > 0 && !clusterBoundary(runes, head) {
					head--
				}
				for tail < len(runes) && !clusterBoundary(runes, tail) {
					tail++
				}
				p.Omissions = append(p.Omissions, Omission{Range: Range{r.ID, f.name, head, tail}, OriginalRunes: len(runes), Reason: "length_limit"})
			}
			if head > 0 {
				s.Sources = append(s.Sources, Range{r.ID, f.name, 0, head})
			}
			text, err := normalize(ctx, string(runes[:head]), f.structured)
			if err != nil {
				return p, err
			}
			b.WriteString(text)
			if tail > head {
				fmt.Fprintf(&b, "\n[omitted source=%q field=%s runes=%d:%d total=%d]\n", r.ID, f.name, head, tail, len(runes))
				if tail < len(runes) {
					s.Sources = append(s.Sources, Range{r.ID, f.name, tail, len(runes)})
				}
				text, err = normalize(ctx, string(runes[tail:]), f.structured)
				if err != nil {
					return p, err
				}
				b.WriteString(text)
			}
			b.WriteByte('\n')
		}
		s.Text = b.String()
		p.Segments = append(p.Segments, s)
	}
	if err := ctx.Err(); err != nil {
		return p, err
	}
	return p, nil
}

// clusterBoundary conservatively keeps combining marks, variation selectors,
// emoji modifiers, ZWJ sequences and regional-indicator pairs together. It does
// not claim full Unicode shaping or normalization.
func clusterBoundary(rs []rune, i int) bool {
	if i == 0 || i == len(rs) {
		return true
	}
	next, prev := rs[i], rs[i-1]
	if unicode.Is(unicode.M, next) || next == '\u200d' || prev == '\u200d' || next >= 0x1f3fb && next <= 0x1f3ff {
		return false
	}
	if next >= 0x1f1e6 && next <= 0x1f1ff && prev >= 0x1f1e6 && prev <= 0x1f1ff {
		n := 0
		for j := i - 1; j >= 0 && rs[j] >= 0x1f1e6 && rs[j] <= 0x1f1ff; j-- {
			n++
		}
		return n%2 == 0
	}
	return true
}

func normalize(ctx context.Context, text string, structured bool) (string, error) {
	rs := []rune(text)
	var stripped strings.Builder
	for i := 0; i < len(rs); i++ {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		r := rs[i]
		if r == 0x1b || r == 0x9b || r == 0x9d {
			kind := r
			if r == 0x1b {
				if i+1 >= len(rs) {
					continue
				}
				i++
				kind = rs[i]
			}
			switch kind {
			case '[', 0x9b:
				for i+1 < len(rs) {
					i++
					if rs[i] >= 0x40 && rs[i] <= 0x7e {
						break
					}
				}
			case ']', 'P', 'X', '^', '_', 0x9d:
				for i+1 < len(rs) {
					i++
					if rs[i] == 7 || rs[i] == 0x9c {
						break
					}
					if rs[i] == 0x1b && i+1 < len(rs) && rs[i+1] == '\\' {
						i++
						break
					}
				}
			default: // Consume remaining ESC intermediates and its final byte.
				for rs[i] >= 0x20 && rs[i] <= 0x2f && i+1 < len(rs) {
					i++
				}
			}
			continue
		}
		if r == '\r' {
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			stripped.WriteByte('\n')
			continue
		}
		if r == '\t' {
			stripped.WriteString("    ")
			continue
		}
		if unicode.IsControl(r) && r != '\n' {
			continue
		}
		stripped.WriteRune(r)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	lines := strings.Split(stripped.String(), "\n")
	out := make([]string, 0, len(lines))
	blank := false
	fence := ""
	for i, line := range lines {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		trim := strings.TrimSpace(line)
		marker := ""
		if strings.HasPrefix(trim, "```") {
			marker = "```"
		} else if strings.HasPrefix(trim, "~~~") {
			marker = "~~~"
		}
		preserve := structured || fence != "" || marker != ""
		if marker != "" {
			switch fence {
			case "":
				fence = marker
			case marker:
				fence = ""
			}
		}
		if preserve {
			line = strings.TrimRightFunc(line, func(r rune) bool { return unicode.IsSpace(r) })
		} else {
			line = strings.Join(strings.Fields(line), " ")
		}
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, line)
	}
	return strings.Trim(strings.Join(out, "\n"), "\n"), nil
}

// Arguments intentionally accept only JSON data, not arbitrary MarshalJSON
// implementations whose output can vary between invocations.
func validateArgument(ctx context.Context, v reflect.Value, depth int, size *int, limit int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 128 {
		return fmt.Errorf("snapcompact: argument nesting exceeds 128")
	}
	*size += 8
	if *size > limit {
		return fmt.Errorf("snapcompact: input byte limit exceeded")
	}
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Interface {
		return validateArgument(ctx, v.Elem(), depth+1, size, limit)
	}
	if v.Type().PkgPath() != "" && v.Type() != reflect.TypeFor[json.Number]() {
		return fmt.Errorf("snapcompact: arguments must contain JSON data values")
	}
	switch v.Kind() {
	case reflect.String:
		text := v.String()
		if !utf8.ValidString(text) {
			return fmt.Errorf("snapcompact: invalid argument UTF-8")
		}
		if len(text) > limit-*size {
			return fmt.Errorf("snapcompact: input byte limit exceeded")
		}
		*size += len(text)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("snapcompact: argument keys must be strings")
		}
		iter := v.MapRange()
		for iter.Next() {
			if err := validateArgument(ctx, iter.Key(), depth+1, size, limit); err != nil {
				return err
			}
			if err := validateArgument(ctx, iter.Value(), depth+1, size, limit); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := validateArgument(ctx, v.Index(i), depth+1, size, limit); err != nil {
				return err
			}
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
	default:
		return fmt.Errorf("snapcompact: arguments must contain JSON data values")
	}
	return nil
}

// avoidEscapeCuts widens the omission to include complete terminal escape
// sequences. Otherwise a retained suffix of an OSC URL could appear as text.
func avoidEscapeCuts(rs []rune, head, tail int) (int, int) {
	for i := 0; i < len(rs); i++ {
		if rs[i] != 0x1b && rs[i] != 0x9b && rs[i] != 0x9d {
			continue
		}
		start := i
		kind := rs[i]
		if kind == 0x1b {
			if i+1 >= len(rs) {
				break
			}
			i++
			kind = rs[i]
		}
		switch kind {
		case '[', 0x9b:
			for i+1 < len(rs) {
				i++
				if rs[i] >= 0x40 && rs[i] <= 0x7e {
					break
				}
			}
		case ']', 'P', 'X', '^', '_', 0x9d:
			for i+1 < len(rs) {
				i++
				if rs[i] == 7 || rs[i] == 0x9c {
					break
				}
				if rs[i] == 0x1b && i+1 < len(rs) && rs[i+1] == '\\' {
					i++
					break
				}
			}
		default:
			for rs[i] >= 0x20 && rs[i] <= 0x2f && i+1 < len(rs) {
				i++
			}
		}
		end := i + 1
		if head > start && head < end {
			head = start
		}
		if tail > start && tail < end {
			tail = end
		}
	}
	return head, tail
}
