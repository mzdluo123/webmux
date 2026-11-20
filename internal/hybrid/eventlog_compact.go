package hybrid

// isStateOnlyEvent reports whether kind only carries replaceable state
// (plan, commands, config, usage) where only the latest snapshot matters.
// Keeping at most one stored event per kind prevents repeated updates across
// long sessions from consuming scrollback budget or bloating initial replays
// while still delivering every live event on sub.Out.
func isStateOnlyEvent(kind string) bool {
	switch kind {
	case EventPlan, EventCommands, EventConfig, EventUsage:
		return true
	default:
		return false
	}
}

// pruneSupersededStateLocked removes any earlier stored event of the same
// replaceable state-only kind before appending the newer update.
func (l *chatLog) pruneSupersededStateLocked(kind string) {
	if !isStateOnlyEvent(kind) {
		return
	}
	for i := len(l.events) - 1; i >= 0; i-- {
		if l.events[i].Kind != kind {
			continue
		}
		l.bytes -= l.sizes[i]
		copy(l.events[i:], l.events[i+1:])
		l.events[len(l.events)-1] = Event{}
		l.events = l.events[:len(l.events)-1]
		copy(l.sizes[i:], l.sizes[i+1:])
		l.sizes = l.sizes[:len(l.sizes)-1]
		copy(l.spans[i:], l.spans[i+1:])
		l.spans = l.spans[:len(l.spans)-1]
		if i < l.degraded {
			l.degraded--
		}
		return
	}
}

func isTerminalToolStatus(status *string) bool {
	return status != nil && (*status == "completed" || *status == "failed")
}

func isActiveToolStatus(status *string) bool {
	return status != nil && (*status == "pending" || *status == "in_progress")
}

// canMergeToolPatch reports whether next can be folded into prev without
// changing the resulting tool entry or turn-diff ownership. Restarting a
// completed/failed tool call changes its owning turn diff and stays separate.
func canMergeToolPatch(prev, next *Tool) bool {
	if prev == nil || next == nil || prev.ToolCallID == "" || prev.ToolCallID != next.ToolCallID {
		return false
	}
	if isTerminalToolStatus(prev.Status) && isActiveToolStatus(next.Status) {
		return false
	}
	return true
}

// mergeToolPatch returns a new Tool combining prev with non-nil fields from
// next. Neither input is mutated.
func mergeToolPatch(prev, next *Tool) *Tool {
	merged := *prev
	if next.Kind != nil {
		merged.Kind = next.Kind
	}
	if next.Title != nil {
		merged.Title = next.Title
	}
	if next.Status != nil {
		merged.Status = next.Status
	}
	if next.Content != nil {
		merged.Content = next.Content
	}
	if next.Locations != nil {
		merged.Locations = next.Locations
	}
	return &merged
}

func hasDiffContent(t *Tool) bool {
	if t == nil {
		return false
	}
	for _, c := range t.Content {
		if c.Type == "diff" && c.Path != "" {
			return true
		}
	}
	return false
}

// mergeConsecutiveToolLocked folds e into the trailing tool.upsert event when
// it patches the same tool call. The merged event keeps the earlier timestamp
// and span.first while taking e.Seq so resuming mid-call replays the complete
// merged state idempotently.
func (l *chatLog) mergeConsecutiveToolLocked(e Event) bool {
	n := len(l.events)
	if n == 0 || e.Kind != EventTool || l.events[n-1].Kind != EventTool {
		return false
	}
	prev := l.events[n-1]
	if !canMergeToolPatch(prev.Tool, e.Tool) {
		return false
	}
	merged := Event{
		Seq:  e.Seq,
		Kind: EventTool,
		At:   prev.At,
		Tool: mergeToolPatch(prev.Tool, e.Tool),
	}
	size := eventSize(merged)
	l.bytes += size - l.sizes[n-1]
	l.events[n-1] = merged
	l.sizes[n-1] = size
	l.degraded = min(l.degraded, n-1)
	return true
}

// compactBacklog returns a fresh slice with interleaved tool.upsert patches
// for the same tool call within a turn folded into the first occurrence's slot.
// Folding preserves timeline order, strictly increasing Seq values, the
// trailing sequence number (so Snapshot.Next-1 stays intact), and
// last-writer-wins turn diff ordering across distinct tools.
func compactBacklog(events []Event) []Event {
	if len(events) < 2 {
		return append([]Event(nil), events...)
	}
	type slot struct {
		idx       int
		diffEpoch int
	}
	out := make([]Event, 0, len(events))
	active := make(map[string]slot)
	diffEpoch := 0
	lastIdx := len(events) - 1
	for i, e := range events {
		switch e.Kind {
		case EventTurnStart, EventTurnEnd:
			clear(active)
			out = append(out, e)
		case EventTool:
			if e.Tool == nil {
				out = append(out, e)
				continue
			}
			hasDiff := hasDiffContent(e.Tool)
			if i < lastIdx {
				if s, ok := active[e.Tool.ToolCallID]; ok && canMergeToolPatch(out[s.idx].Tool, e.Tool) && (!hasDiff || s.diffEpoch == diffEpoch) {
					merged := out[s.idx]
					merged.Tool = mergeToolPatch(merged.Tool, e.Tool)
					out[s.idx] = merged
					if hasDiff {
						diffEpoch++
						active[e.Tool.ToolCallID] = slot{idx: s.idx, diffEpoch: diffEpoch}
					}
					continue
				}
			}
			if hasDiff {
				diffEpoch++
			}
			active[e.Tool.ToolCallID] = slot{idx: len(out), diffEpoch: diffEpoch}
			out = append(out, e)
		default:
			out = append(out, e)
		}
	}
	return out
}
