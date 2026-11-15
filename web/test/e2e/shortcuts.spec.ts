import { readFile } from "node:fs/promises";
import { test, expect, createShell, recordInput } from "./fixtures";

for (const theme of ["light", "dark"] as const) {
  test(`settings shortcut reference supports navigation in ${theme} theme`, async ({
    page,
  }) => {
    await page.emulateMedia({ colorScheme: theme });
    await page.goto("/settings");
    const link = page.getByRole("link", { name: "快捷键", exact: true });
    await link.click();
    await expect(page).toHaveURL(/\/settings#shortcuts$/);
    const reference = page.locator("#shortcuts");
    await expect(reference).toBeFocused();
    await expect(
      reference.getByRole("heading", { name: "窗格管理" }),
    ).toBeInViewport();
    await reference
      .getByRole("heading", { name: "列表与弹窗" })
      .scrollIntoViewIfNeeded();
    await expect(
      reference.getByText("⌘ + K / Ctrl + Alt + K", { exact: true }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);

    await page.setViewportSize({ width: 320, height: 568 });
    await page.reload();
    await expect(reference).toBeFocused();
    await expect(
      reference.getByRole("heading", { name: "窗格管理" }),
    ).toBeInViewport();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  });
}

test("reading shortcuts leaves the terminal input and draft untouched", async ({
  page,
  workDir,
}) => {
  const id = await createShell(page, "Shortcut reference return", workDir);
  const path = await recordInput(page, workDir);
  await expect(
    page.getByRole("button", { name: "快捷键", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "终端菜单", exact: true }).click();
  await expect(
    page.getByRole("menuitem", { name: "快捷键", exact: true }),
  ).toHaveCount(0);
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: "草稿模式", exact: true }).click();
  const draft = page.locator(".terminal-composer textarea");
  await draft.fill("Keep this unsent draft");
  await page.goto(`/settings?session=${id}#shortcuts`);
  await expect(page.locator("#shortcuts")).toBeFocused();
  await page.keyboard.press("Control+b");
  await page.keyboard.press("Shift+5");
  await page.keyboard.type("reference-only");
  expect(await readFile(path, "utf8")).toBe("");

  await page.getByRole("button", { name: "返回终端", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/s/${id}$`));
  await expect(draft).toHaveValue("Keep this unsent draft");
  expect(await readFile(path, "utf8")).toBe("");
});
