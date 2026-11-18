import { createShell, expect, test } from "./fixtures";

const longTitle =
  "Dock layout review: keep the full session name available on a narrow phone";

test("the session title opens its full name and renames the current session", async ({
  page,
  workDir,
}, testInfo) => {
  const phone = !!testInfo.project.use.isMobile;
  if (phone) await page.setViewportSize({ width: 320, height: 568 });
  await createShell(page, longTitle, workDir);
  const header = page.locator(".terminal-header");
  const title = header.getByRole("button", { name: longTitle, exact: true });
  if (phone) {
    await expect
      .poll(() =>
        title
          .locator("span")
          .evaluate((node) => node.scrollWidth > node.clientWidth),
      )
      .toBe(true);
  }
  await title.click();
  const details = page.getByRole("dialog", { name: "会话名称", exact: true });
  await expect(details.getByText(longTitle, { exact: true })).toBeVisible();
  await details.getByRole("button", { name: "修改标题", exact: true }).click();
  const rename = page.getByRole("dialog", { name: "修改标题", exact: true });
  const field = rename.getByRole("textbox");
  await expect(field).toHaveValue(longTitle);
  await field.fill("Dock layout");
  await rename.getByRole("button", { name: "保存", exact: true }).click();
  const renamed = header.getByRole("button", {
    name: "Dock layout",
    exact: true,
  });
  await expect(renamed).toBeFocused();
  await page.reload();
  await expect(renamed).toBeVisible();
});
