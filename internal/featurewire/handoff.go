package featurewire

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/handoff"
)

func HandoffRequest(r contract.HandoffPreviewRequest) handoff.Request {
	return handoff.Request{ConfigID: r.ConfigID, TargetSessionID: r.TargetSessionID, AgentConfig: r.AgentConfig, UserText: r.UserText, Location: handoff.Location(r.Location)}
}
func HandoffPreview(p handoff.Preview) contract.HandoffPreview {
	return contract.HandoffPreview{ID: p.ID, InstanceID: p.InstanceID, OperationID: p.OperationID, SourceSessionID: p.SourceSessionID, SnapshotID: p.SnapshotID, Text: p.Text, UserText: p.UserText, Request: contract.HandoffPreviewRequest{ConfigID: p.Request.ConfigID, TargetSessionID: p.Request.TargetSessionID, AgentConfig: requiredMap(p.Request.AgentConfig), UserText: p.Request.UserText, Location: contract.HandoffLocation(p.Request.Location)}, CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt, SelectedCount: p.SelectedCount, OmittedCount: p.OmittedCount, GapCount: p.GapCount, Budget: contract.HandoffBudget(p.Budget)}
}
func HandoffOperation(o handoff.Operation) contract.HandoffOperation {
	out := contract.HandoffOperation{ID: o.ID, PreviewID: o.PreviewID, InstanceID: o.InstanceID, State: o.State, TargetSessionID: o.TargetSessionID, Error: o.Error, StartedAt: o.StartedAt, FinishedAt: o.FinishedAt}
	if o.Location != nil {
		out.WorktreeID = o.Location.WorktreeID
		out.WorktreePath = o.Location.Path
	}
	return out
}
