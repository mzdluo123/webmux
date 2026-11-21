import { chmod, mkdir, rm, writeFile } from "node:fs/promises";
import { basename, join } from "node:path";
import { expect, openSessionQueue, test } from "./fixtures";
import {
  chooseSessionDirectory,
  chooseSessionProject,
} from "./session-project";
import type { AgentToolsReport } from "../../src/contracts/schemas";

test("merged launch-configs card fuses tool versions, hides uninstalled agents in New session, and reveals newly installed tools on rescan", async ({
  page,
  request,
  server,
  workDir,
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

  // Restore all system presets so installed and uninstalled preset agents are present.
  const restored = await request.post("/api/launch-configs/presets", {
    data: {},
  });
  expect(restored.ok()).toBe(true);

  const fitsViewport = () =>
    page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    );

  // Uninstalled preset agent (CodeBuddy Code) is automatically hidden in New Session -> Run,
  // while Shell and installed Claude Code remain selectable.
  await page.goto("/");
  await openSessionQueue(page);
  await page
    .getByRole("main")
    .getByRole("button", { name: "＋ 新建会话", exact: true })
    .first()
    .click();
  const dialog = page.getByRole("dialog", { name: "新建会话" });
  await expect(dialog).toBeVisible();
  await chooseSessionDirectory(page, workDir);
  await dialog.getByRole("button", { name: "运行", exact: true }).click();
  await expect(
    page.getByRole("option", { name: /^Claude Code/ }),
  ).toBeVisible();
  await expect(page.getByRole("option", { name: /^Shell/ })).toBeVisible();
  await expect(
    page.getByRole("option", { name: /^CodeBuddy Code/ }),
  ).toHaveCount(0);
  await page.keyboard.press("Escape");
  await page.keyboard.press("Escape");

  // Legacy #agent-tools hash lands on the single merged #launch-configs section.
  await page.goto("/settings#agent-tools");

  const card = page.locator("#launch-configs");
  await expect(card).toBeVisible();

  const nav = page.getByRole("navigation", { name: "设置分区", exact: true });
  await expect(nav.locator('a[href="#launch-configs"]')).toHaveCount(1);
  await expect(nav.locator('a[href="#agent-tools"]')).toHaveCount(0);

  const activeList = card.getByRole("list", {
    name: "启动配置列表",
    exact: true,
  });
  const claudeItem = activeList
    .locator('[data-slot="item"]')
    .filter({ hasText: "Claude Code" });
  await expect(claudeItem).toBeVisible();
  await expect(claudeItem.getByText("2.1.316", { exact: true })).toBeVisible();
  await expect(
    claudeItem.getByText("聊天适配器 · claude-agent-acp", { exact: true }),
  ).toBeVisible();
  await expect(claudeItem.getByText("0.99.1", { exact: true })).toBeVisible();

  const codexItem = activeList
    .locator('[data-slot="item"]')
    .filter({ hasText: "Codex CLI" });
  await expect(codexItem).toBeVisible();
  await expect(codexItem.getByText("0.166.0", { exact: true })).toBeVisible();

  // Opening settings must never trigger outbound remote version checks.
  expect(remoteChecks).toEqual([]);
  expect(await fitsViewport()).toBe(true);

  // Copy upgrade command for an installed tool row.
  await card
    .getByRole("button", { name: "复制 Claude Code 升级命令", exact: true })
    .click();
  await expect(
    card.getByRole("button", { name: "已复制命令", exact: true }),
  ).toBeVisible();

  // Uninstalled preset configs are folded into the collapsible section and remain editable/manageable.
  const uninstalledToggle = card.getByRole("button", {
    name: /^未安装的工具（\d+）$/,
  });
  await expect(uninstalledToggle).toHaveAttribute("aria-expanded", "false");
  await uninstalledToggle.click();
  await expect(uninstalledToggle).toHaveAttribute("aria-expanded", "true");
  await expect(
    card.getByRole("button", { name: "编辑 CodeBuddy Code", exact: true }),
  ).toBeVisible();
  expect(await fitsViewport()).toBe(true);

  // Verify interactive startup-shell (.bashrc) PATH resolution on explicit rescan,
  // and confirm the newly installed agent becomes selectable in New Session -> Run.
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
    const buddyItem = activeList
      .locator('[data-slot="item"]')
      .filter({ hasText: "CodeBuddy Code" });
    await expect(buddyItem).toBeVisible();
    await expect(buddyItem.getByText("8.9.10", { exact: true })).toBeVisible();

    await page.goto("/");
    await openSessionQueue(page);
    await page
      .getByRole("main")
      .getByRole("button", { name: "＋ 新建会话", exact: true })
      .first()
      .click();
    await chooseSessionProject(page, basename(workDir));
    await page
      .getByRole("dialog", { name: "新建会话" })
      .getByRole("button", { name: "运行", exact: true })
      .click();
    await expect(
      page.getByRole("option", { name: /^CodeBuddy Code/ }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await page.keyboard.press("Escape");

    await page.goto("/settings#launch-configs");
  } finally {
    await rm(customBin, { recursive: true, force: true });
    await rm(bashrc, { force: true });
  }

  // Trigger on-demand update check without depending on external registries in E2E.
  const baselineResponse = await request.get("/api/agent-tools?refresh=1");
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
    activeList.getByText("可更新至 9.9.9", { exact: true }),
  ).toBeVisible();
  await expect(
    activeList.locator("code", { hasText: "claude update" }),
  ).toBeVisible();
  expect(remoteChecks).toHaveLength(1);
  expect(await fitsViewport()).toBe(true);
});
