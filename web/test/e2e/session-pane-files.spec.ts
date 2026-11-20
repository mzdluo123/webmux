import { writeFile } from "node:fs/promises";
import { join } from "node:path";
import { createShell, expect, openDockCommands, test } from "./fixtures";

test("browse before reading and compare files without replacing the session", async ({
  page,
  workDir,
}, testInfo) => {
  await Promise.all(
    ["first.txt", "second.txt"].map((name) =>
      writeFile(join(workDir, name), `Contents of ${name}`),
    ),
  );
  const id = await createShell(page, "File review", workDir);
  if (!testInfo.project.use.isMobile) {
    await expect(
      page.locator(`[data-dock-panel="${id}"] > .dock-title`),
    ).toHaveCount(0);
  }
  await page.getByRole("button", { name: "文件树 · File review" }).click();
  const tree = page.getByRole("region", { name: "文件树", exact: true });
  const readers = page.locator('[data-dock-panel^="reader:"]:visible');
  await expect(tree).toBeVisible();
  const sessionHeight = (await page
    .locator(
      testInfo.project.use.isMobile
        ? "[data-dock-panel]:visible > .dock-title"
        : `[data-dock-panel="${id}"] > .dock-title`,
    )
    .boundingBox())!.height;
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
    await picker.getByRole("button", { name: /^收起「first\.txt」$/ }).click();
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
  await expect(readers).toContainText("Contents of first.txt");
  await expect(page.locator('[data-dock-panel^="reader:"]')).toHaveCount(1);
  await tree.getByRole("button", { name: "second.txt", exact: true }).click();
  await readers.getByRole("tab", { name: /^first\.txt/ }).click();
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
  const terminal = page.locator(`[data-dock-panel="${id}"]`);
  const first = readers.filter({ hasText: "Contents of first.txt" });
  const second = readers.filter({ hasText: "Contents of second.txt" });
  await first.locator(".dock-title-space").dragTo(terminal);
  const stack = page.locator("[data-dock-panel]:visible [data-dock-stack]");
  const file = stack.getByRole("tab", { name: /^first\.txt/ });
  const shell = stack.getByRole("tab", { name: "File review", exact: true });
  await expect(first.locator(".dock-title").getByRole("tab")).toHaveCount(2);
  await expect(file.locator("svg")).toHaveCount(1);
  await expect(shell.locator("svg")).toHaveCount(1);
  await shell.click();
  await expect(terminal.locator(".xterm")).toBeVisible();
  // Drag the background file, leaving the selected shell where it is.
  const box = (await second.boundingBox())!;
  await file.dragTo(second, {
    targetPosition: { x: box.width / 2, y: box.height - 8 },
  });
  await expect(terminal.locator(".xterm")).toBeVisible();
  await expect(first).toBeVisible();
  await expect
    .poll(async () => (await first.boundingBox())!.y)
    .toBeGreaterThan((await second.boundingBox())!.y);
  await (
    await openDockCommands(first)
  )
    .getByRole("menuitem", { name: "关闭窗格", exact: true })
    .click();
  await expect(first).toHaveCount(0);
  await expect(second).toBeVisible();
  await expect(terminal).toBeVisible();
  await (
    await openDockCommands(terminal)
  )
    .getByRole("menuitem", { name: "关闭窗格", exact: true })
    .click();
  await expect(terminal).toHaveCount(0);
  expect((await page.request.get(`/api/sessions/${id}`)).ok()).toBe(true);
});
