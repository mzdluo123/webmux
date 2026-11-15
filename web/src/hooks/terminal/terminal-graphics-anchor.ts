import type { IBuffer, IDisposable } from "@xterm/xterm";

interface LineList {
  readonly length: number;
  get(index: number): object | undefined;
  onTrim(listener: (amount: number) => void): IDisposable;
  onInsert(
    listener: (event: { index: number; amount: number }) => void,
  ): IDisposable;
  onDelete(
    listener: (event: { index: number; amount: number }) => void,
  ): IDisposable;
}

interface XtermBuffer extends IBuffer {
  _buffer: {
    lines: LineList;
    _cols: number;
    _bufferService: { cols: number };
  };
}

// xterm 6's public markers are text markers: ED/EL dispose them even though
// Kitty graphics must survive text erasure. Subscribe to actual line movement
// instead. Keep the pinned xterm internal API isolated here; browser regressions
// exercise erase, scrolling, scrollback trimming and resize against real xterm.
export function graphicsAnchor(
  buffer: IBuffer,
  initialLine: number,
  rows: () => number,
  removed: () => void,
) {
  // BufferApiView in the pinned xterm version wraps this line list.
  const internal = buffer as XtermBuffer;
  const lines = internal._buffer.lines;
  // CircularList.get wraps indices without bounds checking against length.
  const lineAt = (index: number) =>
    index >= 0 && index < lines.length ? lines.get(index) : undefined;
  let line = initialLine;
  let offset = 0;
  let identity = lineAt(line);
  let disposed = false;
  const listeners = [
    lines.onTrim((amount) => {
      line -= amount;
      if (line + Math.max(1, rows()) <= 0) {
        removed();
        return;
      }
      if (line + offset < 0) {
        offset = -line;
        identity = lineAt(0);
        if (!identity) removed();
      }
    }),
    lines.onInsert((event) => {
      if (line + offset >= event.index) line += event.amount;
    }),
    lines.onDelete((event) => {
      const raw = internal._buffer;
      const reflowing =
        raw._cols !== raw._bufferService.cols || event.index < buffer.baseY;
      const tracked = line + offset;
      if (tracked >= event.index && tracked < event.index + event.amount) {
        // Widening reflow unwraps continuation lines onto the preceding line.
        if (reflowing) line = Math.max(0, event.index - 1) - offset;
        else removed();
      } else if (tracked >= event.index + event.amount) {
        line -= event.amount;
      }
    }),
  ];
  return {
    get line() {
      return line;
    },
    sync() {
      if (internal._buffer.lines !== lines) {
        removed();
        return;
      }
      if (disposed || (identity && lineAt(line + offset) === identity)) return;
      // DECSTBM index scrolling shifts BufferLine objects without emitting
      // insert/delete events. Search only the active screen, not scrollback.
      for (let row = buffer.baseY; row < buffer.length; row++) {
        if (lineAt(row) === identity) {
          line = row - offset;
          return;
        }
      }
      removed();
    },
    resized() {
      if (disposed) return;
      if (internal._buffer.lines !== lines || line + Math.max(1, rows()) <= 0) {
        removed();
        return;
      }
      // Reflow reports insert/delete movement, but can replace line objects.
      identity = lineAt(line + offset);
      if (!identity) removed();
    },
    dispose() {
      disposed = true;
      for (const listener of listeners) listener.dispose();
    },
  };
}
