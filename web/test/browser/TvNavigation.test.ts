import { afterEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./TvNavigationHarness.svelte";
import TreeHarness from "./TvTreeHarness.svelte";
import RadioHarness from "./TvRadioHarness.svelte";
import TextareaHarness from "./TvTextareaHarness.svelte";
import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";
import { tick } from "svelte";
import { xrPresentation } from "../../src/hooks/shared/xr-presentation.svelte";

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  tvMode.enabled = false;
  xrPresentation.active = false;
  localStorage.removeItem("webmux.workbenchTree");
});

test("TV arrows preserve editors, terminal capture routes once, Back releases before route", async () => {
  await page.viewport(1280, 720);
  await render(Harness);
  const start = page.getByRole("button", { name: "导航起点" });
  await expect.element(start).toHaveFocus();
  await userEvent.keyboard("{ArrowRight}");
  const draft = page.getByRole("textbox", { name: "草稿", exact: true });
  await expect.element(draft).toHaveFocus();
  await draft.fill("abc");
  await userEvent.keyboard("{ArrowLeft}");
  await expect.element(draft).toHaveFocus();
  expect((draft.element() as HTMLInputElement).selectionStart).toBe(2);
  await userEvent.keyboard("{Escape}");
  await expect.element(start).toHaveFocus();
  await expect
    .element(page.getByRole("status", { name: "路由返回" }))
    .toHaveTextContent("0");

  const capture = page.getByRole("button", { name: "终端输入", exact: true });
  await capture.click();
  await expect.element(capture).toHaveAttribute("aria-pressed", "true");
  await userEvent.keyboard("x{ArrowUp}");
  await expect
    .element(page.getByRole("status", { name: "发送字节" }))
    .toHaveTextContent('"x\\u001b[A"');
  await userEvent.keyboard("{Escape}");
  await expect.element(start).toHaveFocus();
  await expect.element(capture).toHaveAttribute("aria-pressed", "false");
  await expect
    .element(page.getByRole("status", { name: "路由返回" }))
    .toHaveTextContent("0");
  await page.getByRole("button", { name: "Esc", exact: true }).click();
  await expect.element(capture).toHaveAttribute("aria-pressed", "true");
  await expect
    .element(page.getByRole("status", { name: "发送字节" }))
    .toHaveTextContent('"x\\u001b[A\\u001b"');
  await userEvent.keyboard("{Escape}{Escape}");
  await expect
    .element(page.getByRole("status", { name: "路由返回" }))
    .toHaveTextContent("1");
});

test("topmost dialog owns Back and bounds directional focus", async () => {
  await render(Harness);
  await page.getByRole("button", { name: "打开对话框" }).click();
  const dialog = page.getByRole("dialog");
  await expect.element(dialog).toBeVisible();
  await userEvent.keyboard("{ArrowDown}");
  expect(dialog.element().contains(document.activeElement)).toBe(true);
  await userEvent.keyboard("{Escape}");
  await expect.element(dialog).not.toBeInTheDocument();
  await expect
    .element(page.getByRole("button", { name: "打开对话框" }))
    .toHaveFocus();
  await expect
    .element(page.getByRole("status", { name: "路由返回" }))
    .toHaveTextContent("0");
});

test("device-local toggle persists and disabling TV restores native keyboard behavior", async () => {
  await render(Harness);
  expect(localStorage.getItem("webmux.tv-mode")).toBe("1");
  tvMode.enabled = false;
  await expect
    .poll(() => document.documentElement.dataset.uiMode)
    .toBeUndefined();
  expect(localStorage.getItem("webmux.tv-mode")).toBe("0");
  const start = page.getByRole("button", { name: "导航起点" });
  await start.click();
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(start).toHaveFocus();
  const helper = document.querySelector<HTMLTextAreaElement>(
    ".xterm-helper-textarea",
  )!;
  helper.focus();
  expect(document.activeElement).toBe(helper);
});

test("production tree keeps expand/collapse keys and a leaf can exit into the main region", async () => {
  await page.viewport(1280, 720);
  await render(TreeHarness);
  const repository = page.getByRole("treeitem", { name: /^webmux[，,]/ });
  await repository.click();
  await userEvent.keyboard("{ArrowLeft}");
  await expect.element(repository).toHaveAttribute("aria-expanded", "false");
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(repository).toHaveAttribute("aria-expanded", "true");
  await expect.element(repository).toHaveFocus();
  const worktree = page.getByRole("treeitem", { name: /^main[，,]/ });
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(worktree).toHaveFocus();
  await userEvent.keyboard("{ArrowLeft}");
  await expect.element(worktree).toHaveAttribute("aria-expanded", "false");
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(worktree).toHaveAttribute("aria-expanded", "true");
  await expect.element(worktree).toHaveFocus();
  await userEvent.keyboard("{ArrowRight}");
  const leaf = page.getByRole("treeitem", {
    name: "TV leaf（空闲）",
    exact: true,
  });
  await expect.element(leaf).toBeVisible();
  await expect.element(leaf).toHaveFocus();
  await userEvent.keyboard("{ArrowRight}");
  await expect
    .element(page.getByRole("button", { name: "主内容操作" }))
    .toHaveFocus();
});

test("immersive input bypasses TV Back and restores TV capture focus on exit", async () => {
  await render(Harness);
  xrPresentation.active = true;
  await tick();
  const helper = document.querySelector<HTMLTextAreaElement>(
    ".xterm-helper-textarea",
  )!;
  helper.focus();
  await userEvent.keyboard("{Escape}{ArrowUp}");
  expect(document.activeElement).toBe(helper);
  await expect
    .element(page.getByRole("status", { name: "发送字节" }))
    .toHaveTextContent('"\\u001b\\u001b[A"');
  await expect
    .element(page.getByRole("status", { name: "路由返回" }))
    .toHaveTextContent("0");
  xrPresentation.active = false;
  await expect
    .element(page.getByRole("button", { name: "终端输入", exact: true }))
    .toHaveFocus();
});

test("TV text fields navigate until confirmed, then preserve caret editing", async () => {
  await page.viewport(1280, 720);
  await render(Harness);
  const draft = page.getByRole("textbox", { name: "草稿", exact: true });
  const dialog = page.getByRole("button", { name: "打开对话框" });
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(draft).toHaveFocus();
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(dialog).toHaveFocus();
  await userEvent.keyboard("{ArrowLeft}{Enter}abc{ArrowLeft}");
  await expect.element(draft).toHaveFocus();
  expect((draft.element() as HTMLInputElement).selectionStart).toBe(2);
  await userEvent.keyboard("{Enter}{ArrowRight}");
  await expect.element(dialog).toHaveFocus();
  await expect.element(draft).toHaveValue("abc");
});

test("native radio groups retain their own axis and release perpendicular TV navigation", async () => {
  await render(RadioHarness);
  const above = page.getByRole("button", { name: "上方按钮" });
  const below = page.getByRole("button", { name: "下方按钮" });
  const list = page.getByRole("radio", { name: "列表", exact: true });
  const preview = page.getByRole("radio", { name: "预览", exact: true });
  await expect.element(above).toHaveFocus();
  await userEvent.keyboard("{ArrowDown}");
  await expect.element(list).toHaveFocus();
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(preview).toHaveFocus();
  await expect.element(preview).toHaveAttribute("aria-checked", "true");
  await userEvent.keyboard("{ArrowDown}");
  await expect.element(below).toHaveFocus();
  await userEvent.keyboard("{ArrowUp}");
  await expect.element(preview).toHaveFocus();
  await userEvent.keyboard("{ArrowLeft}");
  await expect.element(list).toHaveFocus();
  await expect.element(list).toHaveAttribute("aria-checked", "true");
  await userEvent.keyboard("{ArrowUp}");
  await expect.element(above).toHaveFocus();
});

test("native command inputs retain Enter selection instead of confirming plain text editing", async () => {
  await render(RadioHarness);
  await page.getByPlaceholder("命令选择").fill("second");
  await userEvent.keyboard("{Enter}");
  await expect
    .element(page.getByRole("status", { name: "已选命令" }))
    .toHaveTextContent("second");
});

test("TV multiline editing finishes with confirm and preserves Shift+Enter before remote form submission", async () => {
  await render(TextareaHarness);
  const input = page.getByRole("textbox", { name: "多行内容" });
  await input.fill("first");
  await userEvent.keyboard("{Shift>}{Enter}{/Shift}second{Enter}");
  await expect.element(input).toHaveValue("first\nsecond");
  const save = page.getByRole("button", { name: "保存表单" });
  await expect.element(save).toBeVisible();
  await userEvent.keyboard("{ArrowDown}");
  await expect.element(save).toHaveFocus();
  await userEvent.keyboard("{Enter}");
  await expect
    .poll(
      () =>
        page.getByRole("status", { name: "提交结果" }).element().textContent,
    )
    .toBe("first\nsecond");
});

test("copy shortcuts leave confirmed text navigation intact and selection keys enter caret editing", async () => {
  await render(Harness);
  const input = page.getByRole("textbox", { name: "草稿", exact: true });
  const dialog = page.getByRole("button", { name: "打开对话框" });
  await userEvent.keyboard(
    "{ArrowRight}{Enter}abc{Enter}{Control>}c{/Control}{ArrowRight}",
  );
  await expect.element(dialog).toHaveFocus();
  await expect.element(input).toHaveValue("abc");
  await userEvent.keyboard("{ArrowLeft}{Shift>}{ArrowLeft}{/Shift}{ArrowLeft}");
  await expect.element(input).toHaveFocus();
  expect((input.element() as HTMLInputElement).selectionStart).toBe(2);
});

test("shifted button arrows do not fall through into TV spatial navigation", async () => {
  await render(RadioHarness);
  const above = page.getByRole("button", { name: "上方按钮" });
  await expect.element(above).toHaveFocus();
  await userEvent.keyboard("{Shift>}{ArrowDown}{/Shift}");
  await expect.element(above).toHaveFocus();
  await userEvent.keyboard("{ArrowDown}");
  await expect
    .element(page.getByRole("radio", { name: "列表", exact: true }))
    .toHaveFocus();
});
