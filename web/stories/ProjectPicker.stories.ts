import type { Meta, StoryObj } from "@storybook/svelte-vite";
import ProjectPickerFlow from "./ProjectPickerFlow.svelte";

const meta = {
  title: "Workspace/Project picker",
  component: ProjectPickerFlow,
} satisfies Meta<typeof ProjectPickerFlow>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Projects: Story = {};
export const Empty: Story = { args: { scene: "empty" } };
export const Sources: Story = { args: { scene: "sources" } };
export const NewProject: Story = { args: { scene: "new" } };
export const LocalFolder: Story = { args: { scene: "local" } };
export const GitURL: Story = { args: { scene: "git" } };
export const GitHub: Story = { args: { scene: "github" } };
export const GitHubUnavailable: Story = {
  args: { scene: "github-unavailable" },
};
export const RegistrationFailure: Story = {
  args: { scene: "registration-failure" },
};
