import { seedSettings } from "./settings-fixture";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import type { FileDiffMetadata } from "@pierre/diffs";
import Harness from "./CodeDiffReaderHarness.svelte";
import { computeDiff } from "../../src/domains/code/code-diff";

beforeEach(async () => {
  await seedSettings();
});
afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  vi.unstubAllGlobals();
});

const shadow = () => document.querySelector("diffs-container")?.shadowRoot;
// Rows of the new side: unified rows carry new line numbers except deletions.
const row = (line: number) =>
  shadow()?.querySelector<HTMLElement>(
    `[data-line="${line}"]:not([data-line-type="change-deletion"])`,
  );

test("captured diff state restores selection, expansion, search and position", async () => {
  const lines = Array.from({ length: 400 }, (_, i) => `ordinary line ${i + 1}`);
  const changed = lines.map((line, i) =>
    i === 49 || i === 299 ? `changed line ${i + 1}` : line,
  );
  const result = computeDiff(
    lines.join("\n") + "\n",
    changed.join("\n") + "\n",
  );
  if (result.kind !== "ready") throw new Error(result.reason);
  const fileDiff = { ...result.fileDiff, name: "code.txt" } as FileDiffMetadata;
  const first = await render(Harness, { fileDiff });
  await expect
    .poll(() => row(50)?.textContent, { timeout: 10000 })
    .toContain("changed line 50");

  await page.getByRole("button", { name: "文件内查找" }).click();
  await page.getByRole("textbox", { name: "查找" }).fill("changed");
  await page.getByRole("button", { name: "下一个匹配", exact: true }).click();

  // Reveal unmodified lines between the two hunks.
  const separator = shadow()!.querySelector('[data-expand-index="1"]')!;
  await page
    .elementLocator(separator)
    .getByRole("button", { name: "向下展开 30 行" })
    .click();
  const [region] = first.component.capture().expanded;
  expect(region).toBeDefined();
  expect(region!.index).toBe(1);
  expect(region!.fromStart + region!.fromEnd).toBe(30);
  // The first revealed line, in either direction.
  const revealed = region!.fromStart > 0 ? 54 : 297 - 30;
  await expect
    .poll(() => row(revealed)?.textContent)
    .toContain(`ordinary line ${revealed}`);

  await page
    .elementLocator(
      shadow()!.querySelector(
        '[data-line-type="change-addition"][data-column-number="50"]',
      )!,
    )
    .click();
  await expect
    .element(page.getByLabelText("Selected lines"))
    .toHaveTextContent("50-50");

  // Bring the revealed line to the top edge.
  const host = document.querySelector<HTMLElement>(".code-diff-reader")!;
  host.scrollTop +=
    row(revealed)!.getBoundingClientRect().top -
    host.getBoundingClientRect().top;
  await expect
    .poll(() => first.component.capture().top)
    .toEqual({ line: revealed, side: "additions" });
  const captured = first.component.capture();
  expect(captured).toMatchObject({
    selected: { start: 50, end: 50, side: "additions" },
    search: { search: "changed" },
    panel: "search",
    cursor: { matches: [50, 300], index: 0 },
  });
  await first.unmount();

  // A hidden tab waits for layout; capture returns the pending state.
  const second = await render(Harness, {
    fileDiff,
    restore: captured,
    hidden: true,
  });
  expect(second.component.capture()).toEqual(captured);
  await second.rerender({ hidden: false });
  const restoredHost =
    document.querySelector<HTMLElement>(".code-diff-reader")!;
  // The revealed line is rendered at the top edge again.
  await expect
    .poll(
      () =>
        (row(revealed)?.getBoundingClientRect().top ?? -1e6) -
        restoredHost.getBoundingClientRect().top,
      { timeout: 10000 },
    )
    .toBeCloseTo(0, 0);
  expect(row(revealed)?.textContent).toContain(`ordinary line ${revealed}`);
  expect(second.component.capture()).toEqual(captured);
  await expect
    .element(page.getByLabelText("Selected lines"))
    .toHaveTextContent("50-50");
  await expect
    .element(page.getByRole("textbox", { name: "查找" }))
    .toHaveValue("changed");
  await expect
    .poll(() =>
      [...document.querySelectorAll('[role="status"]')].map((element) =>
        element.textContent?.replace(/\s+/g, " ").trim(),
      ),
    )
    .toContain("修改后 · 1 / 2");
});

test("a narrower diff column clamps its scroll without moving the longer column", async () => {
  const short = "short old line ".repeat(9);
  const long = "much longer new line ".repeat(17);
  const result = computeDiff(`${short}\n`, `${long}\n`);
  if (result.kind !== "ready") throw new Error(result.reason);
  const fileDiff = { ...result.fileDiff, name: "wide.txt" } as FileDiffMetadata;
  const view = await render(Harness, { fileDiff, width: 900 });
  const columns = () => [
    ...(shadow()?.querySelectorAll<HTMLElement>("[data-code]") ?? []),
  ];
  const limit = (element: HTMLElement) =>
    element.scrollWidth - element.clientWidth;
  await expect.poll(() => columns().length).toBe(2);
  await expect.poll(() => limit(columns()[1]!)).toBeGreaterThan(0);
  const offset = limit(columns()[1]!);
  columns()[1]!.scrollLeft = offset;
  await expect.poll(() => columns()[0]?.scrollLeft).toBe(limit(columns()[0]!));
  expect(columns()[1]!.scrollLeft).toBe(offset);

  await view.rerender({ width: 650 });
  await expect.poll(() => columns().length).toBe(1);
  await expect.poll(() => columns()[0]?.scrollLeft).toBe(limit(columns()[0]!));
  await view.rerender({ width: 900 });
  await expect.poll(() => columns().length).toBe(2);
  await expect.poll(() => columns()[1]?.scrollLeft).toBe(offset);
});

test("diff panel sits above horizontal scrollbar and keeps a bottom selected line visible", async () => {
  const oldLines = [`header ${"w".repeat(180)}`, "tail"];
  const newLines = [
    oldLines[0]!,
    ...Array.from({ length: 35 }, (_, i) => `added line ${i + 1}`),
    oldLines[1]!,
  ];
  const result = computeDiff(
    oldLines.join("\n") + "\n",
    newLines.join("\n") + "\n",
  );
  if (result.kind !== "ready") throw new Error(result.reason);
  const fileDiff = {
    ...result.fileDiff,
    name: "bottom.txt",
  } as FileDiffMetadata;
  render(Harness, { fileDiff, width: 650 });

  const host = document.querySelector<HTMLElement>(".code-diff-reader")!;
  const scrollbar = () =>
    document.querySelector<HTMLElement>("[data-code-scrollbar]");
  await expect
    .poll(() => row(5)?.textContent ?? "", { timeout: 10000 })
    .toContain("added line 4");
  await document.fonts.ready;
  await expect
    .poll(() => scrollbar()?.classList.contains("hidden") ?? true)
    .toBe(false);

  const closedBox = host.getBoundingClientRect();
  const visibleAdditions = [
    ...(shadow()?.querySelectorAll<HTMLElement>(
      '[data-line][data-line-type="change-addition"]',
    ) ?? []),
  ]
    .map((el) => ({
      line: Number(el.dataset.line),
      rect: el.getBoundingClientRect(),
    }))
    .filter(
      ({ line, rect }) =>
        Number.isInteger(line) &&
        rect.top >= closedBox.top &&
        rect.bottom <= closedBox.bottom,
    );
  const bottomTarget = visibleAdditions.at(-1)!;
  expect(bottomTarget).toBeDefined();
  await page
    .elementLocator(
      shadow()!.querySelector(
        `[data-line-type="change-addition"][data-column-number="${bottomTarget.line}"]`,
      )!,
    )
    .click();
  const beforeScrollTop = host.scrollTop;
  const beforeRowBottom = row(bottomTarget.line)!.getBoundingClientRect()
    .bottom;

  await page.getByRole("button", { name: "文件内查找" }).click();
  await expect
    .element(page.getByRole("textbox", { name: "查找" }))
    .toHaveFocus();

  const searchPanel =
    document.querySelector<HTMLElement>("[data-code-search]")!.parentElement!;
  const openBox = host.getBoundingClientRect();
  const panelRect = searchPanel.getBoundingClientRect();
  const barRect = scrollbar()!.getBoundingClientRect();
  expect(panelRect.top).toBeGreaterThanOrEqual(openBox.bottom - 1);
  expect(barRect.top).toBeGreaterThanOrEqual(panelRect.bottom - 1);
  expect(beforeRowBottom).toBeGreaterThan(openBox.bottom);
  expect(host.scrollTop).toBeGreaterThan(beforeScrollTop);
  const afterRowRect = row(bottomTarget.line)!.getBoundingClientRect();
  expect(afterRowRect.top).toBeGreaterThanOrEqual(openBox.top - 0.5);
  expect(afterRowRect.bottom).toBeLessThanOrEqual(openBox.bottom + 0.5);
});

test("emptied file with side=new resolves effectiveSide to old for Ctrl+F search and goto line", async () => {
  const oldLines = Array.from({ length: 120 }, (_, i) =>
    i === 19 || i === 89 ? `removed needle ${i + 1}` : `legacy line ${i + 1}`,
  );
  const result = computeDiff(`${oldLines.join("\n")}\n`, "");
  if (result.kind !== "ready") throw new Error(result.reason);
  const fileDiff = {
    ...result.fileDiff,
    name: "deleted.ts",
    type: "deleted",
  } as FileDiffMetadata;

  const view = await render(Harness, { fileDiff });
  const host = document.querySelector<HTMLElement>(".code-diff-reader")!;
  await expect
    .poll(
      () =>
        shadow()?.querySelector(
          '[data-line="20"][data-line-type="change-deletion"]',
        )?.textContent,
      { timeout: 10000 },
    )
    .toContain("removed needle 20");

  host.focus();
  await userEvent.keyboard("{Control>}f{/Control}");
  const searchInput = page.getByRole("textbox", { name: "查找" });
  await expect.element(searchInput).toHaveFocus();
  await searchInput.fill("removed needle");

  await expect
    .poll(() =>
      [...document.querySelectorAll('[role="status"]')].map((el) =>
        el.textContent?.replace(/\s+/g, " ").trim(),
      ),
    )
    .toContain("修改前 · 0 / 2");

  await page.getByRole("button", { name: "下一个匹配", exact: true }).click();
  await expect
    .poll(() =>
      [...document.querySelectorAll('[role="status"]')].map((el) =>
        el.textContent?.replace(/\s+/g, " ").trim(),
      ),
    )
    .toContain("修改前 · 1 / 2");
  await expect
    .element(page.getByLabelText("Selected lines"))
    .toHaveTextContent("20-20");
  expect(view.component.capture().selected).toEqual({
    start: 20,
    end: 20,
    side: "deletions",
  });

  await page.getByRole("button", { name: "下一个匹配", exact: true }).click();
  await expect
    .element(page.getByLabelText("Selected lines"))
    .toHaveTextContent("90-90");

  await userEvent.keyboard("{Control>}{Alt>}g{/Alt}{/Control}");
  const gotoInput = page.getByLabelText("跳转到行");
  await expect.element(gotoInput).toBeVisible();
  await userEvent.fill(gotoInput, "45");
  await userEvent.keyboard("{Enter}");
  await expect
    .element(page.getByLabelText("Selected lines"))
    .toHaveTextContent("45-45");
  expect(view.component.capture().selected).toEqual({
    start: 45,
    end: 45,
    side: "deletions",
  });
});
