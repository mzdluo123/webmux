import type { FrameLocator, Locator, Page } from "@playwright/test";
import { expect, toggleCode } from "./fixtures";

export const fileTree = (page: Page) =>
  page.getByRole("region", { name: "文件树", exact: true });

export const codeReader = (page: Page) =>
  page.locator('[data-dock-panel^="reader:"]:visible');

export async function openCodeMenu(scope: Page | FrameLocator | Locator) {
  const button = { name: "代码菜单", exact: true };
  if (
    "page" in scope &&
    "getAttribute" in scope &&
    !(await scope.getByRole("button", button).isVisible())
  ) {
    await scope.locator(".dock-title-space").click();
    await scope
      .page()
      .locator(".session-toolbar")
      .getByRole("button", button)
      .click();
  } else await scope.getByRole("button", button).click();
}

export async function showFileTree(page: Page) {
  const tree = fileTree(page);
  if (!(await tree.isVisible())) {
    const back = codeReader(page).getByRole("button", {
      name: "文件导航",
      exact: true,
    });
    if (await back.isVisible()) await back.click();
    else await toggleCode(page);
  }
  await expect(tree).toBeVisible();
  return tree;
}
