import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import App from "../../src/App.svelte";
import LazyPageHarness from "./LazyPageHarness.svelte";
import { FeatureIDSchema } from "../../src/contracts/feature-schemas";
import { emptySettings } from "../../src/domains/settings/settings-intent";
import { clearHomeCache } from "../../src/domains/workspace/home-cache";
import { shellUi } from "../../src/hooks/shared/shell.svelte";
import { initializeLocale, locale } from "../../src/lib/locale";
import { resetRelayAuth } from "../../src/platform/relay-auth";
import { setActiveHostId } from "../../src/platform/storage";
import { installChatFixture, respond, socketAt } from "./chat-fixture";

installChatFixture();

let root: HTMLElement;
const originalURL = location.href;

function capabilitiesBody() {
  return {
    version: 2,
    features: FeatureIDSchema.options.map((id) => ({
      id,
      enabled: false,
      status: "disabled" as const,
      reason: "feature_disabled" as const,
    })),
    capabilities: [{ id: "terminal.pty", available: true }],
  };
}

function installCoreDefaults() {
  respond(/^GET \/api\/config$/, () =>
    Response.json({
      defaultCwd: "/work",
      hostname: "boot-host",
      portDetection: false,
      voice: false,
      workspace: {
        gitAvailable: false,
        registryAvailable: false,
        gitError: "",
        registryError: "",
      },
    }),
  );
  respond(/^GET \/api\/settings$/, () =>
    Response.json({ revision: 0, updatedAt: "", values: emptySettings }),
  );
  respond(/^GET \/api\/sessions$/, () => Response.json([]));
}

beforeEach(() => {
  localStorage.clear();
  initializeLocale();
  locale.setPreference("zh-CN");
  resetRelayAuth();
  setActiveHostId(null);
  history.replaceState(null, "", "/");
  shellUi.setCollapsed(false);
  clearHomeCache();
  root = document.createElement("div");
  root.id = "app";
  document.body.append(root);
  installCoreDefaults();
});

afterEach(async () => {
  vi.useRealTimers();
  await cleanup();
  await userEvent.cleanup();
  resetRelayAuth();
  root.remove();
  setActiveHostId(null);
  history.replaceState(null, "", originalURL);
  localStorage.clear();
  locale.setPreference("zh-CN");
  clearHomeCache();
});

test("first visit hides login card while probing and shows quiet splash only after delay", async () => {
  const health = Promise.withResolvers<Response>();
  respond(/^GET \/api\/health$/, () => health.promise);
  respond(/^GET \/api\/capabilities$/, () => Response.json(capabilitiesBody()));

  await render(App, { target: root });

  // Within the initial grace period, neither the login headline nor connecting text flashes.
  await expect
    .element(page.getByText("你的终端，始终在线。"))
    .not.toBeInTheDocument();
  await expect.element(page.getByText("正在连接…")).not.toBeInTheDocument();

  // After the grace period on a slow connection, show quiet status without login card.
  await expect.element(page.getByText("正在连接…")).toBeVisible();
  await expect
    .element(page.getByText("你的终端，始终在线。"))
    .not.toBeInTheDocument();

  health.resolve(Response.json({ ok: true, sessions: 0 }));
  const feed = await socketAt("/ws/sessions");
  feed.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  await expect
    .element(page.getByRole("searchbox", { name: "搜索会话", exact: true }))
    .toBeVisible();
  expect(localStorage.getItem("webmux.connected")).toBe("1");
});

test("returning session renders optimistic shell skeleton and probes health and capabilities in parallel", async () => {
  localStorage.setItem("webmux.connected", "1");
  const health = Promise.withResolvers<Response>();
  const capabilities = Promise.withResolvers<Response>();
  let healthCalls = 0;
  let capabilitiesCalls = 0;
  respond(/^GET \/api\/health$/, () => {
    healthCalls++;
    return health.promise;
  });
  respond(/^GET \/api\/capabilities$/, () => {
    capabilitiesCalls++;
    return capabilities.promise;
  });

  await render(App, { target: root });

  // Frame 1 immediately shows the shell skeleton without flashing the login card.
  await expect
    .element(page.getByRole("main", { name: "正在恢复工作区" }))
    .toBeVisible();
  await expect
    .element(page.getByText("你的终端，始终在线。"))
    .not.toBeInTheDocument();

  // Both /api/health and /api/capabilities are already in flight concurrently.
  await expect.poll(() => healthCalls).toBe(1);
  await expect.poll(() => capabilitiesCalls).toBe(1);

  health.resolve(Response.json({ ok: true, sessions: 0 }));
  capabilities.resolve(Response.json(capabilitiesBody()));
  const feed = await socketAt("/ws/sessions");
  feed.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  await expect
    .element(page.getByRole("searchbox", { name: "搜索会话", exact: true }))
    .toBeVisible();
});

test("returning session falls back to login card on 401 and keeps form mounted while submitting", async () => {
  localStorage.setItem("webmux.connected", "1");
  localStorage.setItem("webmux.token", "expired-token");
  let allow = false;
  let pendingRetry:
    | { promise: Promise<Response>; resolve: (value: Response) => void }
    | undefined;
  respond(/^GET \/api\/health$/, () => {
    if (pendingRetry) return pendingRetry.promise;
    return allow
      ? Response.json({ ok: true, sessions: 0 })
      : Response.json({ error: "authentication required" }, { status: 401 });
  });
  respond(/^GET \/api\/capabilities$/, () =>
    allow
      ? Response.json(capabilitiesBody())
      : Response.json({ error: "authentication required" }, { status: 401 }),
  );

  await render(App, { target: root });

  await expect.element(page.getByText("你的终端，始终在线。")).toBeVisible();
  await expect.element(page.getByRole("alert")).toBeVisible();
  expect(localStorage.getItem("webmux.connected")).toBeNull();

  // Submitting keeps the input mounted and disables controls while connecting.
  pendingRetry = Promise.withResolvers<Response>();
  const input = page.getByLabelText("共享 Token");
  await input.fill("fresh-token");
  await userEvent.click(page.getByRole("button", { name: "连接 WebMux →" }));
  await expect.element(input).toBeDisabled();
  await expect
    .element(page.getByRole("button", { name: "正在连接…" }))
    .toBeDisabled();

  allow = true;
  pendingRetry.resolve(Response.json({ ok: true, sessions: 0 }));
  pendingRetry = undefined;
  const feed = await socketAt("/ws/sessions");
  feed.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  await expect
    .element(page.getByRole("searchbox", { name: "搜索会话", exact: true }))
    .toBeVisible();
  expect(localStorage.getItem("webmux.connected")).toBe("1");
});

test("non-401 failure shows offline retry gate instead of demanding a token", async () => {
  localStorage.setItem("webmux.connected", "1");
  let online = false;
  respond(/^GET \/api\/health$/, () =>
    online
      ? Response.json({ ok: true, sessions: 0 })
      : Response.json({ error: "service unavailable" }, { status: 503 }),
  );
  respond(/^GET \/api\/capabilities$/, () =>
    online
      ? Response.json(capabilitiesBody())
      : Response.json({ error: "service unavailable" }, { status: 503 }),
  );

  await render(App, { target: root });

  await expect.element(page.getByText("无法连接到 WebMux")).toBeVisible();
  await expect
    .element(page.getByRole("alert"))
    .toHaveTextContent("service unavailable");
  await expect
    .element(page.getByLabelText("共享 Token"))
    .not.toBeInTheDocument();

  // Users can still switch to token login explicitly if needed.
  await userEvent.click(
    page.getByRole("button", { name: "使用共享 Token 登录" }),
  );
  await expect.element(page.getByLabelText("共享 Token")).toBeVisible();

  // Returning to offline state and retrying recovers seamlessly.
  await userEvent.click(page.getByRole("button", { name: "连接 WebMux →" }));
  await expect.element(page.getByText("无法连接到 WebMux")).toBeVisible();

  online = true;
  await userEvent.click(page.getByRole("button", { name: "重试连接" }));
  const feed = await socketAt("/ws/sessions");
  feed.frame({ type: "snapshot", sessions: [], forwards: [] } as never);
  await expect
    .element(page.getByRole("searchbox", { name: "搜索会话", exact: true }))
    .toBeVisible();
});

test("LazyPage omits brief loading text for fast chunks and surfaces slow or failed loads", async () => {
  const fast = Promise.withResolvers<string>();
  await render(LazyPageHarness, {
    load: () => fast.promise,
    label: "设置",
    loadingDelayMs: 150,
  });
  await expect.element(page.getByText("正在加载设置…")).not.toBeInTheDocument();
  fast.resolve("设置正文");
  await expect.element(page.getByText("设置正文")).toBeVisible();

  await cleanup();

  const slow = Promise.withResolvers<string>();
  await render(LazyPageHarness, {
    load: () => slow.promise,
    label: "设置",
    loadingDelayMs: 20,
  });
  await expect.element(page.getByText("正在加载设置…")).toBeVisible();
  slow.reject(new Error("chunk failed"));
  await expect
    .element(page.getByRole("alert"))
    .toHaveTextContent("设置加载失败，请刷新页面后重试。");
});
