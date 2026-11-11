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
