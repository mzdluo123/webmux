import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import RelayAuthGate from "../../src/components/shell/RelayAuthGate.svelte";
import { locale } from "../../src/lib/locale";

beforeEach(() => locale.setPreference("zh-CN"));
afterEach(async () => {
  await cleanup();
  locale.setPreference("zh-CN");
});

test.each([
  ["expired", "中继登录已到期，请重新登录后继续。"],
  [
    "revoked",
    "中继访问权限已撤销或你已退出登录。仅在仍有此主机访问权限时重新登录。",
  ],
  ["unauthenticated", "请通过官方中继登录后访问此主机。"],
] as const)(
  "%s offers official login without a misleading Token error",
  async (reason, message) => {
    const onretry = vi.fn();
    await render(RelayAuthGate, {
      reason,
      returnTo: "/s/aaaaaaaa?message=turn-1",
      onretry,
    });
    await expect
      .element(
        page.getByRole("heading", { name: "通过官方中继连接", level: 1 }),
      )
      .toBeVisible();
    await expect.element(page.getByText(message)).toBeVisible();
    await expect
      .element(page.getByRole("link", { name: "通过 WebMux 控制台登录" }))
      .toHaveAttribute("href", "/auth/relay/start?returnTo=%2Fs%2Faaaaaaaa");
    expect(document.querySelector('input[type="password"]')).toBeNull();
    expect(document.body.textContent).not.toContain("Token");
    expect(
      page.getByRole("button", { name: "再次检查连接" }).elements(),
    ).toEqual([]);
    expect(onretry).not.toHaveBeenCalled();
  },
);

test("reconnecting explains draft preservation and permits an explicit retry after checks stop", async () => {
  const onretry = vi.fn();
  const view = await render(RelayAuthGate, {
    reason: "reconnecting",
    checking: true,
    returnTo: "/s/aaaaaaaa",
    onretry,
  });
  const retry = page.getByRole("button", { name: "再次检查连接" });
  await expect.element(retry).toBeDisabled();
  await expect
    .element(
      page.getByText(
        "保持此标签页打开可保留当前页面和未发送的草稿。中断的操作不会自动重发。",
      ),
    )
    .toBeVisible();
  await view.rerender({ checking: false });
  await expect
    .element(
      page.getByText("主机仍不可用，自动检查已暂停。连接恢复后可再次检查。"),
    )
    .toBeVisible();
  await retry.click();
  expect(onretry).toHaveBeenCalledOnce();
});
