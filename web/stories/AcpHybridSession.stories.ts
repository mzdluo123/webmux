import type { Meta, StoryObj } from "@storybook/svelte-vite";
import HybridSession from "./acp/HybridSession.svelte";

// Interactive prototype for issue #181: the existing session page with a
// hybrid session that switches between the ACP chat view and its TUI.
// Sample data only: no ACP process, PTY or API request is involved.
const meta = {
  title: "Prototype/ACP hybrid session",
  component: HybridSession,
} satisfies Meta<typeof HybridSession>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Conversation: Story = { args: { scenario: "conversation" } };
export const PermissionRequest: Story = { args: { scenario: "permission" } };
export const CommandMenu: Story = { args: { scenario: "commands" } };
export const CancelledTurn: Story = { args: { scenario: "cancelled" } };
export const OpenCode: Story = {
  args: { scenario: "conversation", agent: "opencode" },
};
export const Switching: Story = { args: { scenario: "switching" } };
export const TerminalSide: Story = { args: { scenario: "terminal" } };
