import { expect, type Locator, type Page } from "@playwright/test";

async function showWorkspaceTree(page: Page, tree: Locator, open: Locator) {
  const disclosure = page.getByRole("button", {
    name: "仓库与工作树",
    exact: true,
  });
  await expect(tree.or(open).or(disclosure).first()).toBeVisible({
    timeout: 2000,
  });
  if (await tree.isVisible()) return false;
  const opened = !(await disclosure.isVisible());
  if (opened) await open.click({ timeout: 2000 });
  if (!(await tree.isVisible())) await disclosure.click({ timeout: 2000 });
  return opened;
}

// Opens a sidebar tree row's "⋯" menu (the phone drawer on narrow
// screens); `label` matches the start of the menu button's name. Returns
// whether the phone drawer had to be opened.
async function openSidebarMenu(page: Page, label: RegExp) {
  const tree = page.getByRole("tree", { name: "仓库与会话" });
  const open = page.getByRole("button", { name: "打开侧栏" });
  const trigger = tree.getByRole("button", { name: label }).first();
  let opened = false;
  // The sidebar renders after the page loads and a phone drawer may still be
  // sliding shut from an earlier step: retry from whichever form is shown.
  await expect(async () => {
    if (await showWorkspaceTree(page, tree, open)) opened = true;
    await trigger.click({ timeout: 2000 });
  }).toPass();
  return opened;
}

// Picks an item of a sidebar tree row's "⋯" menu; `label` matches the start
// of the menu button's name.
async function sidebarMenu(page: Page, label: RegExp, item: string) {
  await openSidebarMenu(page, label);
  await page.getByRole("menuitem", { name: item }).click();
}

const escape = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * Picks an item of a repository's "⋯" menu in the sidebar tree, the one
 * place that holds repository actions.
 */
export async function repositoryMenu(
  page: Page,
  item: string,
  repositoryName?: string,
) {
  // Without a name the row's own menu is meant, not the "全部仓库" row's.
  await sidebarMenu(
    page,
    repositoryName === undefined
      ? /^仓库操作：(?!全部仓库$)/
      : new RegExp(`^仓库操作：${escape(repositoryName)}`),
    item,
  );
}

/**
 * Picks an item of the "全部仓库" row's "⋯" menu in the sidebar tree, where
 * registry-level actions like "添加仓库…" live.
 */
export async function allRepositoriesMenu(page: Page, item: string) {
  await sidebarMenu(page, /^仓库操作：全部仓库$/, item);
}

/**
 * Opens the "全部仓库" row's "⋯" menu without picking an item, so a case can
 * assert an item's disabled state. Returns whether the phone drawer had to
 * be opened; an Escape closes the menu, another closes that drawer.
 */
export async function openAllRepositoriesMenu(page: Page) {
  return await openSidebarMenu(page, /^仓库操作：全部仓库$/);
}

/**
 * Picks an item of a worktree's "⋯" menu in the sidebar tree (details and,
 * for a linked worktree, removal). The worktree row must be shown, that is
 * its repository unfolded.
 */
export async function worktreeMenu(
  page: Page,
  item: string,
  worktreeName: string,
) {
  await sidebarMenu(
    page,
    new RegExp(`^工作树操作：${escape(worktreeName)}$`),
    item,
  );
}

/**
 * Unfolds a repository's row in the sidebar tree (the phone drawer on
 * narrow screens) so its worktree rows, and their menus, are shown.
 */
export async function unfoldRepository(page: Page, repositoryName: string) {
  const tree = page.getByRole("tree", { name: "仓库与会话" });
  const open = page.getByRole("button", { name: "打开侧栏" });
  const toggle = tree.getByRole("button", {
    name: new RegExp(`^(展开|收起)${escape(repositoryName)}$`),
  });
  await expect(async () => {
    await showWorkspaceTree(page, tree, open);
    await expect(toggle).toBeVisible({ timeout: 2000 });
  }).toPass();
  if ((await toggle.getAttribute("aria-label"))?.startsWith("展开"))
    await toggle.click();
}

/** Asserts the repository panel overview's "主页 n / 共 m" worktree count. */
export async function expectHomeWorktrees(
  page: Page,
  shown: number,
  total: number,
  repositoryName?: string,
) {
  // A closed phone drawer stays in the DOM, so look only at the open dialog.
  const dialog = page.locator('[role="dialog"][data-state="open"]');
  await expect(async () => {
    await repositoryMenu(page, "仓库详情…", repositoryName);
    try {
      await expect(dialog).toContainText(`主页 ${shown} / 共 ${total}`, {
        timeout: 2000,
      });
    } finally {
      await page.keyboard.press("Escape");
      await expect(dialog).toHaveCount(0);
    }
  }).toPass();
}
