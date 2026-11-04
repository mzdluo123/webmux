import type { Meta, StoryObj } from "@storybook/svelte-vite";
import SessionWithSidebar from "./quick-start/SessionWithSidebar.svelte";

// The chat sidebar beside a hybrid session page (quick-start-chat).
const meta = {
  title: "Workspace/Quick start/Session sidebar",
  component: SessionWithSidebar,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof SessionWithSidebar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const ChatSession: Story = {};
