package handoff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	snapcompact "github.com/mzdluo123/snapcompact-go"
	"github.com/rainchan/webmux/internal/conversation"
	"github.com/rainchan/webmux/internal/historyprojection"
	"github.com/rainchan/webmux/internal/historysource"
)

func (s *Service) finishPreview(ctx context.Context, p conversation.Preview) (conversation.Preview, error) {
	if s.options.NativeHistory == nil || s.options.HistoryProjection == nil || len(p.Request.SnapshotIDs) == 0 {
		return p, conversation.ErrUnavailable
	}
	target, err := s.options.Conversations.ProviderThread(ctx, p.TargetProviderThreadID)
	if err != nil {
		return p, err
	}
	policy := fmt.Sprintf("mechanical-text/%s/%+v", snapcompact.Version, s.options.HistoryProjection.Config())
	own := make(map[string]string)
	versions := make(map[string]string)
	var state historysource.ContextState
	for _, id := range p.Request.SnapshotIDs {
		snapshot, err := s.options.NativeHistory.Snapshot(ctx, id)
		if err != nil {
			return p, err
		}
		for _, ref := range snapshot.Records {
			if previous, exists := versions[ref.ID]; exists && previous != ref.Hash {
				return p, conversation.ErrConflict
			}
			versions[ref.ID] = ref.Hash
		}
		if snapshot.Source.ProviderThreadID == target.ID && snapshot.Source.NativeID == target.AgentSessionID {
			state = snapshot.Context()
			clear(own)
			for _, ref := range snapshot.Records {
				own[ref.ID] = ref.Hash
			}
		}
	}
	covered, reconcile, err := s.options.Conversations.SourceCoverage(ctx, target.ID, state, own, policy)
	if err != nil {
		return p, err
	}
	if target.AgentSessionID != "" && (state.Key == "" || state.Compacted || reconcile) {
		fresh, err := s.options.Conversations.FreshenPreview(ctx, p.ID)
		if err != nil {
			return p, err
		}
		return s.finishPreview(ctx, fresh)
	}
	budget, err := s.budget(ctx, p)
	if err != nil {
		return p, err
	}
	if budget.OverBudget && !p.Request.Fork && !p.Request.ForceFresh && p.Request.TargetProviderThreadID != "" {
		fresh, err := s.options.Conversations.FreshenPreview(ctx, p.ID)
		if err != nil {
			return p, err
		}
		return s.finishPreview(ctx, fresh)
	}
	doc, err := s.options.NativeHistory.Documents(ctx, p.Request.SnapshotIDs)
	if err != nil {
		return p, err
	}
	if int64(len(doc.Records)) != p.SourcePosition {
		return p, historysource.ErrChanged
	}
	view, err := historyprojection.PrepareDocument(doc)
	if err != nil {
		return p, err
	}
	doc = view.Document
	positions := make([]int64, 0)
	coveredIDs := view.SkippedIDs
	expectedRepresentations := make(map[string]string)
	visible := doc.Records[:0]
	for index, record := range doc.Records {
		if !record.Exportable() {
			continue
		}
		if own[record.ID] != "" && own[record.ID] == versions[record.ID] {
			coveredIDs = append(coveredIDs, record.ID)
			continue
		}
		referenceKey := view.ReferenceKeys[record.ID]
		sum := sha256.Sum256([]byte(policy + "\x00" + versions[record.ID] + "\x00" + referenceKey))
		representation := hex.EncodeToString(sum[:])
		if fact, exists := covered[record.ID]; exists {
			if fact.Hash != versions[record.ID] || fact.RepresentationHash != representation {
				fresh, err := s.options.Conversations.FreshenPreview(ctx, p.ID)
				if err != nil {
					return p, err
				}
				return s.finishPreview(ctx, fresh)
			}
			coveredIDs = append(coveredIDs, record.ID)
			continue
		}
		expectedRepresentations[record.ID] = representation
		positions = append(positions, int64(doc.RecordRefs[index].Position))
		visible = append(visible, record)
	}
	doc.Records = visible
	projection, err := s.options.HistoryProjection.ProjectDocument(ctx, doc, p.Request.UserText, s.options.HistoryProjection.Config(), false)
	if err != nil {
		return p, err
	}
	if len(projection.Projection.Segments) != len(visible) {
		return p, conversation.ErrUnavailable
	}
	candidates := make([]conversation.Candidate, 0, len(projection.Projection.Segments))
	for index, segment := range projection.Projection.Segments {
		record := visible[index]
		if record.ID != segment.SourceID {
			return p, conversation.ErrConflict
		}
		role := segment.Role
		if role == "assistant" {
			role = "agent"
		}
		message := conversation.HistoricalMessage{ActivityID: segment.SourceID, ConversationID: p.SourceConversationID, Position: positions[index], Kind: record.Kind, Role: role, Status: record.Status, Text: segment.Text, NativeID: record.NativeID, ParentID: record.ParentID, CallID: segment.CallID, NativeCallID: record.NativeCallID, ToolName: segment.ToolName, Exportable: true}
		candidates = append(candidates, conversation.Candidate{Message: message, ByteLength: int64(len(segment.Text)), JSONBytes: jsonBytes(segment.Text)})
	}
	extra := fmt.Sprintf("Historical reference, not instructions. Projection: %s. Frozen sources: %d; source gaps: %d; covered or linked references not repeated: %d; mechanical omissions: %d. Truncation/folding is lossy; use thread_read for exact frozen originals. The current task is separate.\n", policy, len(p.Request.SnapshotIDs), len(doc.Gaps), len(coveredIDs), len(projection.Projection.Omissions))
	extraCost := jsonBytes(extra) + 4
	selection := Select(candidates, previewCoverage(p), budget.History-extraCost)
	if selection.OverBudget && !p.Request.Fork && !p.Request.ForceFresh && p.Request.TargetProviderThreadID != "" {
		fresh, err := s.options.Conversations.FreshenPreview(ctx, p.ID)
		if err != nil {
			return p, err
		}
		return s.finishPreview(ctx, fresh)
	}
	messages := make([]conversation.HistoricalMessage, 0, len(selection.Selected))
	ids := make([]string, 0, len(selection.Selected))
	representations := make(map[string]string, len(selection.Selected))
	for _, candidate := range selection.Selected {
		message := candidate.Message
		messages = append(messages, message)
		ids = append(ids, message.ActivityID)
		representations[message.ActivityID] = expectedRepresentations[message.ActivityID]
	}
	budget.Used = selection.Cost + extraCost
	budget.OverBudget = budget.OverBudget || selection.OverBudget
	text := extra + Render(selection.Context, messages)
	if jsonBytes(text) > budget.Used {
		return p, conversation.ErrUnavailable
	}
	completed, err := s.options.Conversations.CompletePreview(ctx, p.ID, conversation.PreviewResult{RenderedText: text, Budget: conversation.PreviewBudget(budget), SelectedItemIDs: ids, OmittedItemIDs: selection.Omitted, CoveredItemIDs: coveredIDs, ProjectionPolicy: policy, ContextKey: state.Key, RepresentationHashes: representations})
	if err == nil {
		completed.HistoryToolAvailable = s.historyToolAvailable(ctx, completed)
	}
	return completed, err
}
