import { afterEach, expect, test, vi } from "vite-plus/test";
import { page } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { locale } from "../../src/lib/locale";
import Harness from "./TerminalViewportHarness.svelte";

afterEach(cleanup);

test("bottom toolbar renders as a compact docked strip on narrow viewports, a centered island on wide viewports, and respects hidden mode", async () => {
  try {
    await page.viewport(390, 844);
    const view = await render(Harness, {
      connection: "connected",
      restoring: false,
      width: 390,
      height: 400,
      toolbarMode: "shown",
    });

    const bar = document.querySelector<HTMLElement>(".soft-keys")!;
    const main = document.querySelector<HTMLElement>(".soft-keys-main")!;
    const terminalArea = document.querySelector<HTMLElement>(".terminal-area")!;
    expect(getComputedStyle(bar).display).toBe("flex");
    expect(bar.getBoundingClientRect().top).toBeGreaterThanOrEqual(
      terminalArea.getBoundingClientRect().bottom,
    );
    expect(bar.getBoundingClientRect().height).toBeLessThanOrEqual(36);
    expect(main.getBoundingClientRect().width).toBeCloseTo(390 - 8, 0);

    // Wide tablet/desktop viewport centers the compact pill bar without stretching edge-to-edge.
    await page.viewport(768, 800);
    await view.rerender({
      connection: "connected",
      restoring: false,
      width: 768,
      height: 500,
      toolbarMode: "shown",
    });
    const wideMain = main.getBoundingClientRect();
    expect(wideMain.height).toBeLessThanOrEqual(36);
    expect(wideMain.width).toBeLessThan(600);
    expect(Math.abs(wideMain.left + wideMain.width / 2 - 768 / 2)).toBeLessThan(
      2,
    );

    // Icon buttons in the same row share identical 28x28 bounds.
    const keyboardBox = page
      .getByRole("button", { name: "唤起或收起键盘" })
      .element()
      .getBoundingClientRect();
    const pasteBox = page
      .getByRole("button", { name: "粘贴" })
      .element()
      .getBoundingClientRect();
    expect(keyboardBox.width).toBeCloseTo(28, 0);
    expect(keyboardBox.height).toBeCloseTo(28, 0);
    expect(pasteBox.width).toBeCloseTo(keyboardBox.width, 0);
    expect(pasteBox.height).toBeCloseTo(keyboardBox.height, 0);

    // Hidden mode suppresses the toolbar.
    await view.rerender({
      connection: "connected",
      restoring: false,
      width: 768,
      height: 500,
      toolbarMode: "hidden",
    });
    expect(getComputedStyle(bar).display).toBe("none");
  } finally {
    await page.viewport(1024, 768);
  }
});

test.each(["connecting", "reconnecting"] as const)(
  "a synced %s terminal shows a non-blocking restoration hint",
  async (connection) => {
    const view = await render(Harness, { connection, restoring: false });
    const hint = page.getByText("正在恢复…");
    await expect.element(hint).toBeVisible();
    // The hint floats over retained output without covering or intercepting it.
    expect(hint.element().closest(".pointer-events-none")).not.toBeNull();
    expect(document.querySelector(".restore-overlay")).toBeNull();

    await view.rerender({ connection: "connected", restoring: false });
    await expect.element(hint).not.toBeInTheDocument();
  },
);

test("restoration keeps its blocking overlay labels", async () => {
  const view = await render(Harness, {
    connection: "connecting",
    restoring: true,
  });
  await expect.element(page.getByText("正在连接…")).toBeVisible();
  expect(document.querySelector(".restore-overlay")).not.toBeNull();

  await view.rerender({ connection: "connected", restoring: true });
  await expect.element(page.getByText("正在恢复…")).toBeVisible();

  await view.rerender({ connection: "connected", restoring: false });
  expect(document.querySelector(".restore-overlay")).toBeNull();
});

test("selection and jump controls share a short viewport after replay", async () => {
  const props = {
    selection: "retained text",
    atBottom: false,
    width: 320,
    height: 125,
  };
  const view = await render(Harness, { ...props, restoring: true });
  const host = document.querySelector(".terminal-host")!;
  const jump = page.getByRole("button", { name: "回到最新" });
  const toolbar = page.getByRole("toolbar", { name: "终端文字选择" });
  const copyButton = page.getByRole("button", { name: "复制选中" });
  const clearButton = page.getByRole("button", { name: "取消选择" });
  const grid = host.getBoundingClientRect();
  await expect.element(jump).not.toBeInTheDocument();

  // Selection-only in a < 8rem pane keeps normal text labels visible.
  await view.rerender({
    ...props,
    atBottom: true,
    restoring: false,
    connection: "connected",
  });
  await expect.element(jump).not.toBeInTheDocument();
  await expect.element(toolbar).toBeVisible();
  const soloLabels = [
    ...toolbar
      .element()
      .querySelectorAll<HTMLElement>("button .terminal-control-label"),
  ];
  expect(soloLabels).toHaveLength(2);
  for (const label of soloLabels) {
    expect(getComputedStyle(label).display).not.toBe("none");
    expect(label.getBoundingClientRect().width).toBeGreaterThan(0);
  }
  for (const icon of toolbar
    .element()
    .querySelectorAll<SVGElement>(".terminal-control-icon")) {
    expect(getComputedStyle(icon).display).toBe("none");
  }

  // Both jump and selection in < 8rem separate horizontally as named icon buttons.
  await view.rerender({ ...props, restoring: false, connection: "connected" });
  await expect.element(jump).toBeVisible();
  await expect.element(copyButton).toBeVisible();
  await expect.element(clearButton).toBeVisible();
  await expect.element(toolbar).toBeVisible();
  for (const label of document.querySelectorAll<HTMLElement>(
    ".terminal-controls .terminal-control-label",
  )) {
    expect(getComputedStyle(label).display).toBe("none");
  }
  for (const icon of toolbar
    .element()
    .querySelectorAll<SVGElement>(".terminal-control-icon")) {
    expect(getComputedStyle(icon).display).not.toBe("none");
  }
  const jumpBox = jump.element().getBoundingClientRect();
  const selectionBox = toolbar.element().getBoundingClientRect();
  expect(jumpBox.right).toBeLessThan(selectionBox.left);
  expect(host.getBoundingClientRect()).toEqual(grid);
});

test("floating jump-to-latest button centers near the bottom across desktop and narrow viewports without stealing focus", async () => {
  const scrollToBottom = vi.fn();
  try {
    await page.viewport(1280, 720);
    const view = await render(Harness, {
      connection: "connected",
      restoring: false,
      atBottom: true,
      width: 720,
      scrollToBottom,
    });

    const jump = page.getByRole("button", { name: "回到最新" });
    await expect.element(jump).not.toBeInTheDocument();

    // Restoring or hidden panes never show the floating button even off bottom.
    await view.rerender({
      connection: "connected",
      restoring: true,
      atBottom: false,
      width: 720,
      scrollToBottom,
    });
    await expect.element(jump).not.toBeInTheDocument();
    await view.rerender({
      connection: "connected",
      restoring: false,
      atBottom: false,
      visible: false,
      width: 720,
      scrollToBottom,
    });
    await expect.element(jump).not.toBeInTheDocument();

    for (const [viewportWidth, viewportHeight, hostWidth] of [
      [1280, 720, 720],
      [390, 844, 360],
    ] as const) {
      await page.viewport(viewportWidth, viewportHeight);
      await view.rerender({
        connection: "connected",
        restoring: false,
        atBottom: false,
        visible: true,
        width: hostWidth,
        scrollToBottom,
      });
      await expect.element(jump).toBeVisible();
      const area = document
        .querySelector(".terminal-area")!
        .getBoundingClientRect();
      const softKeys = document
        .querySelector(".soft-keys")!
        .getBoundingClientRect();
      const button = jump.element().getBoundingClientRect();
      expect(
        Math.abs(button.left + button.width / 2 - (area.left + area.width / 2)),
      ).toBeLessThanOrEqual(2);
      expect(button.bottom).toBeLessThanOrEqual(area.bottom);
      expect(area.bottom - button.bottom).toBeLessThanOrEqual(20);
      expect(button.bottom).toBeLessThanOrEqual(softKeys.top);
    }

    const mouseDown = new MouseEvent("mousedown", {
      bubbles: true,
      cancelable: true,
    });
    jump.element().dispatchEvent(mouseDown);
    expect(mouseDown.defaultPrevented).toBe(true);

    await jump.click();
    expect(scrollToBottom).toHaveBeenCalledTimes(1);

    locale.setPreference("en");
    const englishJump = page.getByRole("button", { name: "Jump to bottom" });
    await expect.element(englishJump).toBeVisible();
    locale.setPreference("zh-CN");
    await expect.element(jump).toBeVisible();
  } finally {
    locale.setPreference("system");
    await page.viewport(1024, 768);
  }
});

test("floating upload progress stays inside terminal-area, preserves grid size, and shifts below find bar", async () => {
  const cancelUpload = vi.fn();
  try {
    await page.viewport(1280, 720);
    const view = await render(Harness, {
      connection: "connected",
      restoring: false,
      width: 720,
      cancelUpload,
    });
    const wrapper = document.querySelector(".session-terminal")!;
    const column = document.querySelector(".terminal-column")!;
    const area = document.querySelector(".terminal-area")!;
    const host = document.querySelector(".terminal-host")!;
    const initialHost = host.getBoundingClientRect();

    // Idle terminal fills the full wrapper height with no extra dock below SoftKeys.
    expect(column.getBoundingClientRect().height).toBeCloseTo(
      wrapper.getBoundingClientRect().height,
      0,
    );
    expect(wrapper.children).toHaveLength(1);
    await expect
      .element(page.getByRole("progressbar", { name: "文件上传进度" }))
      .not.toBeInTheDocument();

    const progress = {
      name: "archive.tar.gz",
      index: 1,
      total: 2,
      percent: 42,
      waiting: false,
    };
    await view.rerender({
      connection: "connected",
      restoring: false,
      width: 720,
      uploadProgress: progress,
      cancelUpload,
    });

    const bar = page.getByRole("progressbar", { name: "文件上传进度" });
    await expect.element(bar).toBeVisible();
    const card = document.querySelector<HTMLElement>(
      '[aria-label="文件上传状态"]',
    )!;
    expect(card.parentElement).toBe(area);
    expect(host.getBoundingClientRect()).toEqual(initialHost);

    const desktopArea = area.getBoundingClientRect();
    const desktopCard = card.getBoundingClientRect();
    expect(desktopCard.width).toBeCloseTo(320, 0);
    expect(desktopArea.right - desktopCard.right).toBeCloseTo(8, 0);
    expect(desktopCard.top - desktopArea.top).toBeCloseTo(8, 0);
    expect(desktopCard.bottom).toBeLessThan(desktopArea.bottom - 40);

    // Opening find shifts the floating upload card below the search form.
    await view.rerender({
      connection: "connected",
      restoring: false,
      width: 720,
      findOpen: true,
      uploadProgress: progress,
      cancelUpload,
    });
    const findBox = page
      .getByRole("search", { name: "查找终端输出" })
      .element()
      .getBoundingClientRect();
    const shiftedCard = card.getBoundingClientRect();
    expect(shiftedCard.top).toBeGreaterThanOrEqual(findBox.bottom);
    expect(shiftedCard.bottom).toBeLessThanOrEqual(desktopArea.bottom);
    expect(host.getBoundingClientRect()).toEqual(initialHost);

    // Phone viewport spans inset-x-2 without horizontal overflow or bottom overlap.
    await page.viewport(390, 844);
    await view.rerender({
      connection: "connected",
      restoring: false,
      width: 360,
      findOpen: false,
      uploadProgress: progress,
      cancelUpload,
    });
    const mobileArea = area.getBoundingClientRect();
    const mobileCard = card.getBoundingClientRect();
    expect(Math.abs(mobileCard.left - mobileArea.left - 8)).toBeLessThanOrEqual(
      1,
    );
    expect(
      Math.abs(mobileArea.right - mobileCard.right - 8),
    ).toBeLessThanOrEqual(1);
    expect(mobileCard.bottom).toBeLessThan(mobileArea.bottom - 40);

    await page.getByRole("button", { name: "取消", exact: true }).click();
    expect(cancelUpload).toHaveBeenCalledOnce();

    await view.rerender({
      connection: "connected",
      restoring: false,
      width: 360,
      uploadProgress: null,
      cancelUpload,
    });
    await expect.element(bar).not.toBeInTheDocument();
  } finally {
    await page.viewport(1024, 768);
  }
});
