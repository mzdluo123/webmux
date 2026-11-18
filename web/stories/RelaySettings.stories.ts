import type { Meta, StoryObj } from "@storybook/svelte-vite";
import RelaySettings from "../src/components/settings/RelaySettings.svelte";
import type { RelayStatus, RelayPairing } from "../src/contracts/relay-schemas";
import type { RelaySettingsView } from "../src/hooks/settings/relay-settings.svelte";
import { relayStatus as status, relayPairing } from "../test/fixtures/relay";

const paired: RelayStatus = {
  ...status,
  paired: true,
  ownerId: "1234567",
  ownerLogin: "relay-owner",
  boundPublicUrl:
    "https://h-0123456789abcdef0123456789abcdef.relay.example.test",
};
const ready: RelayStatus = {
  ...paired,
  enabled: true,
  state: "ready",
  controlReady: true,
  bulkReady: true,
  readyConnections: 2,
  hybridSupported: true,
  hybridUnavailableReason: "",
  publicUrl: paired.boundPublicUrl,
  controlReachability: "reachable",
  dataReachability: "reachable",
  connectorUrl: "wss://data.example.test/v1/connector",
  dataOrigin: "https://data.example.test",
  assignmentRevision: "2",
  configRevision: "3",
  appliedRevision: "3",
  nodeId: "wss-west",
  capabilities: ["wss", "turn"],
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
const waiting: RelayPairing = {
  ...relayPairing,
  phase: "waiting",
  userCode: "ABCD-EFGH",
  verificationUrl: "https://github.com/login/device",
  expiresAt: Math.floor(Date.now() / 1000) + 600,
  pollIntervalSeconds: 5,
};
const baseView: RelaySettingsView = {
  status,
  pairing: relayPairing,
  loading: false,
  busy: false,
  phase: "idle",
  error: "",
  notice: "",
};
const meta = {
  title: "Settings/Official Relay",
  component: RelaySettings,
  args: {
    view: baseView,
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

function flow(phase: RelayPairing["phase"], extra: Partial<RelayPairing> = {}) {
  return {
    args: { view: { ...baseView, pairing: { ...waiting, phase, ...extra } } },
  };
}
function operation(
  phase: RelayStatus["operation"]["phase"],
  extra: Partial<RelayStatus["operation"]> = {},
) {
  return {
    args: {
      view: {
        ...baseView,
        status: {
          ...paired,
          operation: {
            ...status.operation,
            kind: "enable" as const,
            phase,
            ...extra,
          },
        },
        phase:
          phase === "submitting"
            ? ("verifying" as const)
            : phase === "uncertain"
              ? ("failed" as const)
              : phase,
        busy: phase === "submitting",
        notice:
          phase === "completed"
            ? "操作已完成；运行状态以当前就绪信息为准。"
            : "",
      },
    },
  };
}

export const Loading: Story = {
  args: {
    view: {
      ...baseView,
      status: undefined,
      pairing: undefined,
      loading: true,
      phase: "loading",
    },
  },
};
export const Unpaired: Story = {};
export const Preparing: Story = flow("preparing", {
  userCode: "",
  verificationUrl: "",
  expiresAt: 0,
});
export const Pairing: Story = flow("waiting");
export const Retrying: Story = flow("retrying", {
  retryAt: Math.floor(Date.now() / 1000) + 15,
  error: "rate-limited",
});
export const CreationUncertain: Story = flow("uncertain", {
  userCode: "",
  verificationUrl: "",
  expiresAt: 0,
});
export const Cancelling: Story = flow("cancelling");
export const Cancelled: Story = {
  args: {
    view: {
      ...baseView,
      pairing: { ...relayPairing, phase: "cancelled" },
      notice: "配对已取消。",
    },
  },
};
export const Denied: Story = flow("denied", {
  userCode: "",
  verificationUrl: "",
  expiresAt: 0,
  error: "denied",
});
export const Expired: Story = flow("expired", {
  userCode: "",
  verificationUrl: "",
  expiresAt: 0,
});
export const AwaitingHostExpiry: Story = flow("waiting", { expiresAt: 1 });
export const Interrupted: Story = flow("interrupted", {
  userCode: "",
  verificationUrl: "",
  expiresAt: 0,
});
export const ExistingBindingConflict: Story = flow("failed", {
  userCode: "",
  verificationUrl: "",
  expiresAt: 0,
  error: "conflict",
});
export const TemporaryReadFailure: Story = {
  args: {
    view: {
      ...baseView,
      pairing: waiting,
      phase: "failed",
      error: "暂时无法连接宿主；将自动核对，也可检查当前状态。",
    },
  },
};
export const PairedSuccess: Story = {
  args: {
    view: {
      ...baseView,
      status: paired,
      pairing: { ...relayPairing, phase: "paired" },
      notice: "GitHub 主机配对已完成；公网访问仍需单独启用。",
    },
  },
};
export const PendingConfirmation: Story = {
  args: { view: { ...baseView, status: paired } },
};
export const Ready: Story = { args: { view: { ...baseView, status: ready } } };
export const Submitting: Story = {
  args: {
    view: { ...baseView, status: paired, phase: "submitting", busy: true },
  },
};
export const Verifying: Story = operation("submitting");
export const OperationUncertain: Story = operation("uncertain");
export const OperationFailed: Story = operation("failed", {
  error: "storage-unavailable",
});
export const OperationCompleted: Story = operation("completed");
export const DeletedBinding: Story = {
  args: {
    view: {
      ...baseView,
      phase: "completed",
      status: {
        ...status,
        operation: {
          kind: "delete",
          phase: "completed",
          error: "",
          deleted: true,
          accessClosed: true,
        },
      },
      notice: "绑定已删除，旧访问已确认关闭。重新配对将取得新地址。",
    },
  },
};
export const DeletedAccessUnconfirmed: Story = {
  args: {
    view: {
      ...baseView,
      phase: "completed",
      status: {
        ...status,
        operation: {
          kind: "delete",
          phase: "completed",
          error: "",
          deleted: true,
          accessClosed: false,
        },
      },
      notice:
        "绑定已删除，但旧直连访问尚未确认关闭。可重新配对并取得新地址；这不代表全局退出已完成。",
    },
  },
};
export const DeleteUncertainRetained: Story = operation("uncertain", {
  kind: "delete",
  deleted: false,
  accessClosed: false,
  error: "unavailable",
});
export const BulkDegraded: Story = {
  args: {
    view: {
      ...baseView,
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
      ...baseView,
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
      ...baseView,
      status: { ...paired, state: "failed", error: "authorization-revoked" },
    },
  },
};
export const WssOnly: Story = {
  args: {
    view: {
      ...baseView,
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
export const Phone: Story = {
  ...Pairing,
  globals: { viewport: { value: "mobile1", isRotated: false } },
};
export const Tablet: Story = {
  ...Ready,
  globals: { viewport: { value: "tablet", isRotated: false } },
};
