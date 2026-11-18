import "../../src/styles.css";
import { afterEach, beforeEach, expect, test } from "vite-plus/test";
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
    await render(RelayAuthGate, {
      reason,
      returnTo: "/s/aaaaaaaa?message=turn-1",
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
  },
);
