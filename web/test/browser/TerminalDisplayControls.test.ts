import { afterEach, expect, test, vi } from "vitest";
import { page } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./TerminalDisplayControlsHarness.svelte";
import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";

afterEach(async () => {
  await cleanup();
  tvMode.enabled = false;
});

test("wide supported entry invokes XR once, while active entry only exits", async () => {
  await page.viewport(1280, 720);
  const enter = vi.fn();
  const exit = vi.fn();
  const rendered = await render(Harness, {
    xr: { supported: true, active: false, busy: false, enter, exit },
  });
  await page.getByRole("button", { name: "进入 VR", exact: true }).click();
  expect(enter).toHaveBeenCalledTimes(1);
  expect(exit).not.toHaveBeenCalled();
  await rendered.rerender({
    xr: { supported: true, active: true, busy: false, enter, exit },
  });
  await page.getByRole("button", { name: "退出 VR", exact: true }).click();
  expect(exit).toHaveBeenCalledTimes(1);
  expect(
    page.getByRole("button", { name: "进入 VR", exact: true }).elements(),
  ).toHaveLength(0);
});

test("TV and unsupported browsers retain menu intent so the owner can explain capability errors", async () => {
  await page.viewport(1280, 720);
  tvMode.enabled = true;
  const enter = vi.fn();
  await render(Harness, {
    xr: { supported: false, active: false, busy: false, enter, exit: vi.fn() },
  });
  expect(
    page.getByRole("button", { name: "进入 VR", exact: true }).elements(),
  ).toHaveLength(0);
  await page.getByRole("button", { name: "显示菜单" }).click();
  await page.getByRole("menuitem", { name: "进入 VR", exact: true }).click();
  expect(enter).toHaveBeenCalledTimes(1);
});
