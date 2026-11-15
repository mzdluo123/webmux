import type { Meta, StoryObj } from "@storybook/svelte-vite";
import RelaySettings from "../src/components/settings/RelaySettings.svelte";
import type { RelayStatus } from "../src/contracts/relay-schemas";
import { relayStatus as status } from "../test/fixtures/relay";

const ready: RelayStatus = {
  ...status,
  enabled: true,
  paired: true,
  state: "ready",
  ownerId: "1234567",
  ownerLogin: "relay-owner",
  controlReady: true,
  bulkReady: true,
  readyConnections: 2,
  boundPublicUrl:
    "https://h-0123456789abcdef0123456789abcdef.relay.example.test",
  hybridSupported: true,
  hybridUnavailableReason: "",
  publicUrl: "https://h-0123456789abcdef0123456789abcdef.relay.example.test",
  controlReachability: "reachable",
  dataReachability: "reachable",
  connectorUrl: "wss://data.example.test/v1/connector",
  dataOrigin: "https://data.example.test",
  assignmentRevision: "2",
  configRevision: "3",
  appliedRevision: "3",
  nodeId: "wss-west",
  capabilities: ["wss"],
  assignmentSource: "authenticated-control",
  tcpAlgorithm: "bbr",
  sessions: [
    {
      id: "browser-a",
      path: "webrtc-turn",
      candidateType: "relay/host",
      transport: "webrtc",
      reason: "stable-latency-advantage",
    },
  ],
};
const meta = {
  title: "Settings/Official Relay",
  component: RelaySettings,
  args: {
    view: {
      status,
      enrollment: undefined,
      loading: false,
      busy: false,
      error: "",
      notice: "",
    },
    onrefresh: () => {},
    onpair: () => {},
    oncancel: () => {},
    onenable: () => {},
    ondisconnect: () => {},
    onreconnect: () => {},
    ondelete: () => {},
  },
} satisfies Meta<typeof RelaySettings>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Loading: Story = {
  args: { view: { ...meta.args.view, status: undefined, loading: true } },
};
export const Unpaired: Story = {};
export const Pairing: Story = {
  args: {
    view: {
      ...meta.args.view,
      status: { ...status, pairing: true },
      enrollment: {
        version: 1,
        handle: "A".repeat(43),
        userCode: "ABCD-EFGH",
        verificationUrl: "https://github.com/login/device",
        expiresAt: 2_000_000_000,
        pollIntervalSeconds: 5,
      },
    },
  },
};
export const Denied: Story = {
  args: {
    view: {
      ...meta.args.view,
      error: "授权被拒绝；此 GitHub 账号尚未获得邀请。",
    },
  },
};
export const PendingConfirmation: Story = {
  args: {
    view: {
      ...meta.args.view,
      status: {
        ...ready,
        enabled: false,
        state: "off",
        controlReady: false,
        bulkReady: false,
        readyConnections: 0,
        publicUrl: "",
      },
    },
  },
};
export const Ready: Story = {
  args: { view: { ...meta.args.view, status: ready } },
};
export const BulkDegraded: Story = {
  args: {
    view: {
      ...meta.args.view,
      status: {
        ...ready,
        state: "degraded",
        bulkReady: false,
        readyConnections: 1,
        reconnectReason: "bulk-unavailable",
      },
    },
  },
};
export const TurnUnavailable: Story = {
  args: {
    view: {
      ...meta.args.view,
      status: {
        ...ready,
        sessions: [
          {
            id: "browser-a",
            path: "wss",
            candidateType: "",
            transport: "wss",
            reason: "turn-unavailable",
          },
        ],
      },
    },
  },
};
export const Revoked: Story = {
  args: {
    view: {
      ...meta.args.view,
      status: {
        ...ready,
        state: "failed",
        controlReady: false,
        bulkReady: false,
        readyConnections: 0,
        publicUrl: "",
        sessions: [],
        error: "authorization-revoked",
      },
    },
  },
};
export const Phone: Story = {
  ...Ready,
  globals: { viewport: { value: "mobile1", isRotated: false } },
};
export const Tablet: Story = {
  ...BulkDegraded,
  globals: { viewport: { value: "tablet", isRotated: false } },
};
export const WssOnly: Story = {
  args: {
    view: {
      ...meta.args.view,
      status: {
        ...ready,
        hybridSupported: false,
        hybridUnavailableReason: "p2p-feature-unavailable",
        sessions: [
          {
            id: "browser-a",
            path: "wss",
            candidateType: "",
            transport: "wss",
            reason: "p2p-feature-unavailable",
          },
        ],
      },
    },
  },
};
