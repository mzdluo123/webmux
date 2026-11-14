import { afterEach, expect, test, vi } from "vitest";
import type { Terminal } from "@xterm/xterm";
import {
  createXrTerminalCanvas,
  type XrTerminalCanvas,
} from "../../src/hooks/terminal/xr-terminal-canvas";
import { installTerminalFixture, openTerminalApi } from "./terminal-fixture";

installTerminalFixture();
const views: XrTerminalCanvas[] = [];
afterEach(() => {
  for (const view of views.splice(0)) view.dispose();
});
async function write(term: Terminal, value: string) {
  const { promise, resolve } = Promise.withResolvers<void>();
  term.write(value, resolve);
  await promise;
}
async function setup() {
  const { api } = await openTerminalApi();
  api.setImmersive(true);
  api.resizeGrid(40, 12);
  const term = api.emulator!;
  term.options.cursorBlink = false;
  term.options.theme = {
    background: "#000000",
    foreground: "#ffffff",
    red: "#ff0000",
    brightRed: "#ff0000",
  };
  await write(term, "\x1b[2J\x1b[H");
  const view = createXrTerminalCanvas(term);
  views.push(view);
  return { term, view, api };
}

test("public parsed cells render wide/combining characters, colors and basic attributes", async () => {
  const { term, view } = await setup();
  await write(
    term,
    "\x1b[1;3;4;31;44m中e\u0301\x1b[0m\r\n\x1b[38;2;0;255;0mgreen",
  );
  const ctx = view.canvas.getContext("2d")!;
  const drawn: {
    text: string;
    color: string | CanvasGradient | CanvasPattern;
    font: string;
  }[] = [];
  const original = ctx.fillText.bind(ctx);
  vi.spyOn(ctx, "fillText").mockImplementation((text, x, y, maxWidth) => {
    drawn.push({ text, color: ctx.fillStyle, font: ctx.font });
    original(text, x, y, maxWidth);
  });
  expect(view.update()).toBe(true);
  expect(drawn.filter((cell) => cell.text === "中")).toHaveLength(1);
  expect(drawn.some((cell) => cell.text === "e\u0301")).toBe(true);
  const wide = drawn.find((cell) => cell.text === "中")!;
  expect(wide.color).toBe("#ff0000");
  expect(drawn.find((cell) => cell.text === "g")?.color).toBe("#00ff00");
  expect(view.update()).toBe(false);
  const background = ctx.getImageData(1, 1, 1, 1).data;
  expect(background[2]).toBeGreaterThan(0);
});

test("dirty output redraws only changed visible rows, including cursor and alternate buffers", async () => {
  const { term, view } = await setup();
  await write(term, "first\r\nsecond");
  view.update();
  const draw = vi.spyOn(view.canvas.getContext("2d")!, "fillText");
  await write(term, "\x1b[2;1Hchanged");
  expect(view.update()).toBe(true);
  expect(draw.mock.calls.some(([value]) => value === "f")).toBe(false);
  expect(draw.mock.calls.some(([value]) => value === "c")).toBe(true);
  draw.mockClear();
  await write(term, "\x1b[1;1H");
  expect(view.update()).toBe(true);
  expect(view.update()).toBe(false);
  await write(term, "\x1b[?1049halternate");
  expect(view.update()).toBe(true);
  draw.mockClear();
  await write(term, "\x1b[?1049l");
  expect(view.update()).toBe(true);
  expect(draw.mock.calls.some(([value]) => value === "f")).toBe(true);
  await write(term, "\x1b[?25l");
  expect(view.update()).toBe(true);
  expect(view.update()).toBe(false);
});

test("scroll/theme/resize invalidate the texture, and long output stays bounded", async () => {
  const { term, view, api } = await setup();
  await write(
    term,
    Array.from({ length: 300 }, (_, i) => `line-${i}\r\n`).join(""),
  );
  expect(view.update()).toBe(true);
  const initial = [view.canvas.width, view.canvas.height];
  api.scrollLines(-50);
  expect(view.update()).toBe(true);
  expect(view.update()).toBe(false);
  await write(term, "new-tail\r\n");
  // Output outside the displayed history does not upload a redundant texture.
  expect(view.update()).toBe(false);
  expect([view.canvas.width, view.canvas.height]).toEqual(initial);
  term.options.theme = { background: "#112233", foreground: "#ffffff" };
  expect(view.update()).toBe(true);
  expect(api.resizeGrid(1000, 1000)).toBe(true);
  expect(view.update()).toBe(true);
  expect(view.canvas.width).toBeLessThanOrEqual(4096);
  expect(view.canvas.height).toBeLessThanOrEqual(4096);
  expect(view.canvas.width * view.canvas.height).toBeLessThanOrEqual(
    4 * 1024 * 1024,
  );
  view.dispose();
  expect(view.canvas.width).toBe(0);
  expect(view.canvas.height).toBe(0);
  expect(view.update()).toBe(false);
});

test("a cursor hidden before entering XR stays hidden using public parser tracking", async () => {
  const { term, view } = await setup();
  view.dispose();
  await write(term, "\x1b[?25l");
  const replacement = createXrTerminalCanvas(term);
  views.push(replacement);
  replacement.update();
  const ctx = replacement.canvas.getContext("2d")!;
  const hasForeground = () => {
    const pixels = ctx.getImageData(
      0,
      0,
      replacement.canvas.width,
      replacement.canvas.height,
    ).data;
    return pixels.some((value, index) => index % 4 !== 3 && value > 0);
  };
  expect(hasForeground()).toBe(false);
  await write(term, "\x1b[?25h");
  expect(replacement.update()).toBe(true);
  expect(hasForeground()).toBe(true);
});
