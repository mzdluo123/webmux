import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import {
  FeatureIDSchema,
  type Capabilities,
} from "../../src/contracts/feature-schemas";
import {
  setFeatureCapabilities,
  resetFeatureCapabilities,
} from "../../src/platform/feature-host.svelte";
import {
  applyRelayAuth,
  clearRelayAuth,
  relayAuthRevision,
} from "../../src/platform/relay-auth";
import { start } from "../../src/features/entries/p2p";

const lifecycle = vi.hoisted(() => ({
  start: vi.fn(),
  stop: vi.fn(),
  ready: vi.fn(),
}));
vi.mock("../../src/platform/p2p.svelte", () => ({
  startP2P: lifecycle.start,
  stopP2P: lifecycle.stop,
}));
vi.mock("../../src/platform/p2p-state.svelte", () => ({
  p2pState: {},
  markFirstScreenReady: lifecycle.ready,
}));
beforeEach(() => {
  clearRelayAuth();
  lifecycle.start.mockReset();
});
afterEach(() => {
  clearRelayAuth();
  resetFeatureCapabilities();
});

test.each([
  {
    selected: false,
    direct: false,
    hybrid: false,
    owner: true,
    forward: false,
    runs: false,
  },
  {
    selected: false,
    direct: false,
    hybrid: true,
    owner: true,
    forward: false,
    runs: false,
  },
  {
    selected: true,
    direct: false,
    hybrid: false,
    owner: true,
    forward: false,
    runs: false,
  },
  {
    selected: true,
    direct: false,
    hybrid: true,
    owner: false,
    forward: false,
    runs: false,
  },
  {
    selected: true,
    direct: false,
    hybrid: true,
    owner: true,
    forward: false,
    runs: true,
  },
  {
    selected: true,
    direct: true,
    hybrid: false,
    owner: false,
    forward: false,
    runs: true,
  },
  {
    selected: true,
    direct: true,
    hybrid: true,
    owner: true,
    forward: true,
    runs: false,
  },
])(
  "starts only selected, compiled and authorized infrastructure: %j",
  (scenario) => {
    const capabilities: Capabilities = {
      version: 2,
      features: FeatureIDSchema.options.map((id) =>
        id === "p2p" && scenario.selected
          ? { id, enabled: true, status: "ready" as const }
          : {
              id,
              enabled: false,
              status: "disabled" as const,
              reason: "feature_disabled" as const,
            },
      ),
      capabilities: [
        scenario.direct
          ? { id: "p2p.direct", available: true }
          : { id: "p2p.direct", available: false, reason: "configuration_off" },
        scenario.hybrid
          ? { id: "relay.hybrid", available: true }
          : { id: "relay.hybrid", available: false, reason: "unsupported" },
      ],
    };
    setFeatureCapabilities(capabilities);
    if (scenario.owner)
      applyRelayAuth(
        {
          authenticated: true,
          csrfToken: "csrf",
          expiresAt: Math.floor(Date.now() / 1000) + 60,
        },
        relayAuthRevision(),
      );
    const dispose = start({ forwardAuth: scenario.forward });
    expect(lifecycle.start).toHaveBeenCalledTimes(scenario.runs ? 1 : 0);
    dispose();
  },
);
