import type { Meta, StoryObj } from "@storybook/svelte-vite";
import PortPrompt from "./port-forwarding/PortPrompt.svelte";

// The terminal page prompt for newly listening ports (openspec change
// `port-forwarding`). Prototype only: sample data, no server.
const meta = {
  title: "Terminal/Port prompt",
  component: PortPrompt,
} satisfies Meta<typeof PortPrompt>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NewPort: Story = { args: { scenario: "new" } };
export const MultiplePorts: Story = { args: { scenario: "multiple" } };
export const Forwarded: Story = { args: { scenario: "forwarded" } };
export const Failed: Story = { args: { scenario: "failed" } };
