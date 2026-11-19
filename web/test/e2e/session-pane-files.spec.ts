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
  const sessionHeight = (await page
    .locator("[data-dock-panel]:visible > .dock-title")
    .boundingBox())!.height;
  await page.getByRole("button", { name: "文件树 · File review" }).click();
  const tree = page.getByRole("region", { name: "文件树", exact: true });
  const readers = page.locator('[data-dock-panel^="reader:"]:visible');
  await expect(tree).toBeVisible();
  expect(
    (await tree.locator(":scope > .dock-title").boundingBox())!.height,
  ).toBe(sessionHeight);
  await expect(readers).toHaveCount(0);
  await tree.getByRole("button", { name: "first.txt", exact: true }).click();
  await expect(readers).toContainText("Contents of first.txt");
  expect(
    (await readers.locator(":scope > .dock-title").boundingBox())!.height,
  ).toBe(sessionHeight);
  if (testInfo.project.use.isMobile) {
    await page.getByRole("button", { name: "打开面板", exact: true }).click();
    const picker = page.getByRole("dialog", { name: "打开面板" });
    await expect(
      picker.getByRole("button", { name: /左右分屏|上下分屏/ }),
    ).toHaveCount(0);
    await picker
      .getByRole("button", { name: /^收起「预览 · first\.txt」$/ })
      .click();
    await expect(tree).toBeVisible();
    await tree.getByRole("button", { name: "返回", exact: true }).click();
    await expect(page.locator(".split-terminal")).toBeVisible();
    await expect(page.locator(".xterm-helper-textarea")).not.toBeFocused();
    return;
  }
  await tree.getByRole("button", { name: "second.txt", exact: true }).click();
  const strip = readers.locator(".dock-title");
  await strip.getByRole("tab", { name: /^first\.txt/ }).click();
  await expect(readers.getByRole("tabpanel")).toContainText(
    "Contents of first.txt",
  );
  await strip.getByRole("button", { name: /second\.txt/ }).click();
  await expect(strip.getByRole("tab")).toHaveCount(1);
  await expect(strip.getByRole("tab")).toHaveText("first.txt");
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
