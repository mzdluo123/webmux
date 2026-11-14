import { expect, test, vi } from "vite-plus/test";
import type { ControlState } from "../../src/contracts/schemas";
import type { TerminalController } from "../../src/domains/terminal/terminal-search";
import {
  host,
  installTerminalFixture,
  openTerminalApi,
  sockets,
  inputText,
} from "./terminal-fixture";

installTerminalFixture();

function text(api: TerminalController) {
  const buffer = api.emulator!.buffer.active;
  return Array.from({ length: buffer.length }, (_, row) =>
    buffer.getLine(row)?.translateToString(true),
  ).join("\n");
}

test("immersive presentation keeps one socket/model and ignores DOM fitting until exit", async () => {
  const { api, socket } = await openTerminalApi();
  const model = api.emulator!;
  const original = { cols: model.cols, rows: model.rows };
  api.setImmersive(true);
  const count = socket.sent.filter((frame) => frame[0] === 1).length;
  host().style.width = "700px";
  host().style.height = "300px";
  api.fit();
  socket.receive(0, "\r\nimmersive-live-output\r\n");
  await expect.poll(() => text(api)).toContain("immersive-live-output");
  expect({ cols: model.cols, rows: model.rows }).toEqual(original);
  expect(socket.sent.filter((frame) => frame[0] === 1)).toHaveLength(count);
  expect(api.emulator).toBe(model);
  expect(sockets).toEqual([socket]);
  expect(socket.closeCount).toBe(0);
  expect(api.resizeGrid(96, 32)).toBe(true);
  expect(socket.sent.filter((frame) => frame[0] === 1)).toHaveLength(count + 1);
  expect(api.resizeGrid(96, 32)).toBe(true);
  expect(socket.sent.filter((frame) => frame[0] === 1)).toHaveLength(count + 1);
  socket.grantControl(96, 32);
  expect(socket.sent.filter((frame) => frame[0] === 1)).toHaveLength(count + 1);
  expect(api.resizeGrid(Number.NaN, 10)).toBe(false);
  api.setImmersive(false);
  expect(model.cols).not.toBe(96);
  expect(api.emulator).toBe(model);
  expect(sockets).toEqual([socket]);
});

test("hidden XR continues parsing live output and snapshots without losing input readiness", async () => {
  const { api, socket } = await openTerminalApi();
  api.setImmersive(true);
  vi.spyOn(document, "hidden", "get").mockReturnValue(true);
  socket.receive(0, "\r\nhidden-xr-tail\r\n");
  await expect.poll(() => text(api)).toContain("hidden-xr-tail");
  socket.receive(1, "hidden-xr-snapshot\r\n");
  await expect.poll(() => text(api)).toContain("hidden-xr-snapshot");
  await expect.poll(() => api.ready).toBe(true);
  expect(api.send("hello\r")).toBe(true);
  expect(inputText(socket)).toBe("hello\r");
  expect(sockets).toEqual([socket]);
});

test("history position survives output and observers cannot resize or claim control", async () => {
  const { api, socket } = await openTerminalApi();
  api.setImmersive(true);
  socket.receive(
    0,
    Array.from({ length: 180 }, (_, i) => `history-${i}\r\n`).join(""),
  );
  await expect
    .poll(() => api.emulator!.buffer.active.baseY)
    .toBeGreaterThan(100);
  api.scrollLines(-45);
  const position = api.emulator!.buffer.active.viewportY;
  expect(api.atBottom).toBe(false);
  socket.receive(0, "new-tail\r\n");
  await expect.poll(() => text(api)).toContain("new-tail");
  expect(api.emulator!.buffer.active.viewportY).toBe(position);
  api.scrollToBottom();
  expect(api.atBottom).toBe(true);
  const frames = socket.sent.length;
  const observerControl = {
    controller: {
      clientId: "other",
      label: "Other",
      since: new Date().toISOString(),
    },
    cols: 90,
    rows: 30,
    colorSchemeReports: false,
  } satisfies ControlState;
  socket.receive(6, JSON.stringify(observerControl));
  expect(api.controlling).toBe(false);
  expect(api.control).toEqual(observerControl);
  expect(api.emulator!.cols).toBe(90);
  expect(api.emulator!.rows).toBe(30);
  host().style.width = "700px";
  host().style.height = "300px";
  api.fit();
  expect(api.emulator!.cols).toBe(90);
  expect(api.emulator!.rows).toBe(30);
  expect(api.resizeGrid(100, 40)).toBe(false);
  expect(socket.sent).toHaveLength(frames);
  expect(socket.sent.some((frame) => frame[0] === 5)).toBe(false);
  expect(socket.closeCount).toBe(0);
});

test("graphics downgrade follows Kitty lifecycle without replacing the 2D graphics layer", async () => {
  const { api, socket } = await openTerminalApi();
  api.setImmersive(true);
  socket.receive(
    0,
    "\x1b_Ga=T,f=32,s=2,v=2,i=1;/wAA/wD/AP8AAP///////w==\x1b\\",
  );
  await expect.poll(() => api.graphicsUnsupported).toBe(true);
  const layer = host().querySelector(".xterm-graphics-layer");
  const image = layer?.querySelector("canvas");
  expect(image).toBeTruthy();
  socket.receive(0, "\r\ntext-still-works\r\n");
  await expect.poll(() => text(api)).toContain("text-still-works");
  api.setImmersive(false);
  expect(host().querySelector(".xterm-graphics-layer")).toBe(layer);
  expect(layer?.querySelector("canvas")).toBe(image);
  socket.receive(0, "\x1b_Ga=d,d=i,i=1\x1b\\");
  await expect.poll(() => api.graphicsUnsupported).toBe(false);
});

test("draft encoding consumes the same one-shot modifiers without delivering input", async () => {
  const { api, socket } = await openTerminalApi();
  api.toggleModifier("ctrl");
  expect(api.consumeInput("c")).toBe("\x03");
  expect(api.modifiers.ctrl).toBe(false);
  expect(inputText(socket)).toBe("");
  api.lockModifier("shift");
  expect(api.consumeInput("\t")).toBe("\x1b[Z");
  expect(api.modifiers.shift).toBe(true);
  expect(api.shiftLocked).toBe(true);
  api.toggleModifier("shift");
  expect(api.send("c")).toBe(true);
  expect(inputText(socket)).toBe("c");
});
