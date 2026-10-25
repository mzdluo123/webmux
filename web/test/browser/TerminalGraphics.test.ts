import { expect, test } from "vitest";
import {
  host,
  installTerminalFixture,
  openTerminal,
  type FixtureSocket,
} from "./terminal-fixture";

installTerminalFixture();

const pixels = "/wAA/wD/AP8AAP///////w==";

function canvases() {
  return Array.from(
    host().querySelectorAll<HTMLCanvasElement>(
      ".xterm-graphics-layer .xterm-graphics-image",
    ),
  );
}

function replies(socket: FixtureSocket) {
  return socket.sent
    .filter((frame) => frame[0] === 6)
    .map((frame) => new TextDecoder().decode(frame.subarray(1)))
    .join("");
}

test("a kitty image displays above the grid and is acknowledged", async () => {
  const { socket } = await openTerminal();
  socket.receive(0, `\x1b_Ga=T,f=32,s=2,v=2,i=1;${pixels}\x1b\\`);
  await expect.poll(() => canvases().length).toBe(1);
  const canvas = canvases()[0];
  expect(canvas.style.width).not.toBe("");
  expect(canvas.style.height).not.toBe("");
  await expect.poll(() => replies(socket)).toContain("\x1b_Gi=1;OK\x1b\\");
});

test("chunks split across socket frames display one image", async () => {
  const { socket } = await openTerminal();
  socket.receive(
    0,
    `\x1b_Ga=T,f=32,s=2,v=2,i=3,m=1;${pixels.slice(0, 8)}\x1b\\`,
  );
  expect(canvases()).toHaveLength(0);
  socket.receive(0, `\x1b_Gm=0;${pixels.slice(8)}\x1b\\`);
  await expect.poll(() => canvases().length).toBe(1);
  await expect.poll(() => replies(socket).split("i=3;OK").length - 1).toBe(1);
});

test("deleting and replaying a session clear kitty placements", async () => {
  const { socket } = await openTerminal();
  socket.receive(0, `\x1b_Ga=T,f=32,s=2,v=2,i=1;${pixels}\x1b\\`);
  await expect.poll(() => canvases().length).toBe(1);
  socket.receive(0, "\x1b_Ga=d,d=i,i=1\x1b\\");
  await expect.poll(() => canvases().length).toBe(0);
  socket.receive(0, `\x1b_Ga=T,f=32,s=2,v=2,i=2;${pixels}\x1b\\`);
  await expect.poll(() => canvases().length).toBe(1);
  socket.receive(1, "replayed");
  await expect.poll(() => canvases().length).toBe(0);
});

test("the graphics query is answered without an image", async () => {
  const { socket } = await openTerminal();
  socket.receive(0, "\x1b_Ga=q,i=7,f=24,s=1,v=1;AAAA\x1b\\");
  await expect.poll(() => replies(socket)).toContain("\x1b_Gi=7;OK\x1b\\");
  expect(canvases()).toHaveLength(0);
});
