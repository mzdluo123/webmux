import type { Meta, StoryObj } from "@storybook/svelte-vite";
import RelayAuthGate from "../src/components/shell/RelayAuthGate.svelte";

const meta = {
  title: "Connection/Official Relay",
  component: RelayAuthGate,
  args: {
    reason: "reconnecting",
    checking: true,
    returnTo: "/s/aaaaaaaa?message=demo",
    onretry: () => {},
  },
} satisfies Meta<typeof RelayAuthGate>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Reconnecting: Story = {};
export const RetryStopped: Story = { args: { checking: false } };
export const Expired: Story = { args: { reason: "expired", checking: false } };
export const Revoked: Story = { args: { reason: "revoked", checking: false } };
export const Login: Story = {
  args: { reason: "unauthenticated", checking: false },
};
