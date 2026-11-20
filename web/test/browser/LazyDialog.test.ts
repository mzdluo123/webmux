import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./LazyDialogHarness.svelte";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
});

test("shows the floating loading notice by default until the dialog resolves", async () => {
  const deferred = Promise.withResolvers<string>();
  const onclose = vi.fn();
  await render(Harness, { load: () => deferred.promise, onclose });

  await expect.element(page.getByText("正在加载端口转发…")).toBeVisible();
  await userEvent.click(page.getByRole("button", { name: "取消" }));
  expect(onclose).toHaveBeenCalledOnce();

  deferred.resolve("已打开");
  await expect
    .element(page.getByRole("dialog", { name: "端口转发" }))
    .toBeVisible();
});

test("omits the floating loading notice when showLoading is false", async () => {
  const deferred = Promise.withResolvers<string>();
  const onclose = vi.fn();
  await render(Harness, {
    load: () => deferred.promise,
    showLoading: false,
    onclose,
  });

  await expect
    .element(page.getByText("正在加载端口转发…"))
    .not.toBeInTheDocument();
  await expect.element(page.getByRole("status")).not.toBeInTheDocument();

  deferred.resolve("端口转发内容");
  await expect
    .element(page.getByRole("dialog", { name: "端口转发" }))
    .toHaveTextContent("端口转发内容");
});

test("still shows the error notice on failure when showLoading is false", async () => {
  const deferred = Promise.withResolvers<string>();
  const onclose = vi.fn();
  await render(Harness, {
    load: () => deferred.promise,
    showLoading: false,
    onclose,
  });

  deferred.reject(new Error("chunk failed"));
  await expect
    .element(page.getByText("端口转发加载失败，请刷新页面后重试。"))
    .toBeVisible();
  await userEvent.click(page.getByRole("button", { name: "关闭" }));
  expect(onclose).toHaveBeenCalledOnce();
});
