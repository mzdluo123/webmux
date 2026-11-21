import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./LazyDialogHarness.svelte";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
});

test("omits the floating loading notice while the dialog chunk loads", async () => {
  const deferred = Promise.withResolvers<string>();
  const onclose = vi.fn();
  await render(Harness, {
    load: () => deferred.promise,
    label: "创建表单",
    onclose,
  });

  await expect
    .element(page.getByText("正在加载创建表单…"))
    .not.toBeInTheDocument();
  await expect.element(page.getByRole("status")).not.toBeInTheDocument();

  deferred.resolve("创建表单内容");
  await expect
    .element(page.getByRole("dialog", { name: "创建表单" }))
    .toHaveTextContent("创建表单内容");
});

test("still shows the floating error notice when loading fails", async () => {
  const deferred = Promise.withResolvers<string>();
  const onclose = vi.fn();
  await render(Harness, {
    load: () => deferred.promise,
    onclose,
  });

  deferred.reject(new Error("chunk failed"));
  await expect
    .element(page.getByText("端口转发加载失败，请刷新页面后重试。"))
    .toBeVisible();
  await userEvent.click(page.getByRole("button", { name: "关闭" }));
  expect(onclose).toHaveBeenCalledOnce();
});
