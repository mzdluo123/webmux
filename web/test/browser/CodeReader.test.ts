import { afterEach, expect, test } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import Harness from "./CodeReaderHarness.svelte";

const shadow = () =>
  document.querySelector<HTMLElement>("[data-code-reader] diffs-container")
    ?.shadowRoot;
const row = (line: number) =>
  shadow()?.querySelector<HTMLElement>(`[data-line="${line}"]`);
const gutter = (line: number) =>
  shadow()?.querySelector<HTMLElement>(`[data-column-number="${line}"]`);
const code = () => shadow()?.querySelector<HTMLElement>("[data-code]");
const token = (text: string) =>
  [...(shadow()?.querySelectorAll<HTMLElement>("[data-line] span") ?? [])].find(
    (element) => element.textContent?.trim() === text,
  );
const tokenColor = (text: string) => {
  const element = token(text);
  return element ? getComputedStyle(element).color : "";
};
afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
});

test("read-only virtualized full file, real line selection, search and goto", async () => {
  const text = Array.from(
    { length: 1000 },
    (_, i) => `const line${i + 1} = ${i + 1};`,
  ).join("\n");
  render(Harness, { text });
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("const line1 = 1;");
  expect(shadow()?.querySelectorAll("[data-line]").length).toBeLessThan(100);
  expect(shadow()?.querySelector("[contenteditable]")).toBeNull();
  await page.elementLocator(gutter(2)!).click();
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("2-2"));
  await page.elementLocator(gutter(4)!).click({ modifiers: ["Shift"] });
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("2-4"));
  await page.getByRole("button", { name: "跳转行" }).click();
  await page.getByRole("textbox", { name: "跳转到行" }).fill("900");
  await userEvent.keyboard("{Enter}");
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("const line900");
  await page.getByRole("button", { name: "文件内查找" }).click();
  await page.getByRole("textbox", { name: "查找" }).fill("line850");
  await page.getByRole("button", { name: "下一个匹配", exact: true }).click();
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("const line850");
});

test("goto accepts N:column and scrolls the code horizontally", async () => {
  const long = `const ${"x".repeat(400)} = 1;`;
  const text = Array.from({ length: 1000 }, (_, i) =>
    i === 499 ? long : `line ${i + 1}`,
  ).join("\n");
  render(Harness, { text });
  await expect.poll(() => shadow()?.textContent ?? "").toContain("line 1");
  await page.getByRole("button", { name: "跳转行" }).click();
  await page.getByRole("textbox", { name: "跳转到行" }).fill("500:200");
  await userEvent.keyboard("{Enter}");
  await expect.poll(() => row(500)?.textContent ?? "").toContain("xxxx");
  await expect.poll(() => code()?.scrollLeft ?? 0).toBeGreaterThan(0);
});

test("renders HTML and control-looking content as code text", async () => {
  const text =
    '<img src="invalid" onerror="window.codeExecuted=true">\n<script>window.codeExecuted=true</script>\ncarriage\rreturn\x1b[31m text';
  render(Harness, { text, path: "file.html" });
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("window.codeExecuted");
  expect(shadow()?.querySelector("img[src], [onerror], script")).toBeNull();
  expect(shadow()?.querySelectorAll("[data-line]")).toHaveLength(4);
  expect(
    (window as unknown as { codeExecuted?: boolean }).codeExecuted,
  ).toBeUndefined();
});

test("keeps syntax highlighting across language and view extensions", async () => {
  render(Harness, {
    text: "const answer = 42; // highlighted",
    path: "test.ts",
  });
  await expect
    .poll(() => tokenColor("const"), { timeout: 10_000 })
    .not.toBe("");
  const keyword = tokenColor("const");
  const comment = tokenColor("// highlighted");
  expect(keyword).toMatch(/^rgb\(/);
  expect(comment).toMatch(/^rgb\(/);
  expect(keyword).not.toBe(comment);
});

test("highlights languages matched by extension and filename", async () => {
  render(Harness, {
    text: "name: webmux # highlighted",
    path: "compose.yaml",
  });
  await expect
    .poll(() => tokenColor("# highlighted"), { timeout: 10_000 })
    .not.toBe("");
  await cleanup();
  render(Harness, {
    text: "FROM alpine # highlighted",
    path: "Dockerfile",
  });
  await expect.poll(() => tokenColor("FROM"), { timeout: 10_000 }).not.toBe("");
});

test("retries uppercase extensions as lowercase", async () => {
  render(Harness, {
    text: "const answer = 42;",
    path: "FILE.TS",
  });
  await expect
    .poll(() => tokenColor("const"), { timeout: 10_000 })
    .not.toBe("");
});

test("keeps highlighting files larger than the old tokenize cutoff", async () => {
  const text = [
    "const answer = 42;",
    ...Array.from({ length: 4200 }, (_, i) => `line ${i + 1}`),
  ].join("\n");
  render(Harness, { text, path: "large.ts" });
  await expect
    .poll(() => tokenColor("const"), { timeout: 20_000 })
    .not.toBe("");
});

test("keeps Svelte script highlighting on JavaScript", async () => {
  render(Harness, {
    text: "const answer = 42;",
    path: "App.svelte",
  });
  await expect
    .poll(() => tokenColor("const"), { timeout: 10_000 })
    .not.toBe("");
});

test("captured reading state restores in a new reader, also one mounted hidden", async () => {
  const text = Array.from(
    { length: 1000 },
    (_, i) => `const line${i + 1} = ${i + 1};`,
  ).join("\n");
  const first = await render(Harness, { text });
  await expect
    .poll(() => shadow()?.textContent ?? "")
    .toContain("const line1 = 1;");
  await page.getByRole("button", { name: "文件内查找" }).click();
  await page.getByRole("textbox", { name: "查找" }).fill("line250");
  await page.getByRole("button", { name: "跳转行" }).click();
  await page.getByRole("textbox", { name: "跳转到行" }).fill("210");
  await userEvent.keyboard("{Enter}");
  await expect
    .poll(() => row(210)?.textContent ?? "")
    .toContain("const line210");
  await page.elementLocator(gutter(210)!).click();
  await page.elementLocator(gutter(215)!).click({ modifiers: ["Shift"] });
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("210-215"));
  await page.getByRole("button", { name: "文件内查找" }).click();
  // Scroll halfway into line 200.
  const host = document.querySelector<HTMLElement>("[data-code-reader]")!;
  const scrollIntoLine = () => {
    const line = row(200);
    if (!line) return;
    host.scrollTop +=
      line.getBoundingClientRect().top - host.getBoundingClientRect().top + 11;
  };
  await expect.poll(() => !!row(200)).toBe(true);
  for (let i = 0; i < 4; i++) scrollIntoLine();
  await expect.poll(() => first.component.line()).toBeGreaterThan(150);
  const captured = first.component.capture();
  expect(Math.abs(captured.line - 200)).toBeLessThanOrEqual(2);
  expect(captured.offset).toBeGreaterThan(0);
  expect(captured).toMatchObject({
    selection: { start: 210, end: 215 },
    search: { search: "line250" },
    panel: "search",
  });
  await first.unmount();

  // A hidden tab has no layout: the state waits, and capture returns it.
  const second = await render(Harness, {
    text,
    restore: captured,
    hidden: true,
  });
  expect(second.component.capture()).toEqual(captured);
  await second.rerender({ hidden: false });
  await expect.poll(() => second.component.line()).toBe(captured.line);
  const restored = second.component.capture();
  expect(restored).toMatchObject({ ...captured, offset: expect.any(Number) });
  expect(restored.offset).toBeCloseTo(captured.offset, 1);
  await expect
    .element(page.getByRole("textbox", { name: "查找" }))
    .toHaveValue("line250");
  expect(document.activeElement?.closest("[data-code-search]")).toBeFalsy();
  await expect
    .element(page.getByLabelText("Selected reference"))
    .toMatchTextContent(new RegExp("210-215"));
  await second.unmount();

  // Restoring into a visible reader applies at once.
  const third = await render(Harness, { text, restore: restored });
  await expect.poll(() => third.component.line()).toBe(restored.line);
  expect(third.component.capture()).toMatchObject({
    line: restored.line,
    selection: restored.selection,
    search: restored.search,
    panel: "search",
  });
});
