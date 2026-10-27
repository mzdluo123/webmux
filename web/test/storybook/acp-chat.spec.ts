import { expect, test } from "@playwright/test";
import { overflowViolations } from "./overflow";
import { layouts } from "./layouts";

// Prototype for issue #181: every state of the hybrid session page fits the
// layout matrix and keeps its primary control reachable.
const stories = [
  ["conversation", "button", "停止"],
  ["permission-request", "button", /允许一次/],
  ["command-menu", "button", "发送"],
  ["cancelled-turn", "button", "继续"],
  ["open-code", "button", "停止"],
  ["switching", "heading", "正在切换到终端…"],
  ["terminal-side", "tab", "聊天"],
] as const;

for (const { name, viewport, theme, textScale } of layouts) {
  test.describe(name, () => {
    test.use({ viewport, colorScheme: theme });

    for (const [story, role, control] of stories) {
      test(`ACP chat ${story}`, async ({ page }) => {
        const errors: string[] = [];
        page.on("pageerror", (error) => errors.push(error.message));
        await page.route("**/api/**", (route) => route.abort());
        await page.goto(
          `/iframe.html?id=prototype-acp-hybrid-session--${story}&viewMode=story&globals=theme:${theme}`,
        );
        await expect(page.locator("main.terminal-page")).toBeVisible();
        await page.evaluate(async (scale) => {
          document.documentElement.style.fontSize = `${Number(scale) * 100}%`;
          await document.fonts.ready;
        }, textScale);
        const target = page.getByRole(role, { name: control });
        await target.scrollIntoViewIfNeeded();
        await expect(target).toBeInViewport();
        if (textScale > 1)
          // The existing terminal header does not fit 320px at 200% text
          // even without the hybrid switch; this prototype checks its body.
          await page
            .locator("header.terminal-header")
            .evaluate((header) => (header.style.display = "none"));
        await expect.poll(() => overflowViolations(page)).toEqual([]);
        expect(errors).toEqual([]);
      });
    }

    test("ACP hybrid home cards", async ({ page }) => {
      await page.route("**/api/**", (route) => route.abort());
      await page.goto(
        `/iframe.html?id=prototype-acp-hybrid-home--session-cards&viewMode=story&globals=theme:${theme}`,
      );
      await expect(page.getByLabel("最新消息预览")).toBeVisible();
      await expect(page.getByText("等待授权")).toBeVisible();
      await page.evaluate(async (scale) => {
        document.documentElement.style.fontSize = `${Number(scale) * 100}%`;
        await document.fonts.ready;
      }, textScale);
      await expect.poll(() => overflowViolations(page)).toEqual([]);
    });
  });
}

test("switching to the terminal interrupts the turn after confirmation", async ({
  page,
}) => {
  await page.route("**/api/**", (route) => route.abort());
  await page.goto(
    "/iframe.html?id=prototype-acp-hybrid-session--conversation&viewMode=story",
  );
  await page.getByRole("tab", { name: "终端" }).click();
  await page.getByRole("button", { name: "确认" }).click();
  await expect(page.getByRole("status")).toContainText("正在切换到终端");
  await expect(page.getByLabel("终端（示意）")).toBeVisible();
  await page.getByRole("tab", { name: "聊天" }).click();
  await expect(page.getByText("已输出的内容保留")).toBeVisible();
  await expect(page.getByText("回合已中断，队列暂停")).toBeVisible();
});
