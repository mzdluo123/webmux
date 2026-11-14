import type { IBufferCell, ITheme, Terminal } from "@xterm/xterm";
import { observeTerminalCursor } from "./terminal-cursor";

const paletteNames = [
  "black",
  "red",
  "green",
  "yellow",
  "blue",
  "magenta",
  "cyan",
  "white",
  "brightBlack",
  "brightRed",
  "brightGreen",
  "brightYellow",
  "brightBlue",
  "brightMagenta",
  "brightCyan",
  "brightWhite",
] as const;
const defaultPalette = [
  "#2e3436",
  "#cc0000",
  "#4e9a06",
  "#c4a000",
  "#3465a4",
  "#75507b",
  "#06989a",
  "#d3d7cf",
  "#555753",
  "#ef2929",
  "#8ae234",
  "#fce94f",
  "#729fcf",
  "#ad7fa8",
  "#34e2e2",
  "#eeeeec",
];
const rgb = (value: number) => `#${value.toString(16).padStart(6, "0")}`;

export interface XrTerminalCanvas {
  readonly canvas: HTMLCanvasElement;
  update(): boolean;
  dispose(): void;
}

/** Draws only public parsed buffer cells; never reads the DOM renderer or its canvases. */
export function createXrTerminalCanvas(terminal: Terminal): XrTerminalCanvas {
  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  if (!context) throw new Error("VR 终端画布不可用");
  const ctx = context;
  let disposed = false;
  let dirty = true;
  let force = true;
  const cursorState = observeTerminalCursor(terminal);
  let previousOptions = "";
  let signatures: string[] = [];
  let cellWidth = 1;
  let cellHeight = 1;
  let fontSize = 14;
  let fontFamily = "monospace";
  let theme: ITheme = {};
  const palette: string[] = [];
  let foreground = "#ffffff";
  let background = "#000000";
  let previousCursor = "";
  let hasBlinkingText = false;
  let previousBlink = true;
  const mark = () => {
    dirty = true;
  };
  const listeners = [
    terminal.onWriteParsed(mark),
    terminal.onScroll(mark),
    terminal.onResize(() => {
      dirty = true;
      force = true;
    }),
    terminal.onCursorMove(mark),
    terminal.buffer.onBufferChange(mark),
  ];
  const colors = new MutationObserver(() => {
    dirty = true;
    force = true;
  });
  colors.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  });
  const fonts = () => {
    dirty = true;
    force = true;
  };
  document.fonts.addEventListener("loadingdone", fonts);

  function color(cell: IBufferCell, fg: boolean) {
    if (fg ? cell.isFgDefault() : cell.isBgDefault())
      return fg ? foreground : background;
    const value = fg ? cell.getFgColor() : cell.getBgColor();
    if (fg ? cell.isFgRGB() : cell.isBgRGB()) return rgb(value);
    const index =
      fg &&
      cell.isBold() &&
      terminal.options.drawBoldTextInBrightColors &&
      value < 8
        ? value + 8
        : value;
    return palette[index] ?? (fg ? foreground : background);
  }
  function attributes(cell: IBufferCell) {
    return (
      Number(!!cell.isBold()) |
      (Number(!!cell.isItalic()) << 1) |
      (Number(!!cell.isDim()) << 2) |
      (Number(!!cell.isUnderline()) << 3) |
      (Number(!!cell.isInverse()) << 4) |
      (Number(!!cell.isInvisible()) << 5) |
      (Number(!!cell.isStrikethrough()) << 6) |
      (Number(!!cell.isOverline()) << 7) |
      (Number(!!cell.isBlink()) << 8)
    );
  }
  function drawRow(row: number, blink: boolean) {
    const line = terminal.buffer.active.getLine(
      terminal.buffer.active.viewportY + row,
    );
    const y = row * cellHeight;
    ctx.globalAlpha = 1;
    ctx.fillStyle = background;
    ctx.fillRect(0, y, terminal.cols * cellWidth, cellHeight);
    if (!line) return;
    const reusable = terminal.buffer.active.getNullCell();
    for (let column = 0; column < terminal.cols; column++) {
      const cell = line.getCell(column, reusable);
      if (!cell || !cell.getWidth()) continue;
      const width = cell.getWidth() * cellWidth;
      const x = column * cellWidth;
      let fg = color(cell, true);
      let bg = color(cell, false);
      if (cell.isInverse()) [fg, bg] = [bg, fg];
      ctx.globalAlpha = 1;
      ctx.fillStyle = bg;
      ctx.fillRect(x, y, width, cellHeight);
      if (cell.isInvisible() || (cell.isBlink() && !blink)) continue;
      ctx.save();
      ctx.beginPath();
      ctx.rect(x, y, width, cellHeight);
      ctx.clip();
      ctx.fillStyle = fg;
      ctx.globalAlpha = cell.isDim() ? 0.5 : 1;
      const weight = cell.isBold()
        ? (terminal.options.fontWeightBold ?? 700)
        : (terminal.options.fontWeight ?? 400);
      ctx.font = `${cell.isItalic() ? "italic " : ""}${weight} ${fontSize}px ${fontFamily}`;
      ctx.fillText(
        cell.getChars(),
        x,
        y + (cellHeight - fontSize) / 2 + fontSize * 0.8,
      );
      const thickness = Math.max(1, fontSize / 14);
      if (cell.isUnderline())
        ctx.fillRect(x, y + cellHeight - thickness * 2, width, thickness);
      if (cell.isStrikethrough())
        ctx.fillRect(x, y + cellHeight * 0.5, width, thickness);
      if (cell.isOverline()) ctx.fillRect(x, y, width, thickness);
      ctx.restore();
    }
  }

  return {
    canvas,
    update(): boolean {
      if (disposed || terminal.modes.synchronizedOutputMode) return false;
      const options = terminal.options;
      const optionSignature = JSON.stringify([
        terminal.cols,
        terminal.rows,
        options.theme,
        options.fontFamily,
        options.fontSize,
        options.fontWeight,
        options.fontWeightBold,
        options.lineHeight,
        options.letterSpacing,
        options.drawBoldTextInBrightColors,
      ]);
      if (optionSignature !== previousOptions) {
        previousOptions = optionSignature;
        force = true;
        dirty = true;
        theme = options.theme ?? {};
        foreground = theme.foreground ?? "#ffffff";
        background = theme.background ?? "#000000";
        for (let index = 0; index < 256; index++) {
          if (index < 16)
            palette[index] =
              theme[paletteNames[index]!] ?? defaultPalette[index]!;
          else if (index < 232) {
            const n = index - 16;
            const levels = [0, 95, 135, 175, 215, 255];
            palette[index] = rgb(
              (levels[Math.floor(n / 36)]! << 16) |
                (levels[Math.floor(n / 6) % 6]! << 8) |
                levels[n % 6]!,
            );
          } else {
            const gray = 8 + (index - 232) * 10;
            palette[index] = rgb((gray << 16) | (gray << 8) | gray);
          }
        }
        fontSize = options.fontSize ?? 14;
        fontFamily = options.fontFamily ?? "monospace";
        ctx.font = `${options.fontWeight ?? 400} ${fontSize}px ${fontFamily}`;
        cellWidth = Math.max(
          1,
          ctx.measureText("W").width + (options.letterSpacing ?? 0),
        );
        cellHeight = Math.max(
          1,
          Math.ceil(fontSize * (options.lineHeight ?? 1) * 1.2),
        );
        const width = terminal.cols * cellWidth;
        const height = terminal.rows * cellHeight;
        const scale = Math.min(
          2,
          4096 / width,
          4096 / height,
          Math.sqrt((4 * 1024 * 1024) / (width * height)),
        );
        canvas.width = 0;
        canvas.height = Math.max(1, Math.floor(height * scale));
        canvas.width = Math.max(1, Math.floor(width * scale));
        ctx.setTransform(scale, 0, 0, scale, 0, 0);
        signatures = [];
      }
      const buffer = terminal.buffer.active;
      const cursorRow = buffer.baseY + buffer.cursorY - buffer.viewportY;
      const blink = Math.floor(performance.now() / 500) % 2 === 0;
      const cursor =
        cursorState.visible &&
        (!options.cursorBlink || blink) &&
        cursorRow >= 0 &&
        cursorRow < terminal.rows;
      if (hasBlinkingText && previousBlink !== blink) dirty = true;
      const cursorSignature = `${cursor}:${cursorRow}:${buffer.cursorX}:${options.cursorStyle}:${theme.cursor}`;
      if (cursorSignature !== previousCursor) dirty = true;
      if (!dirty) return false;
      previousCursor = cursorSignature;
      previousBlink = blink;
      hasBlinkingText = false;
      let changed = false;
      const reusable = buffer.getNullCell();
      for (let row = 0; row < terminal.rows; row++) {
        const line = buffer.getLine(buffer.viewportY + row);
        let signature = row === cursorRow && cursor ? cursorSignature : "";
        if (line)
          for (let column = 0; column < terminal.cols; column++) {
            const cell = line.getCell(column, reusable);
            if (cell)
              signature += `|${cell.getChars()}:${cell.getWidth()}:${cell.getFgColorMode()}:${cell.getFgColor()}:${cell.getBgColorMode()}:${cell.getBgColor()}:${attributes(cell)}`;
            if (cell?.isBlink()) {
              hasBlinkingText = true;
              signature += `:${blink}`;
            }
          }
        if (!force && signature === signatures[row]) continue;
        signatures[row] = signature;
        drawRow(row, blink);
        changed = true;
        if (cursor && row === cursorRow && buffer.cursorX < terminal.cols) {
          ctx.globalAlpha =
            options.cursorStyle === "bar" || options.cursorStyle === "underline"
              ? 1
              : 0.45;
          ctx.fillStyle = theme.cursor ?? foreground;
          const x = buffer.cursorX * cellWidth;
          const y = cursorRow * cellHeight;
          const style = options.cursorStyle;
          ctx.fillRect(
            x,
            y + (style === "underline" ? cellHeight - 2 : 0),
            style === "bar" ? 2 : cellWidth,
            style === "underline" ? 2 : cellHeight,
          );
          ctx.globalAlpha = 1;
        }
      }
      dirty = false;
      force = false;
      return changed;
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      for (const listener of listeners) listener.dispose();
      cursorState.dispose();
      colors.disconnect();
      document.fonts.removeEventListener("loadingdone", fonts);
      signatures = [];
      canvas.width = 0;
      canvas.height = 0;
    },
  };
}
