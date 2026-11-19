import {
  reloadSettings,
  stopSettings,
} from "../../src/domains/settings/settings.svelte";
import { emptySettings } from "../../src/domains/settings/settings-intent";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import LaunchConfigsSettings from "../../src/components/settings/LaunchConfigsSettings.svelte";
import Settings from "../../src/pages/Settings.svelte";
import { FeatureIDSchema } from "../../src/contracts/feature-schemas";
import type {
  AgentToolsReport,
  LaunchConfigSummary,
} from "../../src/contracts/schemas";
import {
  enableCreationFeatures,
  resetCreationFeatures,
} from "./feature-fixture";

beforeEach(() => enableCreationFeatures(FeatureIDSchema.options));

const timestamp = "2026-04-15T08:30:00Z";

const baseConfigs: LaunchConfigSummary[] = [
  {
    id: "shellcfg",
    name: "Shell",
    icon: "terminal",
    cmd: ["/bin/bash", "-l", "-i"],
    envNames: [],
    chatOptions: [],
    chat: false,
    chatSupport: {
      supported: false,
      supportedAgents: [],
      reason: "unknown_agent",
    },
    createdAt: timestamp,
    updatedAt: timestamp,
  },
  {
    id: "alphacfg",
    name: "Alpha CLI",
    icon: "claudecode",
    cmd: [
      "/usr/bin/env",
      "bash",
      "-ic",
      'exec -- "$@"',
      "webmux",
      "alpha",
      "--dangerously-skip-permissions",
    ],
    envNames: ["API_KEY"],
    chatOptions: [],
    chat: true,
    chatSupport: {
      supported: true,
      supportedAgents: ["Alpha CLI"],
      agent: "alpha",
    },
    createdAt: timestamp,
    updatedAt: timestamp,
  },
  {
    id: "betacfg2",
    name: "Beta CLI",
    icon: "codex",
    cmd: ["/usr/bin/env", "bash", "-ic", 'exec -- "$@"', "webmux", "beta"],
    envNames: [],
    chatOptions: [],
    chat: false,
    chatSupport: {
      supported: false,
      supportedAgents: [],
      reason: "unknown_agent",
    },
    createdAt: timestamp,
    updatedAt: timestamp,
  },
];

const localFixture: AgentToolsReport = {
  checkedAt: timestamp,
  shell: "/bin/bash",
  tools: [
    {
      id: "alpha",
      title: "Alpha CLI",
      icon: "claudecode",
      kind: "hybrid",
      installed: true,
      executable: "alpha",
      executables: ["alpha"],
      path: "/usr/local/bin/alpha",
      installedVersion: "1.0.0",
      verifiedVersion: "1.1.0",
      belowVerified: true,
      updateStatus: "unchecked",
      upgradeCommand: "alpha update",
      docsUrl: "https://example.com/alpha",
      connector: {
        executable: "alpha-acp",
        title: "Alpha ACP",
        installed: false,
        updateStatus: "not_installed",
        installCommand: "npm install -g @example/alpha-acp",
        docsUrl: "https://example.com/alpha-acp",
      },
    },
    {
      id: "beta",
      title: "Beta CLI",
      icon: "codex",
      kind: "terminal",
      installed: false,
      executable: "beta",
      executables: ["beta"],
      updateStatus: "not_installed",
      installCommand: "npm install -g @example/beta",
    },
  ],
};

const remoteFixture: AgentToolsReport = {
  ...localFixture,
  remoteCheckedAt: "2026-04-15T08:31:00Z",
  tools: [
    {
      ...localFixture.tools[0]!,
      latestVersion: "1.2.0",
      updateStatus: "update_available",
    },
    localFixture.tools[1]!,
  ],
};

type Call = { method: string; url: string; body?: unknown };

function serve(options: { failInitialTools?: boolean } = {}) {
  const calls: Call[] = [];
  let configs = [...baseConfigs];
  let toolsCalls = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input);
      const parsed = new URL(url, location.href);
      const method = init?.method ?? "GET";
      const body = init?.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ method, url, body });
      if (parsed.pathname === "/api/settings") {
        return Response.json({
          revision: 0,
          updatedAt: "",
          values: emptySettings,
        });
      }
      if (parsed.pathname === "/api/config") {
        return Response.json({ defaultCwd: "/tmp", portDetection: true });
      }
      if (parsed.pathname === "/api/launch-configs" && method === "GET") {
        return Response.json(configs);
      }
      if (
        parsed.pathname === "/api/launch-configs/presets" &&
        method === "GET"
      ) {
        return Response.json([]);
      }
      if (
        parsed.pathname.startsWith("/api/launch-configs/") &&
        method === "DELETE"
      ) {
        const id = parsed.pathname.split("/").at(-1);
        configs = configs.filter((item) => item.id !== id);
        return new Response(null, { status: 204 });
      }
      if (parsed.pathname === "/api/agent-tools/check" && method === "POST") {
        return Response.json(remoteFixture);
      }
      if (parsed.pathname === "/api/agent-tools" && method === "GET") {
        toolsCalls++;
        if (options.failInitialTools && toolsCalls === 1) {
          return Response.json({ error: "扫描超时" }, { status: 500 });
        }
        return Response.json(localFixture);
      }
      return Response.json({ error: "not found" }, { status: 404 });
    }),
  );
  return calls;
}

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  resetCreationFeatures();
  history.replaceState(null, "", "/");
  localStorage.clear();
});

test("merges installed tool versions into launch config rows, folds uninstalled agent configs, and checks updates on demand", async () => {
  const calls = serve();
  const writeText = vi
    .spyOn(navigator.clipboard, "writeText")
    .mockResolvedValue(undefined);
  stopSettings();
  await reloadSettings();

  render(LaunchConfigsSettings);

  // Shell and installed Alpha CLI appear in the active list; Alpha merges env summary and version status.
  const editAlpha = page.getByRole("button", { name: "编辑 Alpha CLI" });
  await expect.element(editAlpha).toBeVisible();
  await expect.element(editAlpha).toMatchTextContent(/1 个环境变量/);
  const activeList = page.getByRole("list", { name: "启动配置列表" });
  await expect.element(activeList).toMatchTextContent(/1\.0\.0/);
  await expect.element(activeList).toMatchTextContent(/低于已验证 1\.1\.0/);
  await expect.element(activeList).toMatchTextContent(/缺少聊天适配器/);

  // Mount never triggers remote POST /api/agent-tools/check.
  expect(
    calls.some((call) => call.url.endsWith("/api/agent-tools/check")),
  ).toBe(false);

  // Copy missing connector install command.
  await page.getByRole("button", { name: "复制 Alpha ACP 安装命令" }).click();
  expect(writeText).toHaveBeenCalledWith("npm install -g @example/alpha-acp");

  // Uninstalled Beta CLI config is folded by default; expanding exposes both install command and launch config management.
  const uninstalledToggle = page.getByRole("button", {
    name: "未安装的工具（1）",
  });
  await expect
    .element(uninstalledToggle)
    .toHaveAttribute("aria-expanded", "false");
  await uninstalledToggle.click();
  await expect
    .element(uninstalledToggle)
    .toHaveAttribute("aria-expanded", "true");
  const editBeta = page.getByRole("button", { name: "编辑 Beta CLI" });
  await expect.element(editBeta).toBeVisible();
  const uninstalledList = page.getByRole("list", {
    name: "未安装的 Agent 工具",
  });
  await expect.element(uninstalledList).toMatchTextContent(/未安装 · 已隐藏/);
  await page.getByRole("button", { name: "复制 Beta CLI 安装命令" }).click();
  expect(writeText).toHaveBeenCalledWith("npm install -g @example/beta");

  // Checking updates triggers POST /api/agent-tools/check and updates the inline row status.
  await page.getByRole("button", { name: "检查更新" }).click();
  await expect
    .element(page.getByText("可更新至 1.2.0", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByText("1 个可更新", { exact: true }))
    .toBeVisible();
  expect(
    calls.some(
      (call) =>
        call.method === "POST" && call.url.endsWith("/api/agent-tools/check"),
    ),
  ).toBe(true);

  // Rescan triggers GET /api/agent-tools?refresh=1.
  await page.getByRole("button", { name: "重新扫描本机工具" }).click();
  await vi.waitFor(() =>
    expect(
      calls.some((call) => call.url.endsWith("/api/agent-tools?refresh=1")),
    ).toBe(true),
  );
});

test("settings navigation merges launch configs and agent tools into one entry and redirects #agent-tools", async () => {
  serve();
  history.replaceState(null, "", "/settings#agent-tools");
  stopSettings();
  await reloadSettings();

  render(Settings);

  const nav = page.getByRole("navigation", { name: "设置分区", exact: true });
  await expect
    .element(nav.getByRole("link", { name: "启动配置", exact: true }))
    .toBeVisible();
  expect(nav.element().querySelector('a[href="#agent-tools"]')).toBeNull();
  await expect.poll(() => document.activeElement?.id).toBe("launch-configs");
});

test("renders warning notice when local scan fails while keeping launch configs usable, and recovers on reload", async () => {
  serve({ failInitialTools: true });
  stopSettings();
  await reloadSettings();

  render(LaunchConfigsSettings);

  await expect
    .element(page.getByText("无法读取本机 Agent 工具状态", { exact: true }))
    .toBeVisible();
  await expect.element(page.getByText("扫描超时")).toBeVisible();
  await expect
    .element(page.getByRole("button", { name: "编辑 Alpha CLI" }))
    .toBeVisible();

  await page.getByRole("button", { name: "重新读取" }).click();
  await expect
    .element(page.getByText("1 个已安装", { exact: true }))
    .toBeVisible();
});

test("enables Check updates when only an external ACP connector is installed", async () => {
  render(LaunchConfigsSettings, {
    fixture: {
      configs: [],
      presets: [],
      launchState: "ready",
      agentState: "ready",
      report: {
        checkedAt: timestamp,
        shell: "/bin/bash",
        tools: [
          {
            id: "alpha",
            title: "Alpha CLI",
            icon: "claudecode",
            kind: "hybrid",
            installed: false,
            executable: "alpha",
            executables: ["alpha"],
            updateStatus: "not_installed",
            connector: {
              executable: "alpha-acp",
              title: "Alpha ACP",
              installed: true,
              path: "/usr/local/bin/alpha-acp",
              installedVersion: "0.9.0",
              updateStatus: "unchecked",
              docsUrl: "https://example.com/alpha-acp",
            },
          },
        ],
      },
    },
  });

  await expect
    .element(page.getByRole("button", { name: "检查更新" }))
    .toBeEnabled();
});
