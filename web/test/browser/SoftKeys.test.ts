import { afterEach, expect, test, vi } from "vite-plus/test";
import { cleanup, render } from "vitest-browser-svelte/pure";
import SoftKeys from "../../src/components/terminal/SoftKeys.svelte";

afterEach(() => cleanup());

test("soft keys keep the focused input by canceling mousedown, not pointerdown", async () => {
  const send = vi.fn();
  const { container } = await render(SoftKeys, {
    send,
    modifiers: { ctrl: false, alt: false, shift: false },
    toggle: () => {},
    paste: () => {},
    keyboard: () => {},
  });
  const arrow = [...container.querySelectorAll("button")].find(
    (button) => button.textContent === "↑",
  )!;

  // iOS Safari blurs the active input on pointerdown even when canceled, so the
  // event must reach the button's compatibility mousedown.
  const pointer = new PointerEvent("pointerdown", {
    bubbles: true,
    cancelable: true,
    isPrimary: true,
  });
  arrow.dispatchEvent(pointer);
  expect(pointer.defaultPrevented).toBe(false);
  arrow.dispatchEvent(
    new PointerEvent("pointerup", { bubbles: true, isPrimary: true }),
  );

  const mouse = new MouseEvent("mousedown", {
    bubbles: true,
    cancelable: true,
  });
  arrow.dispatchEvent(mouse);
  expect(mouse.defaultPrevented).toBe(true);

  // Canceling mousedown must not swallow the key action.
  arrow.click();
  expect(send).toHaveBeenCalledWith("\x1b[A");
});

test("Fn switches the single key row between primary keys and extended symbols, navigation, and shortcuts", async () => {
  const send = vi.fn();
  const { container } = await render(SoftKeys, {
    send,
    modifiers: { ctrl: false, alt: false, shift: false },
    toggle: () => {},
    paste: () => {},
    keyboard: () => {},
  });

  const rowLabels = () =>
    [...container.querySelectorAll(".soft-row button")]
      .map((button) => button.textContent?.trim())
      .filter(Boolean);

  const fnButton = [...container.querySelectorAll(".soft-leading button")].find(
    (node) => node.textContent === "Fn",
  )!;
  expect(fnButton).toBeTruthy();
  expect(fnButton.getAttribute("aria-pressed")).toBe("false");

  // Default layer holds core RDP terminal keys only.
  expect(rowLabels()).toEqual([
    "Esc",
    "Tab",
    "↵",
    "Ctrl",
    "Alt",
    "Shift",
    "↑",
    "↓",
    "←",
    "→",
  ]);
  const enterButton = [...container.querySelectorAll(".soft-row button")].find(
    (node) => node.textContent === "↵",
  )!;
  enterButton.click();
  expect(send).toHaveBeenCalledWith("\r");

  // Switching Fn reveals symbols, navigation keys, and Ctrl combinations in-place.
  fnButton.click();
  await expect.poll(() => fnButton.getAttribute("aria-pressed")).toBe("true");
  expect(rowLabels()).toEqual([
    "/",
    "@",
    "#",
    "!",
    "Home",
    "End",
    "PgUp",
    "PgDn",
    "Ctrl+C",
    "Ctrl+D",
    "Ctrl+Z",
    "Ctrl+L",
    "Ctrl+R",
  ]);

  for (const symbol of ["/", "@", "#", "!"]) {
    const button = [...container.querySelectorAll(".soft-row button")].find(
      (node) => node.textContent === symbol,
    )!;
    send.mockClear();
    button.click();
    expect(send).toHaveBeenCalledWith(symbol);
  }

  // Toggling Fn off returns to the primary layer.
  fnButton.click();
  await expect.poll(() => fnButton.getAttribute("aria-pressed")).toBe("false");
  expect(rowLabels()).toContain("Esc");
  expect(rowLabels()).not.toContain("Ctrl+R");
});

test("Shift taps toggle once while a long press locks it", async () => {
  const send = vi.fn();
  const toggle = vi.fn();
  const lock = vi.fn();
  const { container } = await render(SoftKeys, {
    send,
    modifiers: { ctrl: false, alt: false, shift: false },
    toggle,
    lock,
    paste: () => {},
    keyboard: () => {},
  });
  const buttons = [...container.querySelectorAll("button")];
  const shift = buttons.find((button) => button.textContent === "Shift")!;
  const tab = buttons.find((button) => button.textContent === "Tab")!;
  const pointer = (type: string, pointerType = "touch") =>
    shift.dispatchEvent(
      new PointerEvent(type, {
        bubbles: true,
        cancelable: true,
        isPrimary: true,
        pointerType,
      }),
    );

  // A short tap keeps the one-shot toggle behavior of the other modifiers.
  pointer("pointerdown");
  pointer("pointerup");
  shift.click();
  expect(toggle).toHaveBeenCalledWith("shift");
  expect(lock).not.toHaveBeenCalled();
  // Releasing must cancel the pending timer, not just hide it.
  await new Promise((resolve) => setTimeout(resolve, 700));
  expect(lock).not.toHaveBeenCalled();

  // A mouse hold also locks Shift. Only its release click is ignored.
  toggle.mockClear();
  pointer("pointerdown", "mouse");
  await new Promise((resolve) => setTimeout(resolve, 700));
  expect(lock).toHaveBeenCalledWith("shift");
  pointer("pointerup", "mouse");
  shift.click();
  expect(toggle).not.toHaveBeenCalled();
  tab.click();
  expect(send).toHaveBeenCalledWith("\t");

  // Moving the pointer turns the press into a scroll, not a lock.
  lock.mockClear();
  pointer("pointerdown");
  shift.dispatchEvent(
    new PointerEvent("pointermove", {
      bubbles: true,
      clientX: shift.getBoundingClientRect().left + 20,
      clientY: shift.getBoundingClientRect().top,
    }),
  );
  await new Promise((resolve) => setTimeout(resolve, 700));
  expect(lock).not.toHaveBeenCalled();
  pointer("pointerup");
  shift.click();
  expect(toggle).toHaveBeenCalledWith("shift");
});

test("a locked Shift shows its state and unlock hint", async () => {
  const { container } = await render(SoftKeys, {
    send: () => {},
    modifiers: { ctrl: false, alt: false, shift: true },
    locked: true,
    toggle: () => {},
    paste: () => {},
    keyboard: () => {},
  });
  const shift = [...container.querySelectorAll("button")].find(
    (button) => button.textContent === "Shift",
  );

  expect(shift).toBeTruthy();
  expect(shift!.querySelector("svg")).toBeTruthy();
  expect(shift!.getAttribute("title")).toBe("已锁定，点按解除");
  expect(shift!.getAttribute("aria-pressed")).toBe("true");
});

test("arrow keys repeat while held stationary and cancel on horizontal scroll drag", async () => {
  const send = vi.fn();
  const { container } = await render(SoftKeys, {
    send,
    modifiers: { ctrl: false, alt: false, shift: false },
    toggle: () => {},
    paste: () => {},
    keyboard: () => {},
  });
  const left = [...container.querySelectorAll(".soft-row button")].find(
    (button) => button.textContent === "←",
  )!;

  // Stationary hold triggers repeat and suppresses the release click.
  left.dispatchEvent(
    new PointerEvent("pointerdown", {
      bubbles: true,
      cancelable: true,
      isPrimary: true,
      pointerId: 1,
      button: 0,
      clientX: 100,
      clientY: 100,
    }),
  );
  await new Promise((resolve) => setTimeout(resolve, 720));
  const repeatedCount = send.mock.calls.length;
  expect(repeatedCount).toBeGreaterThanOrEqual(2);
  expect(send).toHaveBeenLastCalledWith("\x1b[D");
  left.dispatchEvent(
    new PointerEvent("pointerup", {
      bubbles: true,
      isPrimary: true,
      pointerId: 1,
    }),
  );
  left.click();
  expect(send).toHaveBeenCalledTimes(repeatedCount);

  // Dragging past threshold cancels the hold so scrolling emits nothing.
  send.mockClear();
  left.dispatchEvent(
    new PointerEvent("pointerdown", {
      bubbles: true,
      cancelable: true,
      isPrimary: true,
      pointerId: 2,
      button: 0,
      clientX: 100,
      clientY: 100,
    }),
  );
  left.dispatchEvent(
    new PointerEvent("pointermove", {
      bubbles: true,
      isPrimary: true,
      pointerId: 2,
      clientX: 125,
      clientY: 100,
    }),
  );
  await new Promise((resolve) => setTimeout(resolve, 400));
  left.dispatchEvent(
    new PointerEvent("pointercancel", {
      bubbles: true,
      isPrimary: true,
      pointerId: 2,
    }),
  );
  expect(send).not.toHaveBeenCalled();
});
