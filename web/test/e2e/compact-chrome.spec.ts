import { execFileSync } from "node:child_process";
import { writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { Locator, Page } from "@playwright/test";
import {
  test,
  expect,
  createShell,
  closePanelPicker,
  openSessionMenu,
  toggleCode,
  openSplitPanel,
  sessionHeader,
  sessionViewTabs,
} from "./fixtures";
import { codeReader, fileTree } from "./code-dock";

function git(root: string, ...args: string[]) {
  execFileSync("git", ["-C", root, ...args], {
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "E2E",
      GIT_AUTHOR_EMAIL: "e2e@example.com",
      GIT_COMMITTER_NAME: "E2E",
      GIT_COMMITTER_EMAIL: "e2e@example.com",
    },
  });
}

async function modifiedRepository(workDir: string) {
  git(workDir, "init", "-q");
  const original =
    Array.from({ length: 200 }, (_, i) => `ordinary line ${i + 1}`).join("\n") +
    "\n";
  await writeFile(join(workDir, "code.txt"), original);
  git(workDir, "add", ".");
  git(workDir, "commit", "-qm", "initial");
  await writeFile(
    join(workDir, "code.txt"),
    original.replace("ordinary line 50\n", "changed line 50\n"),
  );
}

async function expectOneRow(row: Locator) {
  const box = (await row.boundingBox())!;
  for (const child of await row.locator(":scope > :visible").all()) {
    const bounds = (await child.boundingBox())!;
    expect(bounds.y).toBeGreaterThanOrEqual(box.y - 0.5);
    expect(bounds.y + bounds.height).toBeLessThanOrEqual(
      box.y + box.height + 0.5,
    );
  }
  expect(
    await row.evaluate((element) => element.scrollWidth - element.clientWidth),
  ).toBeLessThanOrEqual(0);
}

async function openDiff(page: Page) {
  const files = fileTree(page);
  await files.getByRole("tab", { name: "修改", exact: true }).click();
  await files
    .getByRole("button", { name: "修改 code.txt", exact: true })
    .first()
    .click();
  await expect(
    page.locator(".code-diff-reader:visible [data-code]").first(),
  ).toContainText("changed line 50");
}

test("narrow headers keep one row and move rare actions into the menu", async ({
  page,
  workDir,
}, testInfo) => {
  await page.setViewportSize({ width: 393, height: 851 });
  await createShell(page, "demo-agent", workDir);
  const header = sessionHeader(page);
  const toolbar = page.locator(".session-toolbar");
  const actions = toolbar.getByRole("button", {
    name: "终端菜单",
    exact: true,
  });
  const title = header.getByRole("button", {
    name: "demo-agent",
    exact: true,
  });
  const label = title.getByText("demo-agent", { exact: true });
  const sidebar = toolbar.getByRole("button", {
    name: "打开侧栏",
    exact: true,
  });
  const switcher = toolbar.getByRole("button", {
    name: "快速切换会话",
    exact: true,
  });
  const panels = toolbar.getByRole("button", { name: "打开面板", exact: true });
  const files = toolbar.getByRole("button", { name: /^文件树 · / });
  const display = toolbar.getByRole("button", { name: "显示", exact: true });
  await expect(panels).toHaveAttribute("title", "打开面板");
  for (const width of [393, 320]) {
    await page.setViewportSize({ width, height: 851 });
    await expectOneRow(toolbar.locator("header"));
    for (const button of [sidebar, switcher, files, panels, actions, display]) {
      await expect(button).toBeVisible();
      expect(await button.boundingBox()).toMatchObject({
        width: 44,
        height: 44,
      });
    }
    await expectOneRow(header);
    await expect(title).toBeVisible();
    expect(await label.evaluate((e) => e.scrollWidth)).toBeLessThanOrEqual(
      await label.evaluate((e) => e.clientWidth),
    );
    await expect(header.getByRole("button")).toHaveCount(1);
    await expect(header.locator("[data-dock-actions]")).toHaveCount(0);
    for (const name of ["全屏", "上传文件", "滚动到最新位置", "快捷指令"])
      await expect(
        toolbar.getByRole("button", { name, exact: true }),
      ).toHaveCount(0);
    await expect(sessionViewTabs(page)).toHaveCount(0);
    await toggleCode(page);
    await expect(fileTree(page)).toBeVisible();
    await expect(header).toBeHidden();
    await expect(actions).toHaveCount(0);
    await expect(codeReader(page)).toBeHidden();
    await toggleCode(page);
  }
  // Soft keys exist on touch devices; a narrow desktop window has the menu.
  if (testInfo.project.use.isMobile) {
    const draft = page
      .getByRole("complementary", { name: "终端软键" })
      .getByRole("button", { name: /^草稿模式/ });
    await draft.click();
    await expect(draft).toHaveAttribute("aria-pressed", "true");
    await draft.click();
    await expect(draft).toHaveAttribute("aria-pressed", "false");
  }
  await openSplitPanel(page);
  await expect(
    page
      .getByRole("dialog", { name: "打开面板", exact: true })
      .getByRole("button", { name: "新建会话…", exact: true }),
  ).toBeVisible();
  await closePanelPicker(page);
  await page
    .locator(".session-toolbar")
    .getByRole("button", { name: "显示", exact: true })
    .click();
  await expect(
    page.getByRole("menuitem", { name: "快速切换会话", exact: true }),
  ).toHaveCount(0);
  // Draft mode left the phone header for the soft keys and this menu.
  await expect(
    page.getByRole("menuitem", { name: "全屏", exact: true }),
  ).toBeEnabled();
  await page.keyboard.press("Escape");
  await openSessionMenu(page);
  await expect(
    page.getByRole("menuitem", { name: "上传文件", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole("menuitem", { name: "滚动到最新位置", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("menuitemcheckbox", { name: /^草稿模式/ }),
  ).toBeVisible();
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("menuitem", { name: "上传文件", exact: true }).click();
  await chooser;
});

test("a phone Diff leaves most of the screen to code and reaches document actions from the code menu", async ({
  page,
  workDir,
}) => {
  await modifiedRepository(workDir);
  await page.setViewportSize({ width: 393, height: 851 });
  await createShell(page, "Phone diff", workDir);
  await toggleCode(page);
  await openDiff(page);
  const tabs = page.getByRole("tablist", { name: "文件标签" });
  await expectOneRow(tabs.locator(".."));
  const readerTitle = codeReader(page).locator(":scope > .dock-title");
  await expect(readerTitle.locator("[data-dock-actions]")).toHaveCount(0);
  await expect(
    readerTitle.getByRole("button", { name: /^收起「/ }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "完整文件", exact: true }),
  ).toBeHidden();
  await expect(
    page.getByRole("button", { name: "在独立窗口中显示代码" }),
  ).toBeHidden();
  const code = (await page
    .locator(".code-diff-reader")
    .filter({ visible: true })
    .boundingBox())!;
  expect(code.height).toBeGreaterThanOrEqual(851 * 0.7);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth - innerWidth,
    ),
  ).toBeLessThanOrEqual(0);

  const readerMenu = codeReader(page).getByRole("button", {
    name: "代码菜单",
    exact: true,
  });
  await readerMenu.click();
  const menu = page.getByRole("menu");
  await expect(menu).toContainText(join(workDir, "code.txt"));
  const box = (await menu.boundingBox())!;
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.y + box.height).toBeLessThanOrEqual(851);
  await page.getByRole("menuitem", { name: /修改前的文件/ }).click();
  await expect(
    page.getByRole("tab", { name: /^code\.txt · index(?: ●)?$/ }),
  ).toHaveAttribute("aria-selected", "true");
  await readerMenu.click();
  await page.getByRole("menuitem", { name: "跳转行", exact: true }).click();
  await page.locator("[data-code-goto-line]:visible input").fill("50");
  await page.locator("[data-code-goto-line]:visible input").press("Enter");
  await expect(
    page.locator("[data-code-reader]:visible [data-content]"),
  ).toContainText("ordinary line 50");
  // The Diff tab and its reading state stay open beside the snapshot.
  await expect(
    page.getByRole("tab", { name: /^code\.txt · unstaged(?: ●)?$/ }),
  ).toBeVisible();
});

test("desktop terminal commands live in the workspace bar and code stays on single rows", async ({
  page,
  workDir,
}, info) => {
  test.skip(info.project.use.isMobile, "Wide desktop layout");
  await modifiedRepository(workDir);
  await page.setViewportSize({ width: 1440, height: 900 });
  await createShell(page, "Wide layout", workDir);
  const toolbar = page.locator(".session-toolbar");
  const commands = toolbar.locator(".terminal-header");
  await expect(commands).toBeVisible();
  await expect(page.locator("[data-dock-panel] .terminal-header")).toHaveCount(
    0,
  );
  await expect(page.locator("[data-dock-panel] > .dock-title")).toHaveCount(0);
  for (const name of ["全屏", "打开面板", "窗格"])
    await expect(
      toolbar.getByRole("button", { name, exact: true }),
    ).toBeVisible();
  await expect(
    toolbar.getByRole("button", { name: "快速切换会话", exact: true }),
  ).toBeHidden();
  await expect(
    page
      .getByRole("button", { name: "快速切换会话", exact: true })
      .filter({ visible: true }),
  ).toHaveCount(1);
  for (const name of ["Wide layout", "快捷指令"])
    await expect(
      commands.getByRole("button", { name, exact: true }),
    ).toBeVisible();
  await expect(
    commands.getByRole("button", { name: "终端菜单", exact: true }),
  ).toHaveText("终端");
  for (const name of ["上传文件", "草稿模式", "滚动到最新位置"])
    await expect(
      commands.getByRole("button", { name, exact: true }),
    ).toHaveCount(0);
  const terminalMenu = await openSessionMenu(page);
  await expect(
    terminalMenu.getByRole("menuitem", { name: "上传文件", exact: true }),
  ).toBeEnabled();
  await expect(
    terminalMenu.getByRole("menuitemcheckbox", {
      name: "草稿模式",
      exact: true,
    }),
  ).not.toBeChecked();
  await page.keyboard.press("Escape");
  await toggleCode(page);
  await openDiff(page);
  await expect(commands).toHaveCount(0);
  await expect(fileTree(page).getByRole("navigation")).toContainText(workDir);
  const tabs = page.getByRole("tablist", { name: "文件标签" });
  await expect(tabs).toBeVisible();
  const menu = toolbar.getByRole("button", { name: "代码菜单", exact: true });
  await expect(
    toolbar.getByRole("button", { name: "文件内查找", exact: true }),
  ).toBeVisible();
  await expect(
    codeReader(page).getByRole("button", { name: "代码菜单", exact: true }),
  ).toHaveCount(0);
  await menu.click();
  await expect(page.getByRole("menu")).toContainText(join(workDir, "code.txt"));
  await expect(
    page.getByRole("menuitem", { name: "刷新全部", exact: true }),
  ).toBeVisible();
});
