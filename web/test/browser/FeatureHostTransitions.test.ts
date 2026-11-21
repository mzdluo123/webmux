import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import App from "../../src/App.svelte";
import { api, navigate } from "../../src/platform/api";
import {
  applyRelayAuth,
  relayAuthRevision,
  relayAuthStatus,
  resetRelayAuth,
} from "../../src/platform/relay-auth";
import { NoContentSchema } from "../../src/contracts/schemas";
import { setActiveHostId } from "../../src/platform/storage";
import { requestFeatureHostRefresh } from "../../src/platform/feature-host.svelte";
import { p2pState } from "../../src/platform/p2p-state.svelte";
import { voicePreference } from "../../src/hooks/shared/voice-preference.svelte";
import { emptySettings } from "../../src/domains/settings/settings-intent";
import { clearHomeCache } from "../../src/domains/workspace/home-cache";
import { shellUi } from "../../src/hooks/shared/shell.svelte";
import {
  FeatureIDSchema,
  type FeatureID,
} from "../../src/contracts/feature-schemas";
import type { P2PStatus } from "../../src/contracts/schemas";
import type { NegotiatedSession } from "../../src/platform/p2p-session";
import type * as P2PFraming from "../../src/platform/p2p-framing";
import {
  FakeSocket,
  commands,
  installChatFixture,
  respond,
  socketAt,
  sockets,
} from "./chat-fixture";
import { launchConfig, repository, session } from "./shell-fixture";

const direct = vi.hoisted(() => ({ negotiate: vi.fn(), baseline: vi.fn() }));
vi.mock("../../src/platform/p2p-session", () => ({
  negotiateDirectPeer: direct.negotiate,
  inspectSelectedCandidatePair: vi.fn(async () => ({
    path: "webrtc-direct",
    isRelay: false,
    candidateType: "host/srflx",
    ipFamily: "ipv4",
  })),
}));
vi.mock("../../src/platform/p2p-signaling", () => ({
  measureBaselineHttpRtt: direct.baseline,
}));
vi.mock("../../src/platform/p2p-framing", async (original) => ({
  ...(await original<typeof P2PFraming>()),
  isLocalOrOverlayHost: () => false,
}));

installChatFixture();
let root: HTMLElement;
let enabled: FeatureID[];
let unavailable: FeatureID[];
let voiceReady: boolean;
let p2pReady: boolean;
let discoveryReads: number;
let network: EventTarget;
const primary: Partial<Record<FeatureID, string>> = {
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
  resetRelayAuth();
  enabled = [];
  unavailable = [];
  voiceReady = false;
  p2pReady = false;
  discoveryReads = 0;
  history.replaceState(null, "", "/");
  shellUi.setCollapsed(false);
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
      path: "webrtc-direct",
      expiresAt: null,
      onFailure: null,
      sendPing: vi.fn().mockResolvedValue(95),
      close: vi.fn(),
    }));
  respond(/^GET \/api\/health$/, () =>
    Response.json({ ok: true, sessions: 0 }),
  );
  respond(/^GET \/api\/capabilities$/, () => {
    discoveryReads++;
    const availability: Partial<Record<FeatureID, boolean>> = {
      ...Object.fromEntries(unavailable.map((id) => [id, false])),
      voice: voiceReady,
      p2p: p2pReady,
    };
    return Response.json({
      version: 2,
      features: FeatureIDSchema.options.map((id) => ({
        id,
        enabled: enabled.includes(id),
        status: !enabled.includes(id)
          ? "disabled"
          : availability[id] === false
            ? "unavailable"
            : "ready",
        reason: !enabled.includes(id)
          ? "feature_disabled"
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
  resetRelayAuth();
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

async function openTerminalMenu() {
  await page.getByRole("button", { name: "终端菜单", exact: true }).click();
  const menu = page.getByRole("menu");
  await expect.element(menu).toBeVisible();
  return menu;
}

const draftMode = () =>
  page.getByRole("menuitemcheckbox", { name: "草稿模式", exact: true });

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
  expect(
    page.getByRole("button", { name: "会话", exact: true }).elements(),
  ).toEqual([]);
  expect(
    page.getByRole("button", { name: "Git 工作台", exact: true }).elements(),
  ).toEqual([]);
  enabled = [];
  const reconnecting = await reconnect(first, 1);
  await expect
    .element(page.getByRole("searchbox", { name: "搜索会话", exact: true }))
    .toBeVisible();
  await expect.poll(() => reconnecting.readyState).toBe(FakeSocket.CLOSED);
  await expect
    .element(page.getByRole("button", { name: "会话", exact: true }))
    .toBeVisible();
  const core = await socketAt("/ws/sessions", 2);
  core.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  await page
    .getByRole("searchbox", { name: "搜索会话", exact: true })
    .fill("keep filter");
  const before = discoveryReads;
  await reconnect(core, 3);
  await expect.poll(() => discoveryReads).toBeGreaterThan(before);
  await expect
    .element(page.getByRole("searchbox", { name: "搜索会话", exact: true }))
    .toHaveValue("keep filter");
  await requestFeatureHostRefresh();
  enabled = ["workspace"];
  await reconnect(
    sockets.filter((socket) => socket.path === "/ws/sessions")[3]!,
    4,
  );
  await expect
    .poll(
      () =>
        page.getByRole("button", { name: "会话", exact: true }).elements()
          .length,
    )
    .toBe(0);
  await socketAt("/ws/sessions", 5);
});

test("existing terminal recovers optional controls without replacing its owner or draft", async () => {
  await page.viewport(1280, 900);
  enabled = ["files", "code", "voice"];
  unavailable = ["files", "code"];
  voicePreference.enabled = true;
  const info = session({ id: "recover2" });
  respond(/^GET \/api\/sessions\/recover2$/, () => Response.json(info));
  await mount();
  navigate(`/s/${info.id}`);
  const terminal = await socketAt(`/ws/s/${info.id}`);
  terminal.binary(2, info);
  const menu = await openTerminalMenu();
  const upload = menu.getByRole("menuitem", { name: "上传文件", exact: true });
  await expect.element(upload).not.toBeInTheDocument();
  await draftMode().click();
  const draft = page.getByRole("textbox", { name: "草稿内容", exact: true });
  await draft.fill("retain this draft while services recover");
  const owner = document.querySelector(`[data-dock-panel="${info.id}"]`);
  const emulator = document.querySelector(".xterm");
  expect(owner).not.toBeNull();
  expect(emulator).not.toBeNull();
  const voice = page.getByRole("button", { name: "语音输入", exact: true });
  await expect.element(voice).not.toBeInTheDocument();
  await expect
    .element(page.getByRole("button", { name: /^文件树 · / }))
    .not.toBeInTheDocument();

  // Another browser enables the services; this page only refreshes discovery.
  unavailable = [];
  voiceReady = true;
  await requestFeatureHostRefresh();
  await expect
    .element(page.getByRole("button", { name: /^文件树 · / }))
    .toBeVisible();
  await expect.element(voice).toBeVisible();
  await openTerminalMenu();
  await expect.element(upload).toBeVisible();
  await userEvent.keyboard("{Escape}");
  expect(document.querySelector(`[data-dock-panel="${info.id}"]`)).toBe(owner);
  expect(document.querySelector(".xterm")).toBe(emulator);
  expect(
    sockets.filter((socket) => socket.path === `/ws/s/${info.id}`),
  ).toEqual([terminal]);
  expect(terminal.readyState).toBe(FakeSocket.OPEN);
  await expect
    .element(draft)
    .toHaveValue("retain this draft while services recover");

  voiceReady = false;
  await requestFeatureHostRefresh();
  await expect.element(voice).not.toBeInTheDocument();
  expect(document.querySelector(`[data-dock-panel="${info.id}"]`)).toBe(owner);
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
  await openTerminalMenu();
  await draftMode().click();
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
    .element(page.getByRole("searchbox", { name: "搜索会话", exact: true }))
    .toBeVisible();
  await requestFeatureHostRefresh();
  const count = direct.negotiate.mock.calls.length;
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  network.dispatchEvent(new Event("change"));
  document.dispatchEvent(new Event("visibilitychange"));
  await vi.advanceTimersByTimeAsync(60_000);
  vi.useRealTimers();
  expect(direct.negotiate).toHaveBeenCalledTimes(count);
  expect(p2pState.mode).toBe("ws");
});

test("relay recovery keeps the terminal, route and unsent draft without replaying actions", async () => {
  await page.viewport(1280, 900);
  const authenticated = {
    authenticated: true,
    relay: true,
    csrfToken: "owner",
    expiresAt: Math.floor(Date.now() / 1000) + 3600,
  };
  const info = session({ id: "aaaaaaaa" });
  respond(/^GET \/api\/sessions\/aaaaaaaa$/, () => Response.json(info));
  await mount();
  applyRelayAuth(authenticated, relayAuthRevision());
  navigate("/s/aaaaaaaa?message=retained");
  const terminal = await socketAt("/ws/s/aaaaaaaa");
  terminal.binary(2, info);
  await openTerminalMenu();
  await draftMode().click();
  const draft = page.getByRole("textbox", { name: "草稿内容", exact: true });
  await draft.fill("keep my unsent relay draft");
  const ownerSelector = `[data-dock-panel="${info.id}"]`;
  const owner = document.querySelector(ownerSelector);
  expect(owner).not.toBeNull();
  const emulator = document.querySelector(".xterm");
  const commandCount = commands().length;
  const pending = Promise.withResolvers<Response>();
  respond(/^GET \/api\/relay\/auth\/session$/, () => pending.promise);
  const feed = await socketAt("/ws/sessions");
  feed.drop(1006);
  terminal.drop(1006);
  const indicator = page.getByRole("button", {
    name: "重连状态：连接已断开，重连中…",
  });
  // The expanded sidebar's footer is the lower-left reconnect surface:
  // it announces the drop and its details popover carries retry and the
  // official login, so the floating pill stays hidden beside it.
  expect(indicator.elements()).toEqual([]);
  const hostStatus = page.getByRole("button", {
    name: "查看主机状态：连接已断开，重连中…",
  });
  await expect.element(hostStatus).toBeVisible();
  await hostStatus.click();
  await expect
    .element(page.getByRole("button", { name: "再次检查连接" }))
    .toBeVisible();
  await expect
    .element(page.getByRole("link", { name: "通过 WebMux 控制台登录" }))
    .toHaveAttribute("href", "/auth/relay/start?returnTo=%2Fs%2Faaaaaaaa");
  await userEvent.keyboard("{Escape}");
  // Collapsed to the rail there is no visible footer label, so the pill
  // returns, always starting past the sidebar that is actually rendered.
  shellUi.setCollapsed(true);
  await expect.element(indicator).toBeVisible();
  const indicatorBox = (await indicator.element()).getBoundingClientRect();
  const sidebarBox = [...document.querySelectorAll(".app-sidebar")]
    .map((element) => element.getBoundingClientRect())
    .find((rect) => rect.width > 0)!;
  expect(indicatorBox.left).toBeGreaterThanOrEqual(sidebarBox.right);
  expect(
    page.getByRole("heading", { name: "通过官方中继连接" }).elements(),
  ).toEqual([]);
  expect(document.querySelector('input[type="password"]')).toBeNull();
  expect(relayAuthStatus()).toMatchObject({
    relay: true,
    authenticated: false,
    reason: "reconnecting",
  });
  expect(document.querySelector(ownerSelector)).toBe(owner);
  expect(document.querySelector(".xterm")).toBe(emulator);
  pending.resolve(Response.json(authenticated));
  const resumed = await socketAt("/ws/s/aaaaaaaa", 1);
  resumed.binary(2, info);
  await expect.element(draft).toHaveValue("keep my unsent relay draft");
  expect(location.pathname + location.search).toBe(
    "/s/aaaaaaaa?message=retained",
  );
  expect(document.querySelector(ownerSelector)).toBe(owner);
  expect(document.querySelector(".xterm")).toBe(emulator);
  expect(commands()).toHaveLength(commandCount);
});

test.each(["expired", "revoked", "unauthenticated"] as const)(
  "a relay %s response retains console login instead of reporting a bad Token",
  async (reason) => {
    const authenticated = {
      authenticated: true,
      relay: true,
      csrfToken: "owner",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    };
    await mount();
    applyRelayAuth(authenticated, relayAuthRevision());
    navigate("/settings/relay");
    respond(/^POST \/api\/relay-auth-regression$/, () =>
      Response.json(
        { error: "access ended", relay: true, reason },
        { status: 401 },
      ),
    );
    await expect(
      api("/relay-auth-regression", NoContentSchema, "POST"),
    ).rejects.toMatchObject({ status: 401 });
    const login = page.getByRole("link", { name: "通过 WebMux 控制台登录" });
    await expect.element(login).toBeVisible();
    await expect
      .element(login)
      .toHaveAttribute(
        "href",
        "/auth/relay/start?returnTo=%2Fsettings%2Frelay",
      );
    expect(document.querySelector('input[type="password"]')).toBeNull();
    expect(relayAuthStatus()).toMatchObject({ relay: true, reason });
  },
);
