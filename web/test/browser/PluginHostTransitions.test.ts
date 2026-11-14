import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import App from "../../src/App.svelte";
import { navigate } from "../../src/platform/api";
import { setActiveHostId } from "../../src/platform/storage";
import { requestPluginHostRefresh } from "../../src/platform/plugin-host.svelte";
import { p2pState } from "../../src/platform/p2p.svelte";
import { voicePreference } from "../../src/hooks/shared/voice-preference.svelte";
import { emptySettings } from "../../src/domains/settings/settings-intent";
import { clearHomeCache } from "../../src/domains/workspace/home-cache";
import { shellUi } from "../../src/hooks/shared/shell.svelte";
import {
  PluginIDSchema,
  type PluginID,
} from "../../src/contracts/plugin-schemas";
import type { P2PStatus } from "../../src/contracts/schemas";
import type { NegotiatedSession } from "../../src/platform/p2p-session";
import type * as P2PFraming from "../../src/platform/p2p-framing";
import {
  FakeSocket,
  installChatFixture,
  respond,
  socketAt,
  sockets,
} from "./chat-fixture";
import { launchConfig, repository, session } from "./shell-fixture";

const direct = vi.hoisted(() => ({ negotiate: vi.fn(), baseline: vi.fn() }));
vi.mock("../../src/platform/p2p-session", () => ({
  negotiateDirectPeer: direct.negotiate,
  measureBaselineHttpRtt: direct.baseline,
}));
vi.mock("../../src/platform/p2p-framing", async (original) => ({
  ...(await original<typeof P2PFraming>()),
  isLocalOrOverlayHost: () => false,
}));

installChatFixture();
let root: HTMLElement;
let enabled: PluginID[];
let voiceReady: boolean;
let p2pReady: boolean;
let discoveryReads: number;
let network: EventTarget;
const primary: Partial<Record<PluginID, string>> = {
  workspace: "workspace.manage",
  launch: "launch.configs",
  files: "files.upload",
  chat: "chat.acp",
  voice: "voice.recognize",
  code: "code.browse",
  p2p: "p2p.direct",
};
const peerStatus: P2PStatus = {
  supported: true,
  enabled: false,
  udpPort: 0,
  udpBindings: [],
  capabilities: { networks: ["udp4"], iceMode: "full", turn: false },
  portMapping: false,
  stunServers: [],
  source: "config",
  storageAvailable: true,
  portMap: { state: "off" },
  activePeers: 0,
  peers: [],
};

beforeEach(() => {
  enabled = [];
  voiceReady = false;
  p2pReady = false;
  discoveryReads = 0;
  history.replaceState(null, "", "/");
  shellUi.setMode("workbench");
  clearHomeCache();
  root = document.createElement("div");
  root.id = "app";
  document.body.append(root);
  network = new EventTarget();
  const originalNavigator = navigator;
  vi.stubGlobal(
    "navigator",
    new Proxy(originalNavigator, {
      get(target, property) {
        return property === "connection"
          ? network
          : Reflect.get(target, property, target);
      },
    }),
  );
  direct.baseline.mockReset().mockResolvedValue([100, 100, 100]);
  direct.negotiate
    .mockReset()
    .mockImplementation(async (): Promise<NegotiatedSession> => ({
      peerId: crypto.randomUUID(),
      pc: {
        connectionState: "connected",
        onconnectionstatechange: null,
      } as unknown as RTCPeerConnection,
      control: {
        readyState: "open",
        onclose: null,
        send: vi.fn(),
      } as unknown as RTCDataChannel,
      candidateType: "host/srflx",
      ipFamily: "ipv4",
      sendPing: vi.fn().mockResolvedValue(95),
      close: vi.fn(),
    }));
  respond(/^GET \/api\/health$/, () =>
    Response.json({ ok: true, sessions: 0 }),
  );
  respond(/^GET \/api\/capabilities$/, () => {
    discoveryReads++;
    const availability: Partial<Record<PluginID, boolean>> = {
      voice: voiceReady,
      p2p: p2pReady,
    };
    return Response.json({
      version: 1,
      plugins: PluginIDSchema.options.map((id) => ({
        id,
        enabled: enabled.includes(id),
        status: !enabled.includes(id)
          ? "disabled"
          : availability[id] === false
            ? "unavailable"
            : "ready",
        reason: !enabled.includes(id)
          ? "plugin_disabled"
          : availability[id] === false
            ? "configuration_off"
            : undefined,
      })),
      capabilities: [
        { id: "terminal.pty", available: true },
        ...enabled.map((id) => ({
          id: primary[id] ?? `${id}.unused`,
          available: availability[id] ?? true,
          reason: availability[id] === false ? "configuration_off" : undefined,
        })),
      ],
    });
  });
  respond(/^GET \/api\/config$/, () =>
    Response.json({
      defaultCwd: "/work",
      hostname: "transition-host",
      portDetection: false,
      voice: voiceReady,
      workspace: {
        gitAvailable: true,
        registryAvailable: true,
        gitError: "",
        registryError: "",
      },
    }),
  );
  respond(/^GET \/api\/settings$/, () =>
    Response.json({ revision: 0, updatedAt: "", values: emptySettings }),
  );
  respond(/^GET \/api\/repositories$/, () => Response.json([repository()]));
  respond(/^GET \/api\/launch-configs$/, () => Response.json([launchConfig()]));
  respond(/^GET \/api\/sessions$/, () => Response.json([]));
  respond(/^GET \/api\/p2p$/, () =>
    Response.json({ ...peerStatus, enabled: p2pReady }),
  );
});
afterEach(async () => {
  vi.useRealTimers();
  await cleanup();
  root.remove();
  setActiveHostId(null);
  history.replaceState(null, "", "/");
  clearHomeCache();
});

async function mount() {
  await render(App, { target: root });
  const feed = await socketAt("/ws/sessions");
  feed.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  return feed;
}

async function reconnect(feed: FakeSocket, index: number) {
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  try {
    feed.drop(1006);
    vi.runOnlyPendingTimers();
  } finally {
    vi.useRealTimers();
  }
  const next = await socketAt("/ws/sessions", index);
  next.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  return next;
}

test("same-origin reconnect replaces captured shell projections only when selection changes", async () => {
  enabled = ["workspace"];
  const first = await mount();
  await expect
    .element(page.getByRole("button", { name: "添加仓库", exact: true }))
    .toBeVisible();
  enabled = [];
  const reconnecting = await reconnect(first, 1);
  await expect
    .element(page.getByRole("textbox", { name: "搜索会话", exact: true }))
    .toBeVisible();
  expect(reconnecting.readyState).toBe(FakeSocket.CLOSED);
  expect(
    page.getByRole("button", { name: "添加仓库", exact: true }).elements(),
  ).toEqual([]);
  const core = await socketAt("/ws/sessions", 2);
  core.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  await page
    .getByRole("textbox", { name: "搜索会话", exact: true })
    .fill("keep filter");
  const before = discoveryReads;
  await reconnect(core, 3);
  await expect.poll(() => discoveryReads).toBeGreaterThan(before);
  await expect
    .element(page.getByRole("textbox", { name: "搜索会话", exact: true }))
    .toHaveValue("keep filter");
  await requestPluginHostRefresh();
  enabled = ["workspace"];
  await reconnect(
    sockets.filter((socket) => socket.path === "/ws/sessions")[3]!,
    4,
  );
  await expect
    .element(page.getByRole("button", { name: "添加仓库", exact: true }))
    .toBeVisible();
  await socketAt("/ws/sessions", 5);
});

test("saving a recognizer refreshes availability before returning to terminal and quick start", async () => {
  enabled = ["workspace", "launch", "files", "chat", "voice"];
  voicePreference.enabled = true;
  respond(/^GET \/api\/voice\/settings$/, () =>
    Response.json({
      cmd: [],
      env: {},
      enabled: false,
      source: "config",
      storageAvailable: true,
    }),
  );
  respond(/^PUT \/api\/voice\/settings$/, () => {
    voiceReady = true;
    return Response.json({
      cmd: ["recognizer"],
      env: {},
      enabled: true,
      source: "web",
      storageAvailable: true,
    });
  });
  await mount();
  navigate("/settings");
  await page
    .getByRole("textbox", { name: "命令与参数", exact: true })
    .fill("recognizer");
  await page
    .getByRole("form", { name: "语音识别配置" })
    .getByRole("button", { name: "保存", exact: true })
    .click();
  await expect
    .element(page.getByText("已保存，下一次录音起生效。", { exact: true }))
    .toBeVisible();
  const info = session({ id: "aaaaaaaa" });
  respond(/^GET \/api\/sessions\/aaaaaaaa$/, () => Response.json(info));
  navigate("/s/aaaaaaaa");
  const terminal = await socketAt("/ws/s/aaaaaaaa");
  terminal.binary(2, info);
  await page.getByRole("button", { name: "草稿模式", exact: true }).click();
  await expect
    .element(page.getByRole("button", { name: "语音输入", exact: true }))
    .toBeVisible();
  navigate("/new");
  await expect
    .element(page.getByRole("button", { name: "语音输入", exact: true }))
    .toBeVisible();
});

test("P2P business off at bootstrap can start from Settings, retain network lifecycle, and stop on auth failure", async () => {
  enabled = ["p2p"];
  await mount();
  expect(direct.negotiate).not.toHaveBeenCalled();
  respond(/^PUT \/api\/p2p\/settings$/, () => {
    p2pReady = true;
    return Response.json({ ...peerStatus, enabled: true, source: "web" });
  });
  navigate("/settings");
  await page.getByRole("switch", { name: "自动尝试 P2P 直连升级" }).click();
  await expect.poll(() => p2pState.phase).toBe("standby_hysteresis");
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  network.dispatchEvent(new Event("change"));
  vi.advanceTimersByTime(1_000);
  vi.useRealTimers();
  await expect.poll(() => direct.negotiate.mock.calls.length).toBe(2);
  await expect.poll(() => p2pState.phase).toBe("standby_hysteresis");
  const pending = Promise.withResolvers<NegotiatedSession>();
  direct.negotiate.mockReturnValueOnce(pending.promise);
  document.dispatchEvent(new Event("visibilitychange"));
  await expect.poll(() => direct.negotiate.mock.calls.length).toBe(3);
  const signal = direct.negotiate.mock.calls[2]![1] as AbortSignal;
  window.dispatchEvent(new Event("webmux:auth"));
  expect(signal.aborted).toBe(true);
  pending.reject(new DOMException("aborted", "AbortError"));
  await expect.poll(() => p2pState.phase).toBe("idle");
  const count = direct.negotiate.mock.calls.length;
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  network.dispatchEvent(new Event("change"));
  window.dispatchEvent(new Event("online"));
  vi.advanceTimersByTime(60_000);
  vi.useRealTimers();
  expect(direct.negotiate).toHaveBeenCalledTimes(count);
  expect(
    sockets.every((socket) => socket.readyState === FakeSocket.CLOSED),
  ).toBe(true);
});

test("host switching releases old P2P resources and disabled selection never restarts them", async () => {
  enabled = ["p2p"];
  p2pReady = true;
  const pending = Promise.withResolvers<NegotiatedSession>();
  direct.negotiate.mockReturnValueOnce(pending.promise);
  await mount();
  await expect.poll(() => direct.negotiate.mock.calls.length).toBe(1);
  const signal = direct.negotiate.mock.calls[0]![1] as AbortSignal;
  enabled = [];
  setActiveHostId("transition-core");
  expect(signal.aborted).toBe(true);
  pending.reject(new DOMException("aborted", "AbortError"));
  await expect
    .element(page.getByRole("textbox", { name: "搜索会话", exact: true }))
    .toBeVisible();
  await requestPluginHostRefresh();
  const count = direct.negotiate.mock.calls.length;
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  network.dispatchEvent(new Event("change"));
  document.dispatchEvent(new Event("visibilitychange"));
  await vi.advanceTimersByTimeAsync(60_000);
  vi.useRealTimers();
  expect(direct.negotiate).toHaveBeenCalledTimes(count);
  expect(p2pState.mode).toBe("ws");
});
