import type { Meta, StoryObj } from "@storybook/svelte-vite";
import HybridHome from "./acp/HybridHome.svelte";

// Issue #181: hybrid sessions on the existing home page group.
const meta = {
  title: "Prototype/ACP hybrid home",
  component: HybridHome,
} satisfies Meta<typeof HybridHome>;

export default meta;
type Story = StoryObj<typeof meta>;

export const SessionCards: Story = {};
