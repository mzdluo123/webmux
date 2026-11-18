import { expect, test } from "@playwright/test";

for (const width of [639, 640]) {
  for (const reducedMotion of ["reduce", "no-preference"] as const) {
    test(`project picker retains its context across ${width}px (${reducedMotion})`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height: 240 });
      await page.emulateMedia({ reducedMotion });
      await page.goto("/iframe.html?id=workspace-project-picker--projects");
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await page.keyboard.press("Escape");
      const opener = page.getByRole("button", {
        name: "新建会话",
        exact: true,
      });
      await opener.click();
      await dialog.getByRole("button", { name: "添加项目…" }).click();
      const query = dialog.locator("input").first();
      await query.fill("qa-probe");
      await query.evaluate((input: HTMLInputElement) =>
        input.setSelectionRange(2, 5),
      );
      const viewport = dialog
        .locator('[data-slot="scroll-area-viewport"]')
        .first();
      await dialog.evaluate(async (element) => {
        await Promise.all(
          element
            .getAnimations({ subtree: true })
            .filter(
              (animation) =>
                animation.effect?.getComputedTiming().endTime !== Infinity,
            )
            .map((animation) => animation.finished.catch(() => undefined)),
        );
      });
      const scroll = await viewport.evaluate((el) => {
        el.scrollTop = 16;
        return el.scrollTop;
      });
      expect(scroll).toBeGreaterThan(0);
      await page.setViewportSize({
        width: width === 639 ? 640 : 639,
        height: 240,
      });
      await expect(
        dialog.getByRole("heading", { name: "添加项目" }),
      ).toBeVisible();
      await expect(query).toBeFocused();
      await expect(query).toHaveValue("qa-probe");
      expect(
        await query.evaluate((input: HTMLInputElement) => [
          input.selectionStart,
          input.selectionEnd,
        ]),
      ).toEqual([2, 5]);
      expect(await viewport.evaluate((el) => el.scrollTop)).toBe(scroll);
      await page.keyboard.press("Escape");
      await expect(
        dialog.getByRole("heading", { name: "选择项目" }),
      ).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await expect(opener).toBeFocused();
    });
  }
}
