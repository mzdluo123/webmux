import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import AgentToolsSettings from "../../src/components/settings/AgentToolsSettings.svelte";
import type { AgentToolsReport } from "../../src/contracts/schemas";

const localFixture: AgentToolsReport = {
  checkedAt: "2026-04-15T08:30:00Z",
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

function serve(failInitial = false) {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const method = init?.method ?? "GET";
      const url = String(input);
      const body = init?.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ method, url, body });
      if (failInitial && calls.length === 1) {
        return Response.json({ error: "扫描超时" }, { status: 500 });
      }
      if (method === "POST" && url.endsWith("/api/agent-tools/check")) {
        return Response.json(remoteFixture);
      }
      return Response.json(localFixture);
    }),
  );
  return calls;
}

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

test("loads local status on mount without triggering remote check, expands uninstalled tools, and checks updates on demand", async () => {
  const calls = serve();
  const writeText = vi
    .spyOn(navigator.clipboard, "writeText")
    .mockResolvedValue(undefined);

  render(AgentToolsSettings);

  await expect
    .element(page.getByText("Alpha CLI", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByText("低于已验证 1.1.0", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByText("缺少聊天适配器", { exact: true }))
    .toBeVisible();

  // Mount only calls GET /api/agent-tools, never POST /api/agent-tools/check.
  expect(calls).toEqual([
    {
      method: "GET",
      url: expect.stringMatching(/\/api\/agent-tools$/),
      body: undefined,
    },
  ]);

  // Copy missing connector install command.
  await page.getByRole("button", { name: "复制 Alpha ACP 安装命令" }).click();
  expect(writeText).toHaveBeenCalledWith("npm install -g @example/alpha-acp");

  // Uninstalled tools are collapsed by default and expand on click.
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
  await expect
    .element(page.getByText("Beta CLI", { exact: true }))
    .toBeVisible();
  await page.getByRole("button", { name: "复制 Beta CLI 安装命令" }).click();
  expect(writeText).toHaveBeenCalledWith("npm install -g @example/beta");

  // Clicking Check updates triggers POST /api/agent-tools/check and shows available update.
  await page.getByRole("button", { name: "检查更新" }).click();
  await expect
    .element(page.getByText("可更新至 1.2.0", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByText("1 个可更新", { exact: true }))
    .toBeVisible();
  expect(calls.some((c) => c.method === "POST")).toBe(true);

  // Rescan button triggers GET /api/agent-tools?refresh=1.
  await page.getByRole("button", { name: "重新扫描本机工具" }).click();
  await vi.waitFor(() =>
    expect(
      calls.some((c) => c.url.endsWith("/api/agent-tools?refresh=1")),
    ).toBe(true),
  );
});

test("renders error notice and recovers on reload", async () => {
  const calls = serve(true);
  render(AgentToolsSettings);

  await expect
    .element(page.getByText("无法读取本机 Agent 工具状态", { exact: true }))
    .toBeVisible();
  await expect.element(page.getByText("扫描超时")).toBeVisible();

  await page.getByRole("button", { name: "重新读取" }).click();
  await expect
    .element(page.getByText("Alpha CLI", { exact: true }))
    .toBeVisible();
  expect(calls).toHaveLength(2);
});

test("enables Check updates when only an external ACP connector is installed", async () => {
  render(AgentToolsSettings, {
    fixture: {
      state: "ready",
      report: {
        checkedAt: "2026-04-15T08:30:00Z",
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
