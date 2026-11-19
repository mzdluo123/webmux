import type { Page } from "@playwright/test";
import { expect, toggleCode } from "./fixtures";

export const fileTree = (page: Page) =>
  page.getByRole("region", { name: "文件树", exact: true });

export const readerPanel = (page: Page) =>
  page.locator('[data-dock-panel^="reader:"]');

export const codeReader = (page: Page) =>
  readerPanel(page).getByRole("region", { name: "代码工作区", exact: true });

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
