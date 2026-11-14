export interface Modifiers {
  ctrl: boolean;
  alt: boolean;
  shift: boolean;
}

// Shifted US-layout symbols for the soft-key row. Letters are handled by
// upper-casing, and characters that are already shifted stay unchanged so a
// physical Shift press is not applied twice.
const shiftedSymbols: Record<string, string> = {
  "`": "~",
  "1": "!",
  "2": "@",
  "3": "#",
  "4": "$",
  "5": "%",
  "6": "^",
  "7": "&",
  "8": "*",
  "9": "(",
  "0": ")",
  "-": "_",
  "=": "+",
  "[": "{",
  "]": "}",
  "\\": "|",
  ";": ":",
  "'": '"',
  ",": "<",
  ".": ">",
  "/": "?",
};

// xterm's modifier parameter is 2 for Shift: CSI 1 ; 2 <final> for the cursor
// keys and CSI <number> ; 2 ~ for the editing keys. Shift+Tab is its own
// sequence.
const shiftedSequences: Record<string, string> = {
  "\t": "\x1b[Z",
  "\x1b[A": "\x1b[1;2A",
  "\x1b[B": "\x1b[1;2B",
  "\x1b[C": "\x1b[1;2C",
  "\x1b[D": "\x1b[1;2D",
  "\x1b[H": "\x1b[1;2H",
  "\x1b[F": "\x1b[1;2F",
  "\x1b[5~": "\x1b[5;2~",
  "\x1b[6~": "\x1b[6;2~",
};

function withShift(input: string): string {
  // Tab is a single character but has its own Shift sequence.
  const sequence = shiftedSequences[input];
  if (typeof sequence === "string") return sequence;
  if (input.length === 1) {
    // US layout only. Non-ASCII passes through so IME and physical Shift input
    // are not case-mapped twice (or expanded, as with U+00DF).
    if (input >= "a" && input <= "z") return input.toUpperCase();
    return shiftedSymbols[input] ?? input;
  }
  return input;
}

export function modifiedInput(input: string, modifiers: Modifiers): string {
  let result = input;
  if (modifiers.ctrl && input.length === 1 && /^[a-z@[\]\\^_]$/i.test(input)) {
    result = String.fromCharCode(input.toUpperCase().charCodeAt(0) & 0x1f);
  } else if (modifiers.shift) {
    result = withShift(input);
  }
  return modifiers.alt ? "\x1b" + result : result;
}

export const terminalKeys = {
  escape: "\x1b",
  tab: "\t",
  enter: "\r",
  space: " ",
  up: "\x1b[A",
  down: "\x1b[B",
  left: "\x1b[D",
  right: "\x1b[C",
  home: "\x1b[H",
  end: "\x1b[F",
  pageUp: "\x1b[5~",
  pageDown: "\x1b[6~",
  interrupt: "\x03",
  eof: "\x04",
  suspend: "\x1a",
  clear: "\x0c",
  reverseSearch: "\x12",
} as const;
export const terminalSymbols = ["/", "@", "#", "!"] as const;
export const terminalLeadingKeys = [
  ["Esc", terminalKeys.escape],
  ["Tab", terminalKeys.tab],
  ["↵", terminalKeys.enter],
] as const;
export const terminalArrowKeys = [
  ["↑", terminalKeys.up],
  ["↓", terminalKeys.down],
  ["←", terminalKeys.left],
  ["→", terminalKeys.right],
] as const;
export const terminalNavigationKeys = [
  ["Home", terminalKeys.home],
  ["End", terminalKeys.end],
  ["PgUp", terminalKeys.pageUp],
  ["PgDn", terminalKeys.pageDown],
] as const;
export const terminalCombinations = [
  ["Ctrl+C", terminalKeys.interrupt],
  ["Ctrl+D", terminalKeys.eof],
  ["Ctrl+Z", terminalKeys.suspend],
  ["Ctrl+L", terminalKeys.clear],
  ["Ctrl+R", terminalKeys.reverseSearch],
] as const;
export const terminalModifierKeys = [
  ["ctrl", "Ctrl"],
  ["alt", "Alt"],
  ["shift", "Shift"],
] as const;

/** Pointer ownership belongs to the caller; repeat only delivers raw keys. */
export function createTerminalKeyRepeat(send: (input: string) => void) {
  let delay: ReturnType<typeof setTimeout> | undefined;
  let interval: ReturnType<typeof setInterval> | undefined;
  function stop() {
    clearTimeout(delay);
    clearInterval(interval);
    delay = undefined;
    interval = undefined;
  }
  return {
    stop,
    start(input: string) {
      stop();
      send(input);
      delay = setTimeout(() => {
        delay = undefined;
        interval = setInterval(() => send(input), 75);
      }, 350);
    },
  };
}
