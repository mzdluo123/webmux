import type { Meta, StoryObj } from "@storybook/svelte-vite";
import RelayReconnectIndicator from "../src/components/shell/RelayReconnectIndicator.svelte";

const meta = {
  title: "Connection/Relay Reconnect Indicator",
  component: RelayReconnectIndicator,
  args: {
    checking: true,
    returnTo: "/s/aaaaaaaa?message=demo",
    onretry: () => {},
  },
} satisfies Meta<typeof RelayReconnectIndicator>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Reconnecting: Story = {};
export const RetryStopped: Story = { args: { checking: false } };
