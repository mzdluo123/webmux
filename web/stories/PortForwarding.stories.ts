import type { Meta, StoryObj } from "@storybook/svelte-vite";
import ForwardingHome from "./port-forwarding/ForwardingHome.svelte";

// The home page port-forwarding entry and dialog (openspec change
// `port-forwarding`). Prototype only: sample data, no server.
const meta = {
  title: "Workspace/Port forwarding",
  component: ForwardingHome,
} satisfies Meta<typeof ForwardingHome>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Dialog: Story = { args: { dialog: true } };
export const Editing: Story = { args: { dialog: true, editing: 5173 } };
export const Empty: Story = { args: { dialog: true, empty: true } };
export const Unsupported: Story = {
  args: { dialog: true, empty: true, detection: "unsupported" },
};
export const HeaderOnly: Story = { args: { dialog: false } };
