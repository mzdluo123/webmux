// Send-queue backpressure, PTY geometry, control, device replies, session
// frames, submissions and invalid server frames.
import { expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { render } from "vitest-browser-svelte/pure";
import type { SubmissionProgress } from "../../src/contracts/schemas";
import { resetBrowserChrome } from "../../src/platform/browser-chrome";
import { startP2P, stopP2P } from "../../src/platform/p2p.svelte";
import { p2pState } from "../../src/platform/p2p-state.svelte";
import Harness from "./TerminalHarness.svelte";
import { initializeLocale, locale } from "../../src/lib/locale";
import { m } from "$lib/paraglide/messages.js";
import {
  blurAndTrackFocus,
  expectDomText,
  FixtureSocket,
  geometry,
  host,
  hostObserver,
  inputText,
  installTerminalFixture,
  observeGlyphPixels,
  openTerminal,
  openTerminalApi,
  reconnected,
  snapshot,
  sockets,
} from "./terminal-fixture";

installTerminalFixture();

test("saturated input reports rejection and clears it after a successful send", async () => {
  observeGlyphPixels(true);
  const { socket } = await openTerminal();
  socket.bufferedAmount = 1024 * 1024;
  await page.getByRole("button", { name: "Focus terminal input" }).click();
  await userEvent.keyboard("x");
  await expect
    .element(page.getByRole("status", { name: "Terminal error" }))
    .toHaveTextContent("终端输入未发送：连接不可用或发送队列已满，请重试。");
  expect(inputText(socket)).toBe("");
  socket.bufferedAmount = 0;
  await userEvent.keyboard("y");
  await expect.poll(() => inputText(socket)).toBe("y");
  await expect
    .element(page.getByRole("status", { name: "Terminal error" }))
    .toHaveTextContent("");
});

test("visible input uncertainty follows locale changes without replay or clearing on new input", async () => {
  const previousLocale = locale.preference;
  initializeLocale();
  locale.setPreference("zh-CN");
  try {
    observeGlyphPixels(true);
    const { socket: first } = await openTerminal();
    await page.getByRole("button", { name: "Focus terminal input" }).click();
    await userEvent.keyboard("x");
    await expect.poll(() => inputText(first)).toBe("x");
    first.drop();
    const banner = page.getByRole("status", { name: "Terminal error" });
    await expect
      .element(banner)
      .toHaveTextContent(m.relay_sent_input_uncertain());

    locale.setPreference("en");
    await expect
      .element(banner)
      .toHaveTextContent(m.relay_sent_input_uncertain());
    const second = await reconnected(2);
    second.open();
    second.receive(1, snapshot);
    second.grantControl();
    await expect
      .element(page.getByRole("status", { name: "Terminal restoration" }))
      .toHaveTextContent("ready");
    await page.getByRole("button", { name: "Focus terminal input" }).click();
    await userEvent.keyboard("y");
    expect(inputText(first)).toBe("x");
    expect(inputText(second)).toBe("y");
    await expect
      .element(banner)
      .toHaveTextContent(m.relay_sent_input_uncertain());

    locale.setPreference("zh-CN");
    await expect
      .element(banner)
      .toHaveTextContent(m.relay_sent_input_uncertain());
  } finally {
    locale.setPreference(previousLocale);
  }
});

test("initial terminal geometry is delivered after the send queue drains", async () => {
  observeGlyphPixels(true);
  await render(Harness);
  await expect.poll(() => sockets.length).toBe(1);
  const socket = sockets[0];
  socket.bufferedAmount = 1024 * 1024;
  socket.open();
  socket.receive(1, snapshot);
  // Only the controlling client resizes the PTY.
  socket.grantControl();
  await expect
    .element(page.getByRole("status", { name: "Terminal restoration" }))
    .toHaveTextContent("ready");
  expect(socket.sent).toEqual([]);

  socket.bufferedAmount = 0;
  await expect.poll(() => socket.sent.length).toBe(1);
  expect(socket.sent[0][0]).toBe(1);
  const size = JSON.parse(new TextDecoder().decode(socket.sent[0].subarray(1)));
  expect(size.cols).toBeGreaterThan(0);
  expect(size.rows).toBeGreaterThan(0);
  expect(sockets).toEqual([socket]);
  expect(socket.closeCount).toBe(0);
});

test("terminal resize and resync recover on the same socket after saturation", async () => {
  observeGlyphPixels(true);
  const { socket } = await openTerminal();
  const original = geometry(socket);
  socket.sent = [];
  socket.bufferedAmount = 1024 * 1024;
  const bounds = () =>
    host().querySelector(".xterm-screen")!.getBoundingClientRect();
  host().style.width = "600px";
  await expect
    .poll(() => bounds().width)
    .toBeGreaterThan(original.bounds.width);
  const intermediateWidth = bounds().width;
  host().style.width = "800px";
  host().style.height = "650px";
  await expect.poll(() => bounds().width).toBeGreaterThan(intermediateWidth);
  await expect.poll(() => bounds().height).toBeLessThan(original.bounds.height);
  const expected = {
    cols: Math.round(bounds().width / original.cell),
    rows: Math.round(bounds().height / original.row),
  };
  await page
    .getByRole("button", { name: "Resync terminal", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Resync terminal", exact: true })
    .click();
  expect(socket.sent).toEqual([]);

  socket.bufferedAmount = 0;
  await expect.poll(() => socket.sent.length).toBe(2);
  const resize = socket.sent.find((frame) => frame[0] === 1)!;
  expect(JSON.parse(new TextDecoder().decode(resize.subarray(1)))).toEqual(
    expected,
  );
  expect(socket.sent.filter((frame) => frame[0] === 2)).toEqual([
    new Uint8Array([2]),
  ]);
  expect(sockets).toEqual([socket]);
  expect(socket.closeCount).toBe(0);
});

test("locking the terminal size keeps the PTY grid across host resizes", async () => {
  observeGlyphPixels(true);
  let locked = false;
  const { socket, api } = await openTerminalApi({
    sizeLocked: () => locked,
  });
  const sizeFrames = () => socket.sent.filter((frame) => frame[0] === 1).length;
  const screen = () =>
    host().querySelector(".xterm-screen")!.getBoundingClientRect();
  const xtermScrollbar = () =>
    host().querySelector<HTMLElement>(
      ".xterm-scrollable-element > .scrollbar.vertical",
    )!;
  const initial = geometry(socket);
  expect(initial.bounds.width).toBeLessThanOrEqual(host().clientWidth);
  const fitted = sizeFrames();
  expect(fitted).toBeGreaterThan(0);
  // An unlocked terminal scrolls its buffer through xterm's own scrollbar.
  expect(getComputedStyle(xtermScrollbar()).display).not.toBe("none");

  // The menu toggle keeps the current grid instead of refitting it.
  locked = true;
  host().classList.add("size-locked");
  api.fit();
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(sizeFrames()).toBe(fitted);
  // The locked host pans the grid, so xterm's grid-edge scrollbar would float
  // inside the visible area or duplicate the host one.
  expect(getComputedStyle(xtermScrollbar()).display).toBe("none");

  // A keyboard or browser-chrome shrink changes only the height; the
  // scrollable host stays anchored to the newest rows.
  host().style.height = "500px";
  await expect
    .poll(() => host().scrollHeight)
    .toBeGreaterThan(host().clientHeight);
  await expect
    .poll(() => host().scrollTop + host().clientHeight)
    .toBeGreaterThanOrEqual(host().scrollHeight - 1);
  expect(sizeFrames()).toBe(fitted);
  expect(screen().width).toBe(initial.bounds.width);
  expect(screen().height).toBe(initial.bounds.height);

  // Scrolling the host up reports history; the control returns to the newest
  // rows without resizing the grid.
  const position = page.getByRole("status", { name: "Terminal position" });
  host().scrollTop = 0;
  await expect.element(position).toHaveTextContent("history");
  await page.getByRole("button", { name: "Scroll to latest" }).click();
  await expect.element(position).toHaveTextContent("latest");
  expect(sizeFrames()).toBe(fitted);

  // A deliberate width change (rotation, window, code panel) also keeps the
  // grid; the host scrolls horizontally instead of refitting the PTY.
  host().style.width = "300px";
  await expect
    .poll(() => host().scrollWidth)
    .toBeGreaterThan(host().clientWidth);
  expect(sizeFrames()).toBe(fitted);
  expect(screen().width).toBe(initial.bounds.width);

  // Unlocking adopts the host again and restores xterm's scrollbar.
  locked = false;
  host().classList.remove("size-locked");
  api.fit();
  await expect.poll(sizeFrames).toBeGreaterThan(fitted);
  await expect
    .poll(() => screen().width)
    .toBeLessThanOrEqual(host().clientWidth);
  expect(getComputedStyle(xtermScrollbar()).display).not.toBe("none");
});

test("a locked host keeps following output that arrives during a resize", async () => {
  observeGlyphPixels(true);
  const { socket, api } = await openTerminalApi({
    sizeLocked: () => true,
  });
  host().classList.add("size-locked");
  api.fit();
  // Drop the debounced layout so output can update the position from the
  // smaller box before the next fit, as it can while the real timer is pending.
  hostObserver().disconnect();
  // A layout the observer already scheduled (100 ms debounce) would fit and
  // re-anchor after the resize below. The timer is internal and xterm renders
  // on real timers, so instead of faking time, outlast it: timers run in
  // deadline order, so it has run before the box shrinks.
  await new Promise((resolve) => setTimeout(resolve, 150));
  host().style.height = "400px";
  await expect
    .poll(() => host().scrollHeight)
    .toBeGreaterThan(host().clientHeight);
  socket.receive(0, "during resize\r\n");
  await expectDomText("during resize");
  const position = page.getByRole("status", { name: "Terminal position" });
  await expect.element(position).toHaveTextContent("history");
  // The debounced layout calls this same fit, now after the position changed.
  api.fit();
  await expect
    .poll(() => host().scrollTop + host().clientHeight)
    .toBeGreaterThanOrEqual(host().scrollHeight - 1);
  await expect.element(position).toHaveTextContent("latest");
});

test("a locked grid only rescales the font, keeping the PTY rows and columns", async () => {
  observeGlyphPixels(true);
  const { socket, api } = await openTerminalApi({
    sizeLocked: () => true,
  });
  host().classList.add("size-locked");
  const sizeFrames = () => socket.sent.filter((frame) => frame[0] === 1).length;
  const screen = () =>
    host().querySelector(".xterm-screen")!.getBoundingClientRect();
  const fitted = sizeFrames();
  expect(fitted).toBeGreaterThan(0);
  const before = screen();

  api.setFontSize(20);
  await expect.poll(() => screen().width).toBeGreaterThan(before.width);
  expect(screen().height).toBeGreaterThan(before.height);
  expect(sizeFrames()).toBe(fitted);
});

test("a keyboard height hold keeps the grid for height changes only", async () => {
  observeGlyphPixels(true);
  let held = false;
  const { socket, api } = await openTerminalApi({
    heightHeld: () => held,
  });
  const sizeFrames = () => socket.sent.filter((frame) => frame[0] === 1).length;
  // ResizeObserver layout is debounced by 100 ms; the absence of a frame can
  // only be observed after that real delay.
  const settle = () => new Promise((resolve) => setTimeout(resolve, 250));
  const fitted = sizeFrames();
  const original = host().style.height;

  // The keyboard opens and closes: the host returns to its held size and
  // ending the hold sends nothing. The held grid stays anchored to the tail.
  held = true;
  host().classList.add("size-locked");
  host().style.height = "500px";
  await settle();
  expect(sizeFrames()).toBe(fitted);
  expect(host().scrollTop + host().clientHeight).toBeGreaterThanOrEqual(
    host().scrollHeight - 1,
  );
  host().style.height = original;
  await settle();
  held = false;
  host().classList.remove("size-locked");
  api.fit();
  expect(sizeFrames()).toBe(fitted);

  // Rotation during the hold still refits the columns.
  held = true;
  host().classList.add("size-locked");
  host().style.width = "300px";
  await expect.poll(sizeFrames).toBe(fitted + 1);
  // So does a font change.
  api.setFontSize(18);
  await expect.poll(sizeFrames).toBe(fitted + 2);

  // The draft pane grew while the keyboard was open: one refit on release.
  host().style.height = "500px";
  await settle();
  expect(sizeFrames()).toBe(fitted + 2);
  held = false;
  host().classList.remove("size-locked");
  api.fit();
  await expect.poll(sizeFrames).toBe(fitted + 3);
});

test("a persisted size lock still fits the grid once on mount", async () => {
  observeGlyphPixels(true);
  const { socket } = await openTerminalApi({
    sizeLocked: () => true,
  });
  const screen = host().querySelector(".xterm-screen")!.getBoundingClientRect();
  // The default 80-column grid would overflow the 400 px fixture host.
  expect(screen.width).toBeLessThanOrEqual(host().clientWidth);
  expect(socket.sent.filter((frame) => frame[0] === 1).length).toBeGreaterThan(
    0,
  );
});

test("a locked grid refits after a hidden host becomes measurable again", async () => {
  observeGlyphPixels(true);
  const { socket, api } = await openTerminalApi({
    sizeLocked: () => true,
  });
  const sizeFrames = () => socket.sent.filter((frame) => frame[0] === 1).length;
  const hostElement = host();
  const fitted = sizeFrames();
  expect(fitted).toBeGreaterThan(0);

  // A hidden host (the code pane covers the terminal) drops the font-driven
  // fit. Showing it again must refit instead of keeping the stale grid.
  hostElement.style.display = "none";
  api.setFontSize(18);
  await new Promise((resolve) => setTimeout(resolve, 50));
  hostElement.style.display = "";
  api.fit();
  await expect.poll(sizeFrames).toBeGreaterThan(fitted);
  await expect
    .poll(
      () =>
        hostElement.querySelector(".xterm-screen")!.getBoundingClientRect()
          .width,
    )
    .toBeLessThanOrEqual(hostElement.clientWidth);
});

test("an observer sizes to the controller's grid before parsing cursor-addressed history", async () => {
  observeGlyphPixels(true);
  await render(Harness);
  await expect.poll(() => sockets.length, { timeout: 5000 }).toBe(1);
  const socket = sockets[0];
  socket.open();
  // The server sends control before the replay: another client controls a
  // 120-column grid, far wider than this 400 px host fits.
  socket.receive(
    6,
    JSON.stringify({
      controller: {
        clientId: "another-client",
        label: "desk",
        since: new Date().toISOString(),
      },
      cols: 120,
      rows: 30,
      colorSchemeReports: false,
    }),
  );
  socket.receive(1, "\x1b[1;100Hfar-right");
  await expect
    .poll(() =>
      (host().querySelector(".xterm-rows > div")?.textContent ?? "")
        .replaceAll("\u00a0", " ")
        .trimEnd(),
    )
    .toMatch(/^ {99}far-right$/);
  expect(host().classList.contains("observing")).toBe(true);
  // The observer pans the controller's grid with the host; xterm's grid-edge
  // scrollbar would sit off the visible area, so it is hidden.
  const scrollbar = host().querySelector<HTMLElement>(
    ".xterm-scrollable-element > .scrollbar.vertical",
  )!;
  expect(getComputedStyle(scrollbar).display).toBe("none");
  // Observers never resize the PTY.
  expect(socket.sent.filter((frame) => frame[0] === 1)).toEqual([]);
});

test("an observer panning above the controller's newest rows is viewing history", async () => {
  observeGlyphPixels(true);
  await render(Harness);
  await expect.poll(() => sockets.length, { timeout: 5000 }).toBe(1);
  const socket = sockets[0];
  socket.open();
  // Sixty rows are taller than the 750 px host, so the observer pans it.
  socket.receive(
    6,
    JSON.stringify({
      controller: {
        clientId: "another-client",
        label: "desk",
        since: new Date().toISOString(),
      },
      cols: 40,
      rows: 60,
      colorSchemeReports: false,
    }),
  );
  socket.receive(1, "\x1b[60;1Hbottom-row");
  await expect.poll(() => host().classList.contains("observing")).toBe(true);
  await expect
    .poll(() => host().scrollHeight)
    .toBeGreaterThan(host().clientHeight);
  // Restoration ends by scrolling to the newest rows; pan only after it.
  await expect
    .element(page.getByRole("status", { name: "Terminal restoration" }))
    .toHaveTextContent("ready");
  const position = page.getByRole("status", { name: "Terminal position" });
  await expect.element(position).toHaveTextContent("latest");
  host().scrollTop = 0;
  await expect.element(position).toHaveTextContent("history");
  await page.getByRole("button", { name: "Scroll to latest" }).click();
  await expect.element(position).toHaveTextContent("latest");
  expect(host().scrollTop + host().clientHeight).toBeGreaterThanOrEqual(
    host().scrollHeight - 1,
  );
});

test("automatic replies to device queries are sent apart from user input, even when the bytes match a key", async () => {
  const { socket } = await openTerminal();
  const frames = (kind: number) =>
    socket.sent
      .filter((frame) => frame[0] === kind)
      .map((frame) => new TextDecoder().decode(frame.subarray(1)));
  // Primary device attributes and a cursor position report at row 1.
  socket.receive(0, "\x1b[H\x1b[c\x1b[6n");
  // oxlint-disable-next-line no-control-regex -- Match the terminal's device-attributes escape sequence.
  await expect.poll(() => frames(6).join("")).toMatch(/\x1b\[\?[\d;]+c/);
  await expect.poll(() => frames(6).join("")).toContain("\x1b[1;1R");
  expect(frames(0)).toEqual([]);
  // Shift+F3 produces the same shape as a cursor report, but it is a key.
  const textarea = host().querySelector("textarea")!;
  textarea.focus();
  textarea.dispatchEvent(
    new KeyboardEvent("keydown", {
      key: "F3",
      code: "F3",
      keyCode: 114,
      shiftKey: true,
      bubbles: true,
      cancelable: true,
    }),
  );
  await expect.poll(() => frames(0)).toEqual(["\x1b[1;2R"]);
});

test("color scheme reports follow control state without duplicating theme notifications", async () => {
  const root = document.documentElement;
  root.classList.remove("dark");
  const { socket } = await openTerminal();
  const reports = () =>
    socket.sent
      .filter((frame) => frame[0] === 6)
      .map((frame) => new TextDecoder().decode(frame.subarray(1)))
      .filter((reply) => reply.startsWith("\x1b[?997;"));
  // Without DEC mode 2031 a shell would read the report as typed text.
  root.classList.add("dark");
  // Enabling it announces the current scheme, so a program that queried
  // before this browser attached learns it now.
  socket.grantControl(80, 24, true);
  await expect.poll(reports).toEqual(["\x1b[?997;1n"]);
  // Theme changes are broadcast by the backend; this attachment reports its
  // current scheme on control changes and history queries without adding a
  // second client-side notification path.
  root.classList.remove("dark");
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(reports()).toEqual(["\x1b[?997;1n"]);
  // A query in replayed history was answered when it was live.
  socket.receive(1, "\x1b[?996n");
  socket.receive(0, "\x1b[?996n");
  await expect.poll(reports).toEqual(["\x1b[?997;1n", "\x1b[?997;2n"]);
  expect(socket.sent.filter((frame) => frame[0] === 0)).toEqual([]);
});

const generation = "0123456789abcdef";

function sessionInfo(title: string, status = "running") {
  const now = new Date().toISOString();
  return JSON.stringify({
    id: "abcdefgh",
    generation,
    title,
    cwd: "/tmp",
    cmd: ["bash"],
    status,
    exitCode: null,
    listeningPorts: [],
    state: "active",
    views: ["terminal"],
    view: "terminal",
    transition: null,
    agent: null,
    preview: "",
    createdAt: now,
    lastActiveAt: now,
    cols: 80,
    rows: 24,
    clients: 1,
    workspace: null,
    attention: null,
    reportedCwd: null,
  });
}

function framesOf(socket: FixtureSocket, kind: number) {
  return socket.sent
    .filter((frame) => frame[0] === kind)
    .map((frame) => new TextDecoder().decode(frame.subarray(1)));
}

test("submissions carry the run generation and follow acknowledgements across reconnects until settled", async () => {
  observeGlyphPixels(true);
  const title = document.title;
  const { socket: first, api } = await openTerminalApi();
  const reports: SubmissionProgress[] = [];
  const report = (state: SubmissionProgress) => reports.push(state);
  // Without a known run the server could not reject a stale submission.
  expect(api.submit("submission-1", "echo 1", true, report)).toBe(false);
  first.receive(2, sessionInfo("fixture title"));
  expect(api.info?.generation).toBe(generation);
  expect(document.title).toBe("▶ fixture title · WebMux");

  api.toggleModifier("ctrl");
  expect(api.submit("submission-1", "echo 1\nok", true, report)).toBe(true);
  expect(api.modifiers).toEqual({ ctrl: false, alt: false, shift: false });
  expect(reports).toEqual(["sending"]);
  // One JSON header line, then the paste-prepared body and the Enter key.
  const header = JSON.stringify({
    id: "submission-1",
    generation,
    bodyBytes: 9,
  });
  expect(framesOf(first, 3)).toEqual([`${header}\necho 1\rok\r`]);
  first.receive(5, JSON.stringify({ id: "submission-1", state: "received" }));
  expect(reports).toEqual(["sending", "received"]);

  first.drop();
  expect(reports).toEqual(["sending", "received", "unknown"]);
  // Every connection attempt reports it again until the server answers.
  const second = await reconnected(2);
  expect(reports).toEqual(["sending", "received", "unknown", "unknown"]);
  second.open();
  const pending = JSON.stringify({ ids: ["submission-1"] });
  expect(framesOf(second, 4)).toEqual([pending]);
  second.receive(5, JSON.stringify({ id: "submission-1", state: "written" }));
  expect(reports.at(-1)).toBe("written");
  const settled = reports.length;

  const followed: SubmissionProgress[] = [];
  api.track("submission-2", (state) => followed.push(state));
  expect(framesOf(second, 4)).toEqual([
    pending,
    JSON.stringify({ ids: ["submission-2"] }),
  ]);
  api.untrack("submission-2");
  second.drop();
  const third = await reconnected(3);
  third.open();
  expect(framesOf(third, 4)).toEqual([]);
  expect(followed).toEqual([]);
  expect(reports).toHaveLength(settled);
  expect(inputText(first) + inputText(second)).toBe("");
  resetBrowserChrome();
  document.title = title;
});

test("waiting, idle and exited statuses keep the tab static", async () => {
  const title = document.title;
  const { socket } = await openTerminalApi();
  for (const status of ["waiting", "idle", "exited"]) {
    socket.receive(2, sessionInfo("quiet", status));
    expect(document.title).toBe("quiet · WebMux");
    expect(
      document
        .querySelector<HTMLLinkElement>('link[rel="icon"]')!
        .getAttribute("href"),
    ).toBe("/icon.svg");
  }
  resetBrowserChrome();
  document.title = title;
});

test("a session exit clears a running tab marker", async () => {
  const title = document.title;
  const { socket } = await openTerminalApi();
  socket.receive(2, sessionInfo("exiting"));
  expect(document.title).toBe("▶ exiting · WebMux");
  socket.receive(2, sessionInfo("exiting", "exited"));
  expect(document.title).toBe("exiting · WebMux");
  resetBrowserChrome();
  document.title = title;
});

test("an exited session refuses input and manual resync restores final output without reopening input", async () => {
  observeGlyphPixels(true);
  const title = document.title;
  const { socket, api } = await openTerminalApi();
  socket.receive(2, sessionInfo("exit fixture"));
  expect(api.ready).toBe(true);
  socket.receive(3, JSON.stringify({ exitCode: 3 }));
  expect(api.exitCode).toBe(3);
  expect(api.ready).toBe(false);
  expect(api.submit("submission-3", "late", true, () => {})).toBe(false);
  expect(api.paste("late")).toBe(false);
  socket.drop(4410);
  await expect
    .element(page.getByRole("status", { name: "Terminal connection" }))
    .toHaveTextContent("exited");

  await page
    .getByRole("button", { name: "Resync terminal", exact: true })
    .click();
  const recovered = await reconnected(2);
  recovered.open();
  const final = "retained after exit\r\n";
  recovered.receive(
    4,
    JSON.stringify({
      generation,
      mode: "reset",
      start: 0,
      end: new TextEncoder().encode(final).length,
      truncated: false,
    }),
  );
  recovered.receive(1, final);
  recovered.receive(3, JSON.stringify({ exitCode: 3 }));
  recovered.drop(4410);
  await expectDomText("retained after exit");
  await expect
    .element(page.getByRole("status", { name: "Terminal connection" }))
    .toHaveTextContent("exited");
  expect(api.exitCode).toBe(3);
  expect(api.ready).toBe(false);
  expect(api.paste("after resync")).toBe(false);
  expect(api.submit("submission-4", "after resync", true, () => {})).toBe(
    false,
  );
  expect(inputText(socket) + inputText(recovered)).toBe("");
  resetBrowserChrome();
  document.title = title;
});

test("claim and keyboard take control only from another controller", async () => {
  observeGlyphPixels(true);
  const { socket, api } = await openTerminalApi();
  const textarea = host().querySelector<HTMLTextAreaElement>(
    ".xterm-helper-textarea",
  )!;
  expect(api.clientId).toBe(new URL(socket.url).searchParams.get("client"));
  expect(api.controlling).toBe(true);
  api.claim();
  expect(framesOf(socket, 5)).toEqual([]);

  socket.receive(
    6,
    JSON.stringify({
      controller: {
        clientId: "another-client",
        label: "desk",
        since: new Date().toISOString(),
      },
      cols: 80,
      rows: 24,
      colorSchemeReports: false,
    }),
  );
  expect(api.controlling).toBe(false);
  expect(api.control?.controller?.label).toBe("desk");
  api.claim();
  expect(framesOf(socket, 5)).toEqual([""]);
  textarea.blur();
  api.keyboard();
  expect(document.activeElement).toBe(textarea);
  expect(framesOf(socket, 5)).toEqual(["", ""]);
  // A second press hides the keyboard without claiming again.
  api.keyboard();
  expect(document.activeElement).not.toBe(textarea);
  expect(framesOf(socket, 5)).toEqual(["", ""]);
  expect(inputText(socket)).toBe("");
});

test("keyboard button reopens the IME after Android Back keeps helper focus", async () => {
  observeGlyphPixels(true);
  let dismissed = false;
  const { api } = await openTerminalApi({
    keyboardDismissed: () => dismissed,
  });
  const { textarea, focused } = blurAndTrackFocus();
  textarea.focus();
  dismissed = true;
  // focus() alone is a no-op on an already focused helper, so the toggle must
  // restart focus before the next press can hide it again.
  api.keyboard();
  expect(document.activeElement).toBe(textarea);
  expect(focused).toHaveBeenCalledTimes(2);
  // If the browser still does not open the IME, the next press must remain a
  // usable focus toggle instead of repeatedly restarting focus forever.
  api.keyboard();
  expect(document.activeElement).not.toBe(textarea);
  api.keyboard();
  expect(document.activeElement).toBe(textarea);
  expect(focused).toHaveBeenCalledTimes(3);
  dismissed = false;
  api.keyboard();
  expect(document.activeElement).not.toBe(textarea);
});

test("an invalid server frame is reported and the terminal reattaches on a fresh socket", async () => {
  observeGlyphPixels(true);
  const { socket: first } = await openTerminal();
  first.receive(
    6,
    JSON.stringify({
      controller: null,
      cols: 0,
      rows: 24,
      colorSchemeReports: false,
    }),
  );
  await expect
    .poll(
      () =>
        document.querySelector('[aria-label="Terminal error"]')?.textContent,
    )
    .not.toBe("");
  expect(first.closeCount).toBe(1);
  const second = await reconnected(2);
  second.open();
  second.receive(1, "after invalid frame\r\n");
  second.grantControl();
  await expectDomText("after invalid frame");
  expect(inputText(first) + inputText(second)).toBe("");
  // ws.ts closes the rejected socket, then again when it reopens; a real
  // WebSocket ignores the second call.
  expect(first.closeCount).toBe(2);
  first.closeCount = 1;
  const rawError = document.querySelector(
    '[aria-label="Terminal error"]',
  )?.textContent;
  const previousLocale = locale.preference;
  initializeLocale();
  try {
    locale.setPreference("en");
    await expect
      .poll(
        () =>
          document.querySelector('[aria-label="Terminal error"]')?.textContent,
      )
      .toBe(rawError);
    locale.setPreference("zh-CN");
    await expect
      .poll(
        () =>
          document.querySelector('[aria-label="Terminal error"]')?.textContent,
      )
      .toBe(rawError);
  } finally {
    locale.setPreference(previousLocale);
  }
});

test("mounted terminal stays non-editable until replay is ready and never replays premature keys", async () => {
  observeGlyphPixels(true);
  await render(Harness);
  await expect.poll(() => sockets.length).toBe(1);
  const socket = sockets[0];
  const textarea = host().querySelector<HTMLTextAreaElement>(
    ".xterm-helper-textarea",
  )!;
  await expect.poll(() => textarea.readOnly).toBe(true);
  await page.getByRole("button", { name: "Focus terminal input" }).click();
  await userEvent.keyboard("p");
  expect(inputText(socket)).toBe("");
  socket.open();
  await expect.poll(() => textarea.readOnly).toBe(true);
  socket.receive(1, snapshot);
  await expect.poll(() => textarea.readOnly).toBe(false);
  await userEvent.keyboard("printf");
  expect(inputText(socket)).toBe("printf");
});

test("DEC 1003 pointer hover stays on the reply lane without claiming control, consuming modifiers, or marking input uncertain", async () => {
  observeGlyphPixels(true);
  const { socket, api } = await openTerminalApi();
  // Enable agy's all-motion SGR mouse tracking alongside startup queries.
  socket.receive(
    0,
    "\x1b[?1049h\x1b[?1003h\x1b[?1006h\x1b[?2026$p\x1b]11;?\x07",
  );
  await expect
    .poll(() => framesOf(socket, 6).some((reply) => reply.includes("2026")))
    .toBe(true);
  socket.sent = [];

  api.toggleModifier("ctrl");
  const screen = host().querySelector<HTMLElement>(".xterm-screen")!;
  const rect = screen.getBoundingClientRect();
  for (let step = 1; step <= 8; step++) {
    screen.dispatchEvent(
      new MouseEvent("mousemove", {
        clientX: rect.left + step * 24,
        clientY: rect.top + 32,
        bubbles: true,
      }),
    );
  }
  await expect
    .poll(() =>
      framesOf(socket, 6).filter((reply) => reply.startsWith("\x1b[<35;")),
    )
    .not.toEqual([]);
  // Hover never enters FrameInput, consumes sticky modifiers, or triggers probe.
  expect(framesOf(socket, 0)).toEqual([]);
  expect(framesOf(socket, 4)).toEqual([]);
  expect(api.modifiers.ctrl).toBe(true);
  api.resetModifiers();

  // Observers never emit hover reports or claim terminal control on mousemove.
  socket.receive(
    6,
    JSON.stringify({
      controller: {
        clientId: "other-device",
        label: "laptop",
        since: new Date().toISOString(),
      },
      cols: 80,
      rows: 24,
      colorSchemeReports: false,
    }),
  );
  socket.sent = [];
  screen.dispatchEvent(
    new MouseEvent("mousemove", {
      clientX: rect.left + 60,
      clientY: rect.top + 64,
      bubbles: true,
    }),
  );
  await new Promise((resolve) => requestAnimationFrame(() => resolve(null)));
  expect(socket.sent).toEqual([]);

  // Dropping after hover-only traffic never reports unconfirmed user input.
  socket.drop();
  expect(api.error).toBe("");
});

test("acknowledged input stays certain across disconnect and 4409, and 4410 exit suppresses uncertainty", async () => {
  observeGlyphPixels(true);
  const { socket: first, api } = await openTerminalApi();
  first.answerMissingQueries = true;
  const banner = page.getByRole("status", { name: "Terminal error" });

  await page.getByRole("button", { name: "Focus terminal input" }).click();
  await userEvent.keyboard("ls");
  await expect.poll(() => inputText(first)).toBe("ls");
  // Wait for the microtask `0x05` `missing` receipt delivered by answerMissingQueries.
  await new Promise((resolve) => setTimeout(resolve, 0));

  first.drop();
  await expect.element(banner).toHaveTextContent("");
  const second = await reconnected(2);
  second.answerMissingQueries = true;
  second.open();
  second.receive(1, snapshot);
  second.grantControl();
  await expect
    .element(page.getByRole("status", { name: "Terminal restoration" }))
    .toHaveTextContent("ready");

  await page.getByRole("button", { name: "Focus terminal input" }).click();
  await userEvent.keyboard("pwd");
  await expect.poll(() => inputText(second)).toBe("pwd");
  await new Promise((resolve) => setTimeout(resolve, 0));

  second.drop(4409);
  await expect
    .element(page.getByRole("status", { name: "Terminal connection" }))
    .toHaveTextContent("moved");
  await expect.element(banner).toHaveTextContent("");

  api.reconnect();
  const third = await reconnected(3);
  third.open();
  third.receive(1, snapshot);
  third.grantControl();
  await expect
    .element(page.getByRole("status", { name: "Terminal restoration" }))
    .toHaveTextContent("ready");

  // Unacknowledged `exit` followed immediately by `4410` clean session exit remains certain.
  await page.getByRole("button", { name: "Focus terminal input" }).click();
  await userEvent.keyboard("x");
  await expect.poll(() => inputText(third)).toBe("x");
  third.receive(3, JSON.stringify({ exitCode: 0 }));
  third.drop(4410);
  await expect
    .element(page.getByRole("status", { name: "Terminal connection" }))
    .toHaveTextContent("exited");
  await expect.element(banner).toHaveTextContent("");
});

test("manual resync and notice dismissal clear terminal error and p2pState.inputUncertain", async () => {
  observeGlyphPixels(true);
  vi.spyOn(globalThis, "fetch").mockResolvedValue(
    Response.json({
      supported: true,
      udpPort: 0,
      udpBindings: [],
      capabilities: { networks: [], iceMode: "full", turn: false },
      enabled: false,
      portMapping: false,
      stunServers: [],
      source: "config",
      storageAvailable: true,
      portMap: { state: "off" },
      activePeers: 0,
      peers: [],
    }),
  );
  startP2P();
  try {
    const { socket: first, api } = await openTerminalApi({ showNotices: true });

    await page.getByRole("button", { name: "Focus terminal input" }).click();
    await userEvent.keyboard("x");
    await expect.poll(() => inputText(first)).toBe("x");
    first.drop();

    await expect.poll(() => p2pState.inputUncertain).toBe(true);
    await expect
      .element(page.getByRole("alert"))
      .toHaveTextContent(m.relay_sent_input_uncertain());

    const second = await reconnected(2);
    second.open();
    second.receive(1, snapshot);
    second.grantControl();
    await expect
      .element(page.getByRole("status", { name: "Terminal restoration" }))
      .toHaveTextContent("ready");
    // Automatic reconnect preserves uncertainty until explicit user action.
    expect(p2pState.inputUncertain).toBe(true);

    // Dismissing the strip notice clears both the banner and p2pState.inputUncertain.
    await page.getByRole("button", { name: m.s4_dismiss_notice() }).click();
    await expect.poll(() => p2pState.inputUncertain).toBe(false);
    expect(api.error).toBe("");
    await expect.element(page.getByRole("alert")).not.toBeInTheDocument();

    // Trigger uncertainty again, then clear via manual resync().
    await page.getByRole("button", { name: "Focus terminal input" }).click();
    await userEvent.keyboard("y");
    await expect.poll(() => inputText(second)).toBe("y");
    second.drop();
    await expect.poll(() => p2pState.inputUncertain).toBe(true);
    await expect
      .element(page.getByRole("alert"))
      .toHaveTextContent(m.relay_sent_input_uncertain());

    const third = await reconnected(3);
    third.open();
    third.receive(1, snapshot);
    third.grantControl();
    await expect
      .element(page.getByRole("status", { name: "Terminal restoration" }))
      .toHaveTextContent("ready");

    await page
      .getByRole("button", { name: "Resync terminal", exact: true })
      .click();
    await expect.poll(() => p2pState.inputUncertain).toBe(false);
    expect(api.error).toBe("");
    await expect.element(page.getByRole("alert")).not.toBeInTheDocument();

    // Detached unbound methods (`resync`, `reconnect`, `dismissError`) work without `this` binding.
    await page.getByRole("button", { name: "Focus terminal input" }).click();
    await userEvent.keyboard("z");
    await expect.poll(() => inputText(third)).toBe("z");
    third.drop();
    await expect.poll(() => p2pState.inputUncertain).toBe(true);
    const fourth = await reconnected(4);
    fourth.open();
    fourth.receive(1, snapshot);
    fourth.grantControl();
    await expect
      .element(page.getByRole("status", { name: "Terminal restoration" }))
      .toHaveTextContent("ready");

    const { dismissError, resync, reconnect } = api;
    resync();
    await expect.poll(() => p2pState.inputUncertain).toBe(false);
    expect(api.error).toBe("");
    expect(fourth.sent.some((frame) => frame[0] === 2)).toBe(true);

    dismissError();
    reconnect();
    await reconnected(5);
  } finally {
    stopP2P();
  }
});
