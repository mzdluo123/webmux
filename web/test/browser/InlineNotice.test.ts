import { afterEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./InlineNoticeHarness.svelte";

afterEach(async () => {
  try {
    await cleanup();
  } finally {
    await page.viewport(1024, 900);
  }
});

test("single-line InlineNotice centers icon, copy, and action button on the same axis", async () => {
  await page.viewport(1280, 900);
  const onaction = vi.fn();
  render(Harness, { onaction });
  await document.fonts.ready;

  const alert = page.getByRole("alert");
  await expect.element(alert).toBeVisible();
  const el = alert.element();
  const svg = el.querySelector("svg")!;
  const copy = el.querySelector('[data-testid="notice-copy"]')!;
  const button = el.querySelector("button")!;

  const svgRect = svg.getBoundingClientRect();
  const copyRect = copy.getBoundingClientRect();
  const buttonRect = button.getBoundingClientRect();

  const svgCenter = svgRect.top + svgRect.height / 2;
  const copyCenter = copyRect.top + copyRect.height / 2;
  const buttonCenter = buttonRect.top + buttonRect.height / 2;

  expect(Math.abs(svgCenter - copyCenter)).toBeLessThanOrEqual(1);
  expect(Math.abs(svgCenter - buttonCenter)).toBeLessThanOrEqual(1);

  await page.getByRole("button", { name: "关闭" }).click();
  expect(onaction).toHaveBeenCalledOnce();
});

test("multi-line InlineNotice keeps the icon aligned with the first line of copy", async () => {
  await page.viewport(390, 844);
  render(Harness, {
    text: "仓库详情加载失败，请检查当前网络连接或刷新页面后重试；若刚刚升级过服务端版本，重新加载页面即可获取最新前端分块。",
  });
  await document.fonts.ready;

  const el = page.getByRole("alert").element();
  const iconBox = el.querySelector("svg")!.parentElement!;
  const copy = el.querySelector('[data-testid="notice-copy"]')!;
  const copyBlock = copy.parentElement!;

  const iconRect = iconBox.getBoundingClientRect();
  const copyBlockRect = copyBlock.getBoundingClientRect();

  expect(copyBlockRect.height).toBeGreaterThan(iconRect.height * 1.5);
  expect(Math.abs(iconRect.top - copyBlockRect.top)).toBeLessThanOrEqual(1);
});

test("empty actions snippet hides the actions wrapper without adding layout gap", async () => {
  render(Harness, { showAction: false, actionSlot: true });
  const el = page.getByRole("alert").element();
  const actionsEl = el.querySelector('[data-slot="inline-notice-actions"]')!;
  expect(getComputedStyle(actionsEl).display).toBe("none");
});

test("floating surface uses an opaque background while inline keeps translucent tint", async () => {
  render(Harness, {
    surface: "inline",
    actionSlot: false,
  });
  const inlineBg = getComputedStyle(
    page.getByRole("alert").element(),
  ).backgroundColor;
  expect(inlineBg).toContain("/");
  await cleanup();

  render(Harness, {
    surface: "floating",
    actionSlot: true,
  });
  const floatingEl = page.getByRole("alert").element();
  expect(floatingEl.getAttribute("data-surface")).toBe("floating");
  const floatingBg = getComputedStyle(floatingEl).backgroundColor;
  expect(floatingBg).not.toContain("/");
});
