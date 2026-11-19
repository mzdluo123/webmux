import { writeFile } from "node:fs/promises";
import { join } from "node:path";
import { createShell, expect, test } from "./fixtures";

test("browse before reading and compare files without replacing the session", async ({
  page,
  workDir,
}, testInfo) => {
  await Promise.all(
    ["first.txt", "second.txt"].map((name) =>
      writeFile(join(workDir, name), `Contents of ${name}`),
    ),
  );
  await createShell(page, "File review", workDir);
  await page.getByRole("button", { name: "文件树 · File review" }).click();
  const tree = page.getByRole("region", { name: "文件树", exact: true });
  const readers = page.locator('[data-dock-panel^="reader:"]:visible');
  await expect(tree).toBeVisible();
  await expect(readers).toHaveCount(0);
  await tree.getByRole("button", { name: "first.txt", exact: true }).click();
  await expect(readers).toContainText("Contents of first.txt");
  if (testInfo.project.use.isMobile) {
    await readers.getByRole("button", { name: "返回", exact: true }).click();
    await expect(tree).toBeVisible();
    await tree.getByRole("button", { name: "返回", exact: true }).click();
    await expect(page.locator(".split-terminal")).toBeVisible();
    await expect(page.locator(".xterm-helper-textarea")).not.toBeFocused();
    return;
  }
  await tree
    .getByRole("button", { name: /second\.txt/ })
    .first()
    .click({ button: "right" });
  await page.getByRole("menuitem", { name: "在右侧打开" }).click();
  await expect(readers).toHaveCount(2);
  await expect(
    readers.filter({ hasText: "Contents of first.txt" }),
  ).toBeVisible();
  await expect(
    readers.filter({ hasText: "Contents of second.txt" }),
  ).toBeVisible();
  await expect(page.locator(".split-terminal")).toBeVisible();
});
