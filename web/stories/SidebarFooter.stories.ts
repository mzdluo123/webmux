import type { Meta, StoryObj } from "@storybook/svelte-vite";
import SidebarFooter from "../src/components/shell/SidebarFooter.svelte";

const meta = {
  title: "Connection/Host Status Footer",
  component: SidebarFooter,
  args: {
    connection: "reconnecting",
    hostname: "nas",
    setupSettled: true,
    reconnection: {
      checking: true,
      returnTo: "/s/aaaaaaaa?message=demo",
      onretry: () => {},
    },
    onforward: () => {},
  },
} satisfies Meta<typeof SidebarFooter>;
export default meta;
type Story = StoryObj<typeof meta>;

/** On the expanded sidebar the footer's status entry is the lower-left
    reconnect surface; its popover carries retry and the official login. */
export const RelayReconnecting: Story = {};
