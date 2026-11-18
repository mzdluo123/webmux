import { chmod, mkdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "./fixtures";
import type { AgentToolsReport } from "../../src/contracts/schemas";

test("settings agent-tools card scans local tools via startup shell, copies commands, and checks updates on demand", async ({
  page,
  request,
  server,
}) => {
  const remoteChecks: string[] = [];
  page.on("request", (req) => {
    if (
      req.method() === "POST" &&
      new URL(req.url()).pathname === "/api/agent-tools/check"
    ) {
      remoteChecks.push(req.url());
    }
  });

  await page.goto("/settings#agent-tools");

  const card = page.locator("#agent-tools");
  await expect(card).toBeVisible();
  await expect(page.locator("#launch-configs + #agent-tools")).toHaveCount(1);

  const nav = page.getByRole("navigation", { name: "设置分区", exact: true });
  await expect(
    nav.locator(
      'li:has(> a[href="#launch-configs"]) + li:has(> a[href="#agent-tools"])',
    ),
  ).toHaveCount(1);

  const installedList = card.getByRole("list", {
    name: "已安装的 Agent 工具",
    exact: true,
  });
  const claudeItem = installedList
    .locator('[data-slot="item"]')
    .filter({ hasText: "Claude Code" });
  await expect(claudeItem).toBeVisible();
  await expect(claudeItem.getByText("2.1.316", { exact: true })).toBeVisible();
  await expect(
    claudeItem.getByText("聊天适配器 · claude-agent-acp", { exact: true }),
  ).toBeVisible();
  await expect(claudeItem.getByText("0.99.1", { exact: true })).toBeVisible();

  const codexItem = installedList
    .locator('[data-slot="item"]')
    .filter({ hasText: "Codex CLI" });
  await expect(codexItem).toBeVisible();
  await expect(codexItem.getByText("0.166.0", { exact: true })).toBeVisible();

  // Opening /settings#agent-tools must never trigger outbound remote version checks.
  expect(remoteChecks).toEqual([]);

  const fitsViewport = () =>
    page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    );
  expect(await fitsViewport()).toBe(true);

  // Copy upgrade command for an installed tool.
  await card
    .getByRole("button", { name: "复制 Claude Code 升级命令", exact: true })
    .click();
  await expect(
    card.getByRole("button", { name: "已复制命令", exact: true }),
  ).toBeVisible();

  // Expand uninstalled tools collapsible when at least one preset CLI is absent.
  const uninstalledToggle = card.getByRole("button", {
    name: /^未安装的工具（\d+）$/,
  });
  if ((await uninstalledToggle.count()) > 0) {
    await expect(uninstalledToggle).toHaveAttribute("aria-expanded", "false");
    await uninstalledToggle.click();
    await expect(uninstalledToggle).toHaveAttribute("aria-expanded", "true");
    expect(await fitsViewport()).toBe(true);
  }

  // Verify interactive startup-shell (.bashrc) PATH resolution on explicit rescan.
  const home = server.env.HOME!;
  const customBin = join(home, "custom-agent-bin");
  const bashrc = join(home, ".bashrc");
  const fakeBuddy = join(customBin, "codebuddy");
  try {
    await mkdir(customBin, { recursive: true });
    await writeFile(fakeBuddy, '#!/bin/sh\necho "codebuddy 8.9.10"\n');
    await chmod(fakeBuddy, 0o755);
    await writeFile(bashrc, `export PATH="${customBin}:$PATH"\n`);

    await card
      .getByRole("button", { name: "重新扫描本机工具", exact: true })
      .click();
    const buddyItem = installedList
      .locator('[data-slot="item"]')
      .filter({ hasText: "CodeBuddy Code" });
    await expect(buddyItem).toBeVisible();
    await expect(buddyItem.getByText("8.9.10", { exact: true })).toBeVisible();
  } finally {
    await rm(customBin, { recursive: true, force: true });
    await rm(bashrc, { force: true });
  }

  // Trigger on-demand update check without depending on external registries in E2E.
  const baselineResponse = await request.get("/api/agent-tools");
  expect(baselineResponse.ok()).toBe(true);
  const baseline = (await baselineResponse.json()) as AgentToolsReport;
  const mockedRemote: AgentToolsReport = {
    ...baseline,
    remoteCheckedAt: "2026-04-15T08:31:00Z",
    tools: baseline.tools.map((tool) =>
      tool.id === "claudecode"
        ? {
            ...tool,
            latestVersion: "9.9.9",
            updateStatus: "update_available",
          }
        : tool,
    ),
  };
  await page.route("**/api/agent-tools/check", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(mockedRemote),
    });
  });

  await card.getByRole("button", { name: "检查更新", exact: true }).click();
  await expect(card.getByText("1 个可更新", { exact: true })).toBeVisible();
  await expect(
    installedList.getByText("可更新至 9.9.9", { exact: true }),
  ).toBeVisible();
  await expect(
    installedList.locator("code", { hasText: "claude update" }),
  ).toBeVisible();
  expect(remoteChecks).toHaveLength(1);
  expect(await fitsViewport()).toBe(true);
});
