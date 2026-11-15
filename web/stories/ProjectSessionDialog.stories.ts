import type { Meta, StoryObj } from "@storybook/svelte-vite";
import ProjectSessionDialog from "./ProjectSessionDialog.svelte";
const meta = {
  title: "Workspace/Project session",
  component: ProjectSessionDialog,
} satisfies Meta<typeof ProjectSessionDialog>;
export default meta;
type Story = StoryObj<typeof meta>;
export const PlainFolder: Story = {};
export const GitProject: Story = { args: { scene: "git" } };
export const UnavailableProject: Story = { args: { scene: "unavailable" } };
export const LaunchUnavailable: Story = {
  args: { scene: "launch-unavailable" },
};
