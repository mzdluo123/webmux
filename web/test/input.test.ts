import { describe, expect, test, vi } from "vite-plus/test";
import {
  createTerminalKeyRepeat,
  modifiedInput,
  terminalKeys,
} from "../src/domains/terminal/terminal-keys";

describe("one-shot terminal modifiers", () => {
  test.each([
    ["c", "\x03"],
    ["C", "\x03"],
    ["@", "\x00"],
    ["[", "\x1b"],
    ["\\", "\x1c"],
    ["]", "\x1d"],
    ["^", "\x1e"],
    ["_", "\x1f"],
    ["z", "\x1a"],
  ])("Ctrl+%s produces the PTY control byte", (input, output) => {
    expect(modifiedInput(input, { ctrl: true, alt: false, shift: false })).toBe(
      output,
    );
  });

  test.each(["中文", "😀", "\x1b[A", "paste\ncontent", "1", ""])(
    "Ctrl preserves non-control input %j",
    (input) => {
      expect(
        modifiedInput(input, { ctrl: true, alt: false, shift: false }),
      ).toBe(input);
    },
  );

  test("Alt prefixes Escape after Ctrl translation without modifying sticky state", () => {
    const modifiers = { ctrl: true, alt: true, shift: false };
    expect(modifiedInput("C", modifiers)).toBe("\x1b\x03");
    expect(modifiers).toEqual({ ctrl: true, alt: true, shift: false });
    expect(modifiedInput("x", { ctrl: false, alt: true, shift: false })).toBe(
      "\x1bx",
    );
    expect(
      modifiedInput("中文", { ctrl: false, alt: true, shift: false }),
    ).toBe("\x1b中文");
  });
});

describe("shift modifier", () => {
  test.each([
    ["a", "A"],
    ["A", "A"],
    ["1", "!"],
    ["/", "?"],
    ["-", "_"],
    ["\\", "|"],
    [" ", " "],
    ["@", "@"],
    ["中", "中"],
    ["ß", "ß"],
    ["é", "é"],
    ["中文", "中文"],
    ["paste\ncontent", "paste\ncontent"],
    ["\x1b[1;2A", "\x1b[1;2A"],
    ["", ""],
  ])("Shift turns %j into %j without double-shifting", (input, output) => {
    expect(modifiedInput(input, { ctrl: false, alt: false, shift: true })).toBe(
      output,
    );
  });

  test.each([
    ["\t", "\x1b[Z"],
    ["\x1b[A", "\x1b[1;2A"],
    ["\x1b[B", "\x1b[1;2B"],
    ["\x1b[C", "\x1b[1;2C"],
    ["\x1b[D", "\x1b[1;2D"],
    ["\x1b[H", "\x1b[1;2H"],
    ["\x1b[F", "\x1b[1;2F"],
    ["\x1b[5~", "\x1b[5;2~"],
    ["\x1b[6~", "\x1b[6;2~"],
  ])("Shift encodes %j as %j", (input, output) => {
    expect(modifiedInput(input, { ctrl: false, alt: false, shift: true })).toBe(
      output,
    );
  });

  test("Ctrl keeps precedence and Alt is still prefixed", () => {
    expect(modifiedInput("a", { ctrl: true, alt: false, shift: true })).toBe(
      "\x01",
    );
    expect(modifiedInput("A", { ctrl: false, alt: true, shift: true })).toBe(
      "\x1bA",
    );
  });
});

test("shared held keys send once, switch direction once, and stop on release", () => {
  vi.useFakeTimers();
  try {
    const send = vi.fn();
    const repeat = createTerminalKeyRepeat(send);
    repeat.start(terminalKeys.up);
    expect(send.mock.calls).toEqual([[terminalKeys.up]]);
    vi.advanceTimersByTime(424);
    expect(send).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1);
    expect(send.mock.calls).toEqual([[terminalKeys.up], [terminalKeys.up]]);
    repeat.start(terminalKeys.left);
    expect(send.mock.calls.at(-1)).toEqual([terminalKeys.left]);
    repeat.stop();
    vi.advanceTimersByTime(1000);
    expect(send).toHaveBeenCalledTimes(3);
    expect(terminalKeys.interrupt).toBe("\x03");
    expect(
      modifiedInput(terminalKeys.tab, { ctrl: false, alt: false, shift: true }),
    ).toBe("\x1b[Z");
  } finally {
    vi.useRealTimers();
  }
});
