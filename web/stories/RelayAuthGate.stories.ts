import type { Meta, StoryObj } from "@storybook/svelte-vite";
import RelayAuthGate from "../src/components/shell/RelayAuthGate.svelte";

const meta = {
  title: "Connection/Official Relay",
  component: RelayAuthGate,
  args: {
    reason: "expired",
    returnTo: "/s/aaaaaaaa?message=demo",
  },
} satisfies Meta<typeof RelayAuthGate>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Expired: Story = {};
export const Revoked: Story = { args: { reason: "revoked" } };
export const Login: Story = {
  args: { reason: "unauthenticated" },
};
