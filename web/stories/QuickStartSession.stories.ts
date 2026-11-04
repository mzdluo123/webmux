import type { Meta, StoryObj } from "@storybook/svelte-vite";
import SessionWithSidebar from "./quick-start/SessionWithSidebar.svelte";

// A hybrid session page inside the app shell (quick-start-chat); the
// sidebar follows the current mode.
const meta = {
  title: "Workspace/Quick start/Session sidebar",
  component: SessionWithSidebar,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof SessionWithSidebar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const ChatMode: Story = { args: { mode: "chat" } };
export const WorkbenchMode: Story = { args: { mode: "workbench" } };
