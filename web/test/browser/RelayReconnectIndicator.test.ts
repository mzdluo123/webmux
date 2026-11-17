import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import RelayReconnectIndicator from "../../src/components/shell/RelayReconnectIndicator.svelte";
import { locale } from "../../src/lib/locale";

beforeEach(() => locale.setPreference("zh-CN"));
afterEach(async () => {
  await cleanup();
  locale.setPreference("zh-CN");
});

test("reconnecting stays a corner indicator that opens status details", async () => {
  const onretry = vi.fn();
  const view = await render(RelayReconnectIndicator, {
    checking: true,
    returnTo: "/s/aaaaaaaa?message=turn-1",
    onretry,
  });
  const trigger = page.getByRole("button", {
    name: "重连状态：连接已断开，重连中…",
  });
  await expect.element(trigger).toBeVisible();
  await trigger.click();
  await expect
    .element(
      page.getByText(
        "保持此标签页打开可保留当前页面和未发送的草稿。中断的操作不会自动重发。",
      ),
    )
    .toBeVisible();
  const retry = page.getByRole("button", { name: "再次检查连接" });
  await expect.element(retry).toBeDisabled();
  await expect
    .element(page.getByRole("link", { name: "通过 WebMux 控制台登录" }))
    .toHaveAttribute("href", "/auth/relay/start?returnTo=%2Fs%2Faaaaaaaa");

  await view.rerender({ checking: false });
  await expect
    .element(
      page.getByRole("button", { name: "重连状态：无法连接主机，重连已暂停" }),
    )
    .toBeVisible();
  await expect
    .element(
      page.getByText("主机仍不可用，自动检查已暂停。连接恢复后可再次检查。"),
    )
    .toBeVisible();
  await retry.click();
  expect(onretry).toHaveBeenCalledOnce();
});

test("lifts above bottom-docked chrome so it never covers composers", async () => {
  const dock = document.createElement("div");
  dock.setAttribute("data-bottom-dock", "");
  Object.assign(dock.style, {
    position: "fixed",
    left: "0",
    right: "0",
    bottom: "0",
    height: "120px",
    background: "transparent",
  });
  document.body.appendChild(dock);
  try {
    await render(RelayReconnectIndicator, {
      checking: true,
      returnTo: "/s/aaaaaaaa",
      onretry: () => {},
    });
    const trigger = page.getByRole("button", {
      name: "重连状态：连接已断开，重连中…",
    });
    await expect.element(trigger).toBeVisible();
    const gap =
      dock.getBoundingClientRect().top -
      trigger.element().getBoundingClientRect().bottom;
    expect(gap).toBeGreaterThanOrEqual(7);
    expect(gap).toBeLessThanOrEqual(9);
  } finally {
    dock.remove();
  }
});
