package featurewire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/handoff"
)

func HandoffRequest(r contract.HandoffPreviewRequest) handoff.Request {
	return handoff.Request{Format: r.Format, MaxPages: r.MaxPages, ConfigID: r.ConfigID, TargetSessionID: r.TargetSessionID, AgentConfig: r.AgentConfig, UserText: r.UserText, Location: handoff.Location(r.Location)}
}
func HandoffPreview(p handoff.Preview) contract.HandoffPreview {
	pages := make([]contract.HandoffImagePage, len(p.Pages))
	for i, page := range p.Pages {
		pages[i] = contract.HandoffImagePage(page)
	}
	return contract.HandoffPreview{
		Format: p.Format, Pages: pages, ImageBudget: contract.HandoffImageBudget(p.ImageBudget),
		BlockedReason: p.BlockedReason, Capability: p.Capability, HistoryText: p.HistoryText,
		ID: p.ID, InstanceID: p.InstanceID, OperationID: p.OperationID,
		SourceSessionID: p.SourceSessionID, SnapshotID: p.SnapshotID, Text: p.Text, UserText: p.UserText,
		Request: contract.HandoffPreviewRequest{
			Format: p.Request.Format, MaxPages: p.Request.MaxPages,
			ConfigID: p.Request.ConfigID, TargetSessionID: p.Request.TargetSessionID,
			AgentConfig: requiredMap(p.Request.AgentConfig), UserText: p.Request.UserText,
			Location: contract.HandoffLocation(p.Request.Location),
		},
		CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt,
		SelectedCount: p.SelectedCount, OmittedCount: p.OmittedCount, GapCount: p.GapCount,
		Budget: contract.HandoffBudget(p.Budget),
	}
}
func HandoffOperation(o handoff.Operation) contract.HandoffOperation {
	out := contract.HandoffOperation{ID: o.ID, PreviewID: o.PreviewID, InstanceID: o.InstanceID, State: o.State, TargetSessionID: o.TargetSessionID, Error: o.Error, StartedAt: o.StartedAt, FinishedAt: o.FinishedAt}
	if o.WorktreeUpstream != nil {
		check := contract.WorktreeUpstream(*o.WorktreeUpstream)
		out.WorktreeUpstream = &check
	}
	if o.Location != nil {
		out.WorktreeID = o.Location.WorktreeID
		out.WorktreePath = o.Location.Path
	}
	return out
}
