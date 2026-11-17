import { expect, test } from "@playwright/test";
import { checkEachLayout, expectNoOverflow } from "./layouts";

for (const story of [
  "foundations",
  "actions",
  "forms",
  "feedback",
  "surfaces",
  "overlays",
  "session-states",
] as const) {
  test(`design system ${story}`, async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.route("**/api/**", (route) => route.abort());
    await page.goto(
      `/iframe.html?id=design-system--${story}&viewMode=story&globals=theme:light`,
    );
    await expect(page.getByTestId("design-system")).toBeVisible();

    await checkEachLayout(page, async ({ name }) => {
      await expectNoOverflow(page, name);

      if (story === "foundations") {
        // Resolve actual rendered CSS colors (including oklch) to sRGB.
        const ratios = await page.evaluate(() => {
          const style = getComputedStyle(document.documentElement);
          const canvas = document.createElement("canvas");
          canvas.width = canvas.height = 1;
          const context = canvas.getContext("2d")!;
          function luminance(token: string) {
            context.clearRect(0, 0, 1, 1);
            context.fillStyle = style.getPropertyValue(`--${token}`).trim();
            context.fillRect(0, 0, 1, 1);
            const rgb = [...context.getImageData(0, 0, 1, 1).data]
              .slice(0, 3)
              .map((c) => {
                const value = c / 255;
                return value <= 0.04045
                  ? value / 12.92
                  : ((value + 0.055) / 1.055) ** 2.4;
              });
            return rgb[0]! * 0.2126 + rgb[1]! * 0.7152 + rgb[2]! * 0.0722;
          }
          const pairs: Array<[string, string]> = [
            ["foreground", "background"],
            ["muted-foreground", "background"],
            ["muted-foreground", "card"],
            ["primary-foreground", "primary"],
            ["success", "card"],
            ["warning", "card"],
            ["destructive", "card"],
            ["info", "card"],
          ];
          return pairs.map(([text, surface]) => {
            const a = luminance(text),
              b = luminance(surface);
            return {
              pair: `${text}/${surface}`,
              ratio: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05),
            };
          });
        });
        for (const { pair, ratio } of ratios)
          expect.soft(ratio, `${name} ${pair}`).toBeGreaterThanOrEqual(4.5);
      }
      if (story === "forms") {
        await page.getByLabel("名称", { exact: true }).fill("测试快捷指令");
        await page.getByRole("button", { name: "保存示例" }).click();
        await expect
          .soft(page.getByRole("status"), name)
          .toContainText("示例已保存");
        await expect
          .soft(page.getByLabel("工作目录"), name)
          .toHaveAttribute("aria-invalid", "true");
        await expect.soft(page.getByLabel("暂不可用"), name).toBeDisabled();
      }
      if (story === "surfaces") {
        await page.getByRole("tab", { name: "最近活动" }).click();
        await expect
          .soft(page.getByRole("tabpanel"), name)
          .toContainText("暂无最近活动");
        await page.getByRole("button", { name: "工作区操作" }).click();
        await expect
          .soft(page.getByRole("menuitem", { name: "删除示例工作区" }), name)
          .toHaveAttribute("data-variant", "destructive");
        await page.keyboard.press("Escape");
        await page.getByRole("tab", { name: "工作区" }).click();
      }
      if (story === "overlays") {
        const trigger = page.getByRole("button", { name: "打开编辑弹窗" });
        await trigger.click();
        const dialog = page.getByRole("dialog");
        await expect.soft(dialog, name).toBeVisible();
        await expectNoOverflow(page, name);
        await dialog.getByRole("button", { name: "取消", exact: true }).click();
        await expect.soft(dialog, name).toBeHidden();
        await expect.soft(trigger, name).toBeFocused();
        const confirm = page.getByRole("button", {
          name: "打开危险操作确认",
        });
        await confirm.click();
        await expect.soft(page.getByRole("alertdialog"), name).toBeVisible();
        await page.getByRole("button", { name: "取消", exact: true }).click();
        await expect.soft(page.getByRole("alertdialog"), name).toBeHidden();
        await expect.soft(confirm, name).toBeFocused();
      }
      expect.soft(errors, name).toEqual([]);
    });
  });
}

for (const theme of ["light", "dark"]) {
  test(`conversation tick colors and bounded volumes (${theme})`, async ({
    page,
  }) => {
    await page.goto(
      `/iframe.html?id=design-system--forms&viewMode=story&globals=theme:${theme}`,
    );
    const user = page.locator('[data-turn-role="user"]');
    const agent = page.locator(
      '[data-turn-role="agent"][data-turn-volume="single"]',
    );
    await expect(user).toBeVisible();
    const colors = await Promise.all(
      [user, agent].map((tick) =>
        tick
          .locator("span")
          .evaluate((node) => getComputedStyle(node).backgroundColor),
      ),
    );
    expect(colors[0]).not.toBe(colors[1]);
    for (const [volume, width] of [
      ["single", 12],
      ["few", 16],
      ["several", 20],
      ["many", 24],
    ] as const) {
      const tick = page.locator(
        `[data-turn-role="agent"][data-turn-volume="${volume}"]`,
      );
      expect(
        await tick
          .locator("span")
          .evaluate((node) => node.getBoundingClientRect().width),
      ).toBe(width);
      expect(
        await tick.evaluate((node) => node.getBoundingClientRect().height),
      ).toBe(12);
    }
    expect(
      await page
        .locator('[data-turn-role="group"] span')
        .evaluate((node) => node.getBoundingClientRect().width),
    ).toBe(24);
  });
}
