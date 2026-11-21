package featurehub

import (
	"net/url"

	"github.com/rainchan/webmux/internal/hybrid"
)

const (
	// tailBackfillBudget targets ~128 KiB per initial tail or backfill frame
	// so the browser paints the latest screenful without blocking on a 2 MiB
	// JSON parse, then fills earlier history in small non-blocking chunks.
	tailBackfillBudget = 128 << 10
	// tailBackfillMinEvents skips splitting small backlogs.
	tailBackfillMinEvents = 32
)

// ChatOptions configures how featurehub.Chat delivers the initial backlog.
type ChatOptions struct {
	TailBackfill bool
}

// ChatBackfillTail reports whether the client opted into tail-first chunked
// backlog delivery (?backfill=tail).
func ChatBackfillTail(q url.Values) bool {
	return q.Get("backfill") == "tail"
}

// splitBacklogTailFirst splits events into a newest tail slice and zero or
// more earlier backfill chunks ordered from newest-earlier to oldest, so each
// backfill chunk prepends cleanly before what the client already holds.
func splitBacklogTailFirst(events []hybrid.Event, budget, minEvents int) (tail []hybrid.Event, backfill [][]hybrid.Event) {
	if len(events) <= minEvents || budget <= 0 {
		return events, nil
	}
	sizes := make([]int, len(events))
	total := 0
	for i, e := range events {
		sz := estimateChatEventBytes(e)
		sizes[i] = sz
		total += sz
	}
	if total <= budget {
		return events, nil
	}
	tailStart := chooseChunkStart(events, sizes, len(events), budget)
	if tailStart == 0 {
		return events, nil
	}
	tail = events[tailStart:]
	for end := tailStart; end > 0; {
		start := chooseChunkStart(events, sizes, end, budget)
		backfill = append(backfill, events[start:end])
		end = start
	}
	return tail, backfill
}

// chooseChunkStart scans backwards from end until budget is reached (always
// including at least one event), then snaps to a nearby turn.start boundary
// when doing so keeps the chunk within 2*budget so turn diffs and permission
// pairs stay together whenever practical.
func chooseChunkStart(events []hybrid.Event, sizes []int, end, budget int) int {
	start := end - 1
	acc := sizes[start]
	for start > 0 && acc+sizes[start-1] <= budget {
		start--
		acc += sizes[start]
	}
	if start == 0 || events[start].Kind == hybrid.EventTurnStart {
		return start
	}
	snapLimit := budget * 2
	probe, probeAcc := start, acc
	for probe > 0 && probeAcc+sizes[probe-1] <= snapLimit {
		probe--
		probeAcc += sizes[probe]
		if events[probe].Kind == hybrid.EventTurnStart {
			return probe
		}
	}
	return start
}

func estimateChatEventBytes(e hybrid.Event) int {
	n := 128 + len(e.Kind)
	if m := e.Message; m != nil {
		n += len(m.MessageID) + len(m.Role) + len(m.Text)
		for _, a := range m.Attachments {
			n += 64 + len(a.Path) + len(a.Name) + len(a.MimeType)
		}
	}
	if t := e.Tool; t != nil {
		n += len(t.ToolCallID)
		if t.Kind != nil {
			n += len(*t.Kind)
		}
		if t.Title != nil {
			n += len(*t.Title)
		}
		if t.Status != nil {
			n += len(*t.Status)
		}
		for _, c := range t.Content {
			n += 96 + len(c.Type) + len(c.Text) + len(c.Path) + len(c.NewText) + len(c.URI) + len(c.Name) + len(c.MimeType) + len(c.Unsupported)
			if c.OldText != nil {
				n += len(*c.OldText)
			}
		}
		for _, loc := range t.Locations {
			n += 24 + len(loc.Path)
		}
	}
	if c := e.Compaction; c != nil {
		n += len(c.ID) + len(c.Status) + len(c.Summary)
	}
	if c := e.Notice; c != nil {
		n += len(c.Level) + len(c.Text)
	}
	if t := e.TurnStart; t != nil {
		n += len(t.TurnID) + len(t.PromptID) + len(t.QueueItemID)
	}
	if t := e.TurnEnd; t != nil {
		n += len(t.TurnID) + len(t.StopReason) + len(t.Error)
	}
	if p := e.Plan; p != nil {
		for _, entry := range p.Entries {
			n += 48 + len(entry.Content) + len(entry.Priority) + len(entry.Status)
		}
	}
	for _, cmd := range e.Commands {
		n += 48 + len(cmd.Name) + len(cmd.Description) + len(cmd.Hint)
	}
	for _, cfg := range e.Config {
		n += 96 + len(cfg.ID) + len(cfg.Name) + len(cfg.Value)
		for _, opt := range cfg.Options {
			n += 48 + len(opt.Value) + len(opt.Name) + len(opt.Description)
		}
	}
	if r := e.Request; r != nil {
		n += 96 + len(r.RequestID) + len(r.ToolCallID) + len(r.Title)
	}
	return n
}
