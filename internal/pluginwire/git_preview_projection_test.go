package pluginwire

import (
	"encoding/json"
	"testing"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/workspace"
)

// Confirmation consumers render the same agent, view and workspace facts as
// session details. Exercise the named JSON DTOs, not converter source text.
func TestGitConfirmationSessionsRetainPluginProjection(t *testing.T) {
	infos := []session.Info{{ID: "runtime", Cwd: "/worktree", Cmd: []string{"agent"}, Status: session.StatusIdle}}
	providerID := "provider-session"
	metadata := &hybrid.Metadata{
		ConversationID: "conversation", ProviderThreadID: "thread", AgentSessionID: &providerID,
		Side: "acp", State: "active", Agent: "omp", AgentTitle: "Oh My Pi", AgentIcon: "pi", Work: "Read file",
	}
	association := workspace.Association{RepositoryID: "repository", WorktreeID: "worktree", WorktreePath: "/worktree"}
	tests := []struct {
		name    string
		convert func(func(session.Info) contract.SessionInfo) any
	}{
		{"divergence", func(project func(session.Info) contract.SessionInfo) any {
			return GitDivergencePreview(workspace.DivergencePreview{Sessions: infos}, project)
		}},
		{"abort", func(project func(session.Info) contract.SessionInfo) any {
			return GitAbortOperationPreview(workspace.AbortOperationPreview{Sessions: infos}, project)
		}},
		{"stash-create", func(project func(session.Info) contract.SessionInfo) any {
			return GitStashCreatePreview(workspace.StashCreatePreview{Sessions: infos}, project)
		}},
		{"stash-apply", func(project func(session.Info) contract.SessionInfo) any {
			return GitStashApplyPreview(workspace.StashApplyPreview{Sessions: infos}, project)
		}},
		{"branch-rename", func(project func(session.Info) contract.SessionInfo) any {
			return GitBranchRenamePreview(workspace.BranchRenamePreview{Sessions: infos}, project)
		}},
		{"discard", func(project func(session.Info) contract.SessionInfo) any {
			return GitDiscardPreview(workspace.DiscardPreview{Sessions: infos}, project)
		}},
		{"smart-checkout", func(project func(session.Info) contract.SessionInfo) any {
			return GitSmartCheckoutPreview(workspace.SmartCheckoutPreview{Sessions: infos}, project)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			project := func(info session.Info) contract.SessionInfo {
				calls++
				out := (Projection{}).Session(info)
				ProjectChat(&out, metadata)
				value := contract.SessionWorkspace(association)
				out.Workspace = &value
				return out
			}
			data, err := json.Marshal(tt.convert(project))
			if err != nil {
				t.Fatal(err)
			}
			var confirmation struct {
				Sessions []contract.SessionInfo `json:"sessions"`
			}
			if err := json.Unmarshal(data, &confirmation); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || len(confirmation.Sessions) != 1 {
				t.Fatalf("projected %d times, confirmation %s", calls, data)
			}
			got := confirmation.Sessions[0]
			if got.ID != "runtime" || got.Cwd != "/worktree" || got.View != contract.ViewChat || got.State != contract.SessionStateActive {
				t.Fatalf("confirmation lost session/view facts: %s", data)
			}
			if got.Workspace == nil || *got.Workspace != contract.SessionWorkspace(association) {
				t.Fatalf("confirmation lost workspace: %s", data)
			}
			if got.Agent == nil || got.Agent.Name != "omp" || got.Agent.Work != "Read file" || got.Agent.SessionID == nil || *got.Agent.SessionID != providerID {
				t.Fatalf("confirmation lost agent metadata: %s", data)
			}
			if got.ConversationID != "conversation" || got.ProviderThreadID != "thread" {
				t.Fatalf("confirmation lost conversation identity: %s", data)
			}
		})
	}
}
