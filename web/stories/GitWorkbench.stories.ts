import type { Meta, StoryObj } from "@storybook/svelte-vite";
import GitWorkbench from "./GitWorkbench.svelte";

const meta = {
  title: "Workspace/Git workbench",
  component: GitWorkbench,
} satisfies Meta<typeof GitWorkbench>;

export default meta;
type Story = StoryObj<typeof meta>;

export const LogMergeTopology: Story = {
  args: { scenario: "log-merge" },
};

export const CommitHunkStaging: Story = {
  args: { scenario: "commit-staging" },
};

export const CommitFullIndexPreview: Story = {
  args: { scenario: "commit-preview" },
};

export const TaskBaselineCleanCommittedReview: Story = {
  args: { scenario: "task-baseline-review" },
};

export const BranchesWorktreeOccupied: Story = {
  args: { scenario: "branches-occupied" },
};

export const PushConfirmationDialog: Story = {
  args: { scenario: "push-dialog" },
};

export const NoWorktreeReadonly: Story = {
  args: { scenario: "no-worktree" },
};

export const EmptyUnbornRepository: Story = {
  args: { scenario: "empty-unborn" },
};

export const IndexLockBusy: Story = {
  args: { scenario: "index-lock-busy" },
};

export const CommitNeedsReview: Story = {
  args: { scenario: "commit-needs-review" },
};

export const OperationUncertainRecovery: Story = {
  args: { scenario: "operation-uncertain" },
};

export const CommitDraftConflict409: Story = {
  args: { scenario: "commit-draft-conflict" },
};

export const ReworkSessionHandoff: Story = {
  args: { scenario: "rework-session-handoff" },
};

export const ConflictThreeWayText: Story = {
  args: { scenario: "conflict-three-way-text" },
};

export const ConflictBinaryVariant: Story = {
  args: { scenario: "conflict-binary-variant" },
};

export const DivergenceRebasePreview: Story = {
  args: { scenario: "divergence-rebase-preview" },
};

export const AbortOperationDialog: Story = {
  args: { scenario: "abort-operation-dialog" },
};

export const StashViewUntracked: Story = {
  args: { scenario: "stash-view-untracked" },
};

export const StashPopConflictPreserved: Story = {
  args: { scenario: "stash-pop-conflict" },
};

export const CommitAmendPushedWarning: Story = {
  args: { scenario: "commit-amend-pushed" },
};

export const BranchesTagsAndRemotes: Story = {
  args: { scenario: "branches-tags-remotes" },
};

export const FileHistoryAndBlame: Story = {
  args: { scenario: "file-history-blame" },
};

export const SubHunkLineStaging: Story = {
  args: { scenario: "sub-hunk-line-staging" },
};

export const DiscardPreviewDialog: Story = {
  args: { scenario: "discard-preview-dialog" },
};
