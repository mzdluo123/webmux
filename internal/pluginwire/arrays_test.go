package pluginwire

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/hybrid"
	"github.com/rainchan/webmux/internal/launch"
	"github.com/rainchan/webmux/internal/push"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/sessioncreate"
	"github.com/rainchan/webmux/internal/tailscale"
	corewire "github.com/rainchan/webmux/internal/wire"
	"github.com/rainchan/webmux/internal/workspace"
)

var updateArrays = flag.Bool("update-arrays", false, "re-record the response array fixture consumed by frontend Zod tests")

var chatTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// This fixture exercises real Go conversions with nil service arrays and is
// parsed by web/test/wireArrays.test.ts. Keep scalar validation constraints
// intact: command arrays still need a command, and nullable fields stay null.
func TestResponseArrayFixture(t *testing.T) {
	values := map[string]any{
		"sessions":          corewire.Sessions(nil),
		"snapshot":          corewire.SessionEvent(session.Event{Type: "snapshot"}, nil),
		"session":           corewire.Session(session.Info{ID: "abcdefgh", Generation: "0123456789abcdef", Cmd: []string{"sh"}, Status: session.StatusRunning, Cols: 80, Rows: 24}),
		"repository":        Repository(workspace.Repository{Availability: "unavailable"}),
		"repositories":      Repositories(nil),
		"branches":          Branches(workspace.Branches{Branches: []workspace.Branch{{Name: "main"}}}),
		"remotes":           Remotes(workspace.Remotes{}),
		"preview":           Preview(workspace.Preview{Worktree: workspace.Worktree{Kind: "main"}}, func(hybrid.Record) Archived { return Archived{} }, nil),
		"batchPreview":      BatchPreview(workspace.BatchPreview{Items: []workspace.BatchPreviewItem{{Worktree: workspace.Worktree{Kind: "linked"}}}}, func(hybrid.Record) Archived { return Archived{} }, nil),
		"batchRemoveResult": BatchRemoveResult(workspace.BatchRemoveResult{}),
		"launch":            LaunchSummaries([]launch.Summary{{ID: "abcdefgh", Name: "Shell", Cmd: []string{"sh"}}}, nil),
		"launchConfig":      LaunchConfig(launch.Config{ID: "abcdefgh", Name: "Shell", Cmd: []string{"sh"}}),
		"launchPresets":     LaunchPresets([]launch.Preset{{Name: "Shell", Cmd: []string{"sh"}}}),
		"check":             CodeCheck(codeview.Check{Changes: "unchanged"}),
		"launches":          LaunchSummaries(nil, nil),
		"createResult":      corewire.CreateResult(sessioncreate.Result{}, session.Info{ID: "abcdefgh", Cmd: []string{"sh"}, Status: session.StatusRunning}),
		"push":              PushTestReport(push.TestReport{}),
		"listing":           CodeListing(codeview.Listing{}),
		"changes":           CodeChanges(codeview.Changes{}),
		"tailscale":         TailscaleStatus(tailscale.Status{Provider: "none", ConfiguredProvider: "auto"}),
		"chatSnapshot":      ChatSnapshot(hybrid.Snapshot{State: hybrid.ChatState{Side: hybrid.SideACP, Agent: "omp", Plan: &hybrid.Plan{}}, Generation: "0123456789abcdef", From: 1, Next: 1}),
		"chatEvents": ChatEvents("0123456789abcdef", []hybrid.Event{
			{Seq: 1, Kind: hybrid.EventCommands, At: chatTime},
			{Seq: 2, Kind: hybrid.EventConfig, At: chatTime, Config: []hybrid.ConfigOption{{ID: "model", Name: "Model", Type: "select"}}},
			{Seq: 3, Kind: hybrid.EventRequest, At: chatTime, Request: &hybrid.Request{RequestID: "r1", Kind: hybrid.RequestForm, Form: &hybrid.Form{Message: "?"}}},
		}),
		"archivedSession": ArchivedSession(hybrid.Record{
			ID: "abcdefgh", Name: "omp", Title: "session title", ConfigID: "abcdefgh", Agent: "omp", Cwd: "/w", Cmd: []string{"omp"},
			Side: hybrid.SideTUI, State: hybrid.StateClosed, CreatedAt: chatTime, UpdatedAt: chatTime,
			Workspace: &hybrid.WorkspaceRef{RepositoryID: "repo0001", WorktreeID: "tree0001"},
		}, Archived{AgentTitle: "Oh My Pi", AgentIcon: "pi"}),
		"gitChanges":              GitChanges(codeview.Changes{}),
		"gitCompare":              GitCompare(codeview.Compare{TargetOID: "0123456789abcdef0123456789abcdef01234567"}),
		"gitHistory":              GitHistoryPage(workspace.HistoryPage{Status: workspace.HistoryStatusReady, Scope: workspace.HistoryScopeAll, Commits: []workspace.HistoryRow{{OID: "0123456789abcdef0123456789abcdef01234567", ShortOID: "01234567", Subject: "init", AuthorAt: "2026-01-02T03:04:05Z", CommitterAt: "2026-01-02T03:04:05Z"}}}),
		"gitCommitDetail":         GitCommitDetail(workspace.CommitDetail{OID: "0123456789abcdef0123456789abcdef01234567", ShortOID: "01234567", Subject: "init", Message: "init", AuthorAt: "2026-01-02T03:04:05Z", CommitterAt: "2026-01-02T03:04:05Z"}),
		"gitStaging":              GitStagingOverview(workspace.StagingOverview{}),
		"gitStagingDiff":          GitStagingFileDiff(workspace.StagingFileDiff{Identity: workspace.StageFileIdentity{Area: workspace.StageAreaUnstaged, Path: "a.txt"}, Hunks: []workspace.StagingHunk{{ID: "h1", Header: "@@ -1 +1 @@"}}}),
		"gitCommitPreview":        GitCommitPreview(workspace.CommitPreview{RepositoryID: "repo0001", WorktreeID: "tree0001", WorktreePath: "/w", HeadState: "branch", Branch: "main", IndexDigest: "digest"}),
		"gitCommitResult":         GitCommitResult(workspace.CommitResult{OperationID: "op000001", Outcome: workspace.CommitOutcomeMatched, CommitOID: "0123456789abcdef0123456789abcdef01234567", TreeOID: "fedcba9876543210fedcba9876543210fedcba98", ExpectedTreeOID: "fedcba9876543210fedcba9876543210fedcba98"}),
		"gitOperation":            GitOperation(workspace.GitOperation{ID: "op000001", RepositoryID: "repo0001", WorktreeID: "tree0001", Kind: workspace.GitOpCommit, Status: workspace.GitOpSucceeded, StartedAt: chatTime, Summary: "committed"}),
		"gitOperations":           GitOperations(nil),
		"gitReviews":              GitReviewsResponse(workspace.ReviewsOverview{Scope: workspace.ReviewScopeUncommitted, Status: workspace.TaskScopeReady, Applied: true}),
		"gitInProgress":           GitInProgressOperation(workspace.InProgressOperation{WorktreeID: "tree0001", WorktreePath: "/w"}),
		"gitDivergencePreview":    GitDivergencePreview(workspace.DivergencePreview{Kind: workspace.GitOpMerge, RepositoryID: "repo0001", WorktreeID: "tree0001", WorktreePath: "/w", DirectionSummary: "merge", RecoveryHint: "abort", TargetRef: "origin/main", TargetKind: "remote", TargetOID: "0123456789abcdef0123456789abcdef01234567", TargetSubject: "init"}, nil),
		"gitAbortPreview":         GitAbortOperationPreview(workspace.AbortOperationPreview{RepositoryID: "repo0001", WorktreeID: "tree0001", WorktreePath: "/w", Kind: "merge", DirectionSummary: "abort"}, nil),
		"gitDivergenceResult":     GitDivergenceResult(workspace.DivergenceResult{Kind: workspace.GitOpMerge, Outcome: workspace.DivergenceOutcomeCompleted, Summary: "done", InProgress: workspace.InProgressOperation{WorktreeID: "tree0001", WorktreePath: "/w"}, Worktree: workspace.Worktree{Kind: "main"}}),
		"gitConflictDetail":       GitConflictFileDetail(workspace.ConflictFileDetail{RepositoryID: "repo0001", WorktreeID: "tree0001", WorktreePath: "/w", Path: "a.txt", XY: "UU", Kind: workspace.ConflictKindText, TextMergeable: true, Identity: workspace.ConflictSnapshotIdentity{Path: "a.txt", LeftStage: 2, RightStage: 3}, Base: workspace.ConflictSide{Side: "base", Label: "base", Role: "base", Stage: 1}, Left: workspace.ConflictSide{Side: "left", Label: "left", Role: "left", Stage: 2}, Right: workspace.ConflictSide{Side: "right", Label: "right", Role: "right", Stage: 3}, Segments: []workspace.ConflictSegment{{ID: "seg-0", Kind: workspace.SegmentConflict, BaseStart: 1, LeftStart: 1, RightStart: 1}}}),
		"gitConflictResolve":      GitResolveConflictResponse(workspace.ResolveConflictResponse{Path: "a.txt", InProgress: workspace.InProgressOperation{WorktreeID: "tree0001", WorktreePath: "/w"}}),
		"gitStashList":            GitStashList(nil),
		"gitStashDetail":          GitStashDetail(workspace.StashDetail{Entry: workspace.StashEntry{StashRef: "stash@{0}", OID: "0123456789abcdef0123456789abcdef01234567", ShortOID: "01234567", Summary: "WIP on main", CreatedAt: "2026-01-02T03:04:05Z"}}),
		"gitStashCreatePreview":   GitStashCreatePreview(workspace.StashCreatePreview{RepositoryID: "repo0001", WorktreeID: "tree0001", WorktreePath: "/w"}, nil),
		"gitStashCreateResult":    GitStashCreateResult(workspace.StashCreateResult{Entry: workspace.StashEntry{StashRef: "stash@{0}", OID: "0123456789abcdef0123456789abcdef01234567", ShortOID: "01234567", Summary: "WIP on main", CreatedAt: "2026-01-02T03:04:05Z"}, Worktree: workspace.Worktree{Kind: "main"}}),
		"gitStashApplyPreview":    GitStashApplyPreview(workspace.StashApplyPreview{RepositoryID: "repo0001", WorktreeID: "tree0001", WorktreePath: "/w", Mode: "apply", Entry: workspace.StashEntry{StashRef: "stash@{0}", OID: "0123456789abcdef0123456789abcdef01234567", ShortOID: "01234567", Summary: "WIP on main", CreatedAt: "2026-01-02T03:04:05Z"}}, nil),
		"gitStashApplyResult":     GitStashApplyResult(workspace.StashApplyResult{Kind: workspace.GitOpStashApply, Outcome: workspace.StashApplyCompleted, StashRef: "stash@{0}", StashOID: "0123456789abcdef0123456789abcdef01234567", Summary: "applied", InProgress: workspace.InProgressOperation{WorktreeID: "tree0001", WorktreePath: "/w"}, Worktree: workspace.Worktree{Kind: "main"}}),
		"gitStashDropPreview":     GitStashDropPreview(workspace.StashDropPreview{RepositoryID: "repo0001", Entry: workspace.StashEntry{StashRef: "stash@{0}", OID: "0123456789abcdef0123456789abcdef01234567", ShortOID: "01234567", Summary: "WIP on main", CreatedAt: "2026-01-02T03:04:05Z"}, RecoveryCommand: "git stash apply 0123456789abcdef0123456789abcdef01234567"}),
		"gitStashDropResult":      GitStashDropResult(workspace.StashDropResult{StashRef: "stash@{0}", StashOID: "0123456789abcdef0123456789abcdef01234567", RecoveryCommand: "git stash apply 0123456789abcdef0123456789abcdef01234567"}),
		"gitBranchRenamePreview":  GitBranchRenamePreview(workspace.BranchRenamePreview{RepositoryID: "repo0001", OldName: "feat", NewName: "feat-v2", HeadOID: "0123456789abcdef0123456789abcdef01234567", Subject: "init"}, nil),
		"gitBranchMutationResult": GitBranchMutationResult(workspace.BranchMutationResult{Branch: "feat-v2", Repository: workspace.Repository{Availability: "unavailable"}}),
		"gitTagCreateResult":      GitTagCreateResult(workspace.CreateTagResult{Tag: workspace.Tag{Name: "v1.0.0", Head: "0123456789abcdef0123456789abcdef01234567", Subject: "init", CommittedAt: "2026-01-02T03:04:05Z", Kind: "lightweight"}}),
		"gitTagDeleteResult":      GitTagDeleteResult(workspace.DeleteTagResult{Name: "v1.0.0", TargetOID: "0123456789abcdef0123456789abcdef01234567", RecoveryCommand: "git tag v1.0.0 0123456789abcdef0123456789abcdef01234567"}),
		"gitRemoteMutationResult": GitRemoteMutationResult(workspace.RemoteMutationResult{Repository: workspace.Repository{Availability: "unavailable"}}),
		"gitFileHistory":          GitFileHistoryPage(workspace.FileHistoryPage{Status: workspace.HistoryStatusReady, Path: "a.txt", Commits: []workspace.FileHistoryRow{{OID: "0123456789abcdef0123456789abcdef01234567", ShortOID: "01234567", Subject: "init", AuthorAt: "2026-01-02T03:04:05Z", Status: "A", Path: "a.txt"}}}),
		"gitBlame":                GitBlameResult(workspace.BlameResult{CommitOID: "0123456789abcdef0123456789abcdef01234567", ShortOID: "01234567", Path: "a.txt", Kind: "text"}),
	}
	got, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("..", "wire", "testdata", "response-arrays.json")
	if *updateArrays {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("response array fixture changed; review with Zod schemas, then go test ./internal/wire -run TestResponseArrayFixture -update-arrays")
	}
}
