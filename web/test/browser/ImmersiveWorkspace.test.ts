import { afterEach, expect, test } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./ImmersiveWorkspaceHarness.svelte";

afterEach(cleanup);

test("compact code viewing keeps physical chat input focusable during XR and preserves the draft on exit", async () => {
  await render(Harness);
  const input = page.getByLabelText("聊天输入");
  await expect.element(input).not.toBeVisible();
  await page.getByRole("button", { name: "进入 VR 布局" }).click();
  await expect.element(input).toBeVisible();
  await input.fill("保留的物理键盘草稿");
  await expect.element(input).toHaveFocus();
  await page.getByRole("button", { name: "退出 VR 布局" }).click();
  await expect.element(input).not.toBeVisible();
  await page.getByRole("button", { name: "进入 VR 布局" }).click();
  await expect.element(input).toHaveValue("保留的物理键盘草稿");
});
