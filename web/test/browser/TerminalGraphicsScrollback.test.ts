import { expect, test, vi } from "vite-plus/test";
import {
  expectDomText,
  expectGlyphs,
  geometry,
  host,
  installTerminalFixture,
  observeGlyphPixels,
  openTerminalApi,
  prompt as fixturePrompt,
  setDpr,
  setMeasurement,
  swipe,
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

test("multi-row bottom placement scrolls the buffer and returns intact when dragging the scrollbar", async () => {
  observeGlyphPixels(false);
  const { socket, api } = await openTerminalApi();
  await expectGlyphs();
  const layer = host().querySelector<HTMLElement>(".xterm-graphics-layer")!;
  expect(getComputedStyle(layer).zIndex).toBe("3");

  // Jump to the bottom row and emit an 8-row C=0 image, then trailing output.
  const rows = api.emulator!.rows;
  socket.receive(
    0,
    `\x1b[999;1Hbottom-start\r\n` +
      `\x1b_Ga=T,f=32,s=2,v=2,i=101,r=8;${pixels}\x1b\\` +
      `\r\nafter-image-marker` +
      Array.from(
        { length: rows + 20 },
        (_, idx) => `\r\ntail-line-${idx + 1}`,
      ).join(""),
  );
  await expect
    .poll(() => {
      const buffer = api.emulator!.buffer.active;
      for (let line = 0; line < buffer.length; line++) {
        if (
          buffer
            .getLine(line)
            ?.translateToString(true)
            .includes(`tail-line-${rows + 20}`)
        )
          return true;
      }
      return false;
    })
    .toBe(true);
  await expect.poll(() => canvases().length).toBe(1);
  const image = canvases()[0]!;
  await expect.poll(() => image.style.display).toBe("none");

  // Drag xterm's vertical scrollbar slider upward to bring scrollback into view.
  const slider = host().querySelector<HTMLElement>(
    ".xterm-scrollable-element > .scrollbar.vertical > .slider",
  )!;
  const bar = host().querySelector<HTMLElement>(
    ".xterm-scrollable-element > .scrollbar.vertical",
  )!;
  await expect
    .poll(() => slider.getBoundingClientRect().height)
    .toBeGreaterThan(0);
  const sliderBox = slider.getBoundingClientRect();
  const barBox = bar.getBoundingClientRect();
  const centerX = sliderBox.left + sliderBox.width / 2;
  const startY = sliderBox.top + sliderBox.height / 2;
  const targetY = barBox.top + barBox.height * 0.45;
  vi.spyOn(slider, "setPointerCapture").mockImplementation(() => {});
  vi.spyOn(slider, "releasePointerCapture").mockImplementation(() => {});
  slider.dispatchEvent(
    new PointerEvent("pointerdown", {
      bubbles: true,
      cancelable: true,
      pointerType: "mouse",
      pointerId: 1,
      button: 0,
      buttons: 1,
      clientX: centerX,
      clientY: startY,
    }),
  );
  slider.dispatchEvent(
    new PointerEvent("pointermove", {
      bubbles: true,
      cancelable: true,
      pointerType: "mouse",
      pointerId: 1,
      button: 0,
      buttons: 1,
      clientX: centerX,
      clientY: targetY,
    }),
  );
  slider.dispatchEvent(
    new PointerEvent("pointerup", {
      bubbles: true,
      cancelable: true,
      pointerType: "mouse",
      pointerId: 1,
      button: 0,
      buttons: 0,
      clientX: centerX,
      clientY: targetY,
    }),
  );

  await expect.poll(() => image.style.display).not.toBe("none");
  const { row, bounds } = geometry(socket);
  const imageBox = image.getBoundingClientRect();
  expect(imageBox.height).toBeCloseTo(row * 8, 0);
  expect(imageBox.top).toBeGreaterThanOrEqual(bounds.top - 1);
  expect(imageBox.bottom).toBeLessThanOrEqual(bounds.bottom + 1);
});

test("pending-wrap cursor clamps placement inside the right margin and allocates full image rows", async () => {
  observeGlyphPixels(true);
  const { socket, api } = await openTerminalApi();
  const cols = api.emulator!.cols;
  socket.receive(
    0,
    `\x1b[999;1H${"W".repeat(cols)}` +
      `\x1b_Ga=T,f=32,s=2,v=2,i=102,c=1,r=6;${pixels}\x1b\\` +
      `\r\nbelow-image`,
  );
  await expectDomText("below-image");
  await expect.poll(() => canvases().length).toBe(1);
  const image = canvases()[0]!;
  await expect.poll(() => image.style.display).not.toBe("none");
  const { bounds, cell, row } = geometry(socket);
  const imageBox = image.getBoundingClientRect();
  expect(imageBox.left).toBeCloseTo(bounds.left + (cols - 1) * cell, 0);
  expect(imageBox.right).toBeLessThanOrEqual(bounds.right + 1);
  // 6 image rows + 1 "below-image" row stay above the bottom of the screen.
  expect(imageBox.bottom).toBeLessThanOrEqual(bounds.bottom - row + 1);
});

test("scrollback image stays visible while viewing history during active-screen clears and deletes", async () => {
  observeGlyphPixels(true);
  const { socket, api } = await openTerminalApi();
  socket.receive(
    0,
    `scrollback-anchor\r\n\x1b_Ga=T,f=32,s=2,v=2,i=103,p=1,c=6,r=4;${pixels}\x1b\\` +
      Array.from({ length: 50 }, (_, idx) => `\r\nfill-${idx + 1}`).join(""),
  );
  await expectDomText("fill-50");
  await expect.poll(() => canvases().length).toBe(1);
  const historyCanvas = canvases()[0]!;
  await expect.poll(() => historyCanvas.style.display).toBe("none");

  // Scroll back into history via touch swipe until the image is in view.
  await swipe(socket, 25);
  await swipe(socket, 25);
  await expectDomText("scrollback-anchor");
  await expect.poll(() => historyCanvas.style.display).not.toBe("none");

  // Place an image on the active screen below and run active-screen clears/deletes.
  socket.receive(
    0,
    `\x1b[H\x1b_Ga=T,f=32,s=2,v=2,i=104,p=1,c=4,r=2,C=1;${pixels}\x1b\\`,
  );
  await expect.poll(() => canvases().length).toBe(2);
  socket.receive(
    0,
    "\x1b_Ga=d,d=A,q=2\x1b\\" +
      "\x1b_Ga=d,d=X,x=1,q=2\x1b\\" +
      "\x1b_Ga=d,d=Y,y=1,q=2\x1b\\" +
      "\x1b_Ga=d,d=Z,z=0,q=2\x1b\\" +
      "\x1b[2Jactive-cleared",
  );
  await expect.poll(() => canvases().length).toBe(1);
  expect(canvases()).toEqual([historyCanvas]);
  expect(historyCanvas.style.display).not.toBe("none");
  expect(historyCanvas.width).toBeGreaterThan(0);

  // Stored image 103 was not evicted by d=A because its scrollback placement survived.
  api.scrollToBottom();
  await expectDomText("active-cleared");
  socket.receive(0, "\x1b_Ga=p,i=103,p=2,c=4,r=2,C=1\x1b\\reused-after-clear");
  await expectDomText("reused-after-clear");
  await expect.poll(() => canvases().length).toBe(2);
});

test("scrollback image on wrapped lines survives narrowing and widening reflow and renderer fallback", async () => {
  observeGlyphPixels(false);
  const { socket, api } = await openTerminalApi();
  await expectGlyphs();
  const initialCols = api.emulator!.cols;
  socket.receive(
    0,
    `${"R".repeat(initialCols + 14)}` +
      `\x1b_Ga=T,f=32,s=2,v=2,i=105,c=4,r=3,C=1;${pixels}\x1b\\` +
      `\r\nwrapped-anchor` +
      Array.from({ length: 45 }, (_, idx) => `\r\nreflow-tail-${idx + 1}`).join(
        "",
      ),
  );
  await expect.poll(() => canvases().length).toBe(1);
  const image = canvases()[0]!;
  await expect.poll(() => image.style.display).toBe("none");

  // Narrow then widen the grid to trigger _reflowSmaller and _reflowLarger (onDelete).
  expect(api.resizeGrid(Math.max(20, initialCols - 16), 18)).toBe(true);
  expect(api.resizeGrid(initialCols + 24, 24)).toBe(true);

  // Trigger dynamic WebGL -> DOM fallback via inconsistent DPR measurement.
  setMeasurement("css-pixels");
  setDpr(2);
  await expectDomText("reflow-tail-45");

  api.findNext(fixturePrompt.slice(0, 7), {
    caseSensitive: true,
    wholeWord: false,
    regex: false,
  });
  api.findNext("wrapped-anchor", {
    caseSensitive: true,
    wholeWord: false,
    regex: false,
  });
  await expectDomText("wrapped-anchor");
  await expect.poll(() => image.style.display).not.toBe("none");
  expect(canvases()).toEqual([image]);
  expect(image.width).toBeGreaterThan(0);
});

test("multi-row placements survive partial top trimming in alternate and normal buffers until all rows leave", async () => {
  observeGlyphPixels(true);
  const { socket, api } = await openTerminalApi();
  const rows = api.emulator!.rows;

  // Alternate screen has zero scrollback, so C=0 IND scrolling trims top rows immediately.
  socket.receive(
    0,
    `\x1b[?1049h\x1b[H` +
      `\x1b_Ga=T,f=32,s=2,v=2,i=106,r=${rows + 6};${pixels}\x1b\\` +
      `\ralt-tall-tail`,
  );
  await expectDomText("alt-tall-tail");
  await expect.poll(() => canvases().length).toBe(1);
  const altImage = canvases()[0]!;
  await expect.poll(() => altImage.style.display).not.toBe("none");
  const { bounds, row } = geometry(socket);
  expect(altImage.getBoundingClientRect().top).toBeCloseTo(
    bounds.top - row * 6,
    0,
  );
  expect(altImage.getBoundingClientRect().bottom).toBeCloseTo(
    bounds.top + row * rows,
    0,
  );

  // Scrolling the remaining visible rows off the top of the alternate screen releases it.
  socket.receive(0, `${"\r\nalt-flush".repeat(rows)}alt-done`);
  await expectDomText("alt-done");
  await expect.poll(() => canvases().length).toBe(0);

  // Return to normal screen, straddle baseY with a 10-row image, and clear scrollback (CSI 3 J).
  socket.receive(
    0,
    `\x1b[?1049l\x1b[999;1H` +
      `\x1b_Ga=T,f=32,s=2,v=2,i=107,r=10;${pixels}\x1b\\` +
      `${"\r\npush-straddle".repeat(rows - 6)}\x1b[3Jstraddle-kept`,
  );
  await expectDomText("straddle-kept");
  await expect.poll(() => canvases().length).toBe(1);
  const straddleImage = canvases()[0]!;
  await expect.poll(() => straddleImage.style.display).not.toBe("none");
  expect(straddleImage.getBoundingClientRect().bottom).toBeGreaterThan(
    bounds.top + row * 4,
  );

  // Once the remaining visible rows scroll into history, CSI 3 J trims the placement cleanly.
  socket.receive(0, `${"\r\npush-out".repeat(10)}\x1b[3Jstraddle-trimmed`);
  await expectDomText("straddle-trimmed");
  await expect.poll(() => canvases().length).toBe(0);
});
