import type { IDisposable, Terminal } from "@xterm/xterm";

interface CursorState {
  visible: boolean;
  owners: number;
  listeners: IDisposable[];
}
const cursors = new WeakMap<Terminal, CursorState>();

/** Track DECTCEM from model creation: xterm's public modes omit cursor visibility. */
export function observeTerminalCursor(terminal: Terminal) {
  let state = cursors.get(terminal);
  if (!state) {
    state = { visible: true, owners: 0, listeners: [] };
    const cursor = state;
    cursor.listeners = [
      terminal.parser.registerCsiHandler(
        { prefix: "?", final: "h" },
        (params) => {
          if (params.includes(25)) cursor.visible = true;
          return false;
        },
      ),
      terminal.parser.registerCsiHandler(
        { prefix: "?", final: "l" },
        (params) => {
          if (params.includes(25)) cursor.visible = false;
          return false;
        },
      ),
      terminal.parser.registerEscHandler({ final: "c" }, () => {
        cursor.visible = true;
        return false;
      }),
    ];
    cursors.set(terminal, state);
  }
  state.owners++;
  const cursor = state;
  let disposed = false;
  return {
    get visible() {
      return cursor.visible;
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      if (--cursor.owners) return;
      for (const listener of cursor.listeners) listener.dispose();
      cursors.delete(terminal);
    },
  };
}

export function resetTerminalCursor(terminal: Terminal) {
  const cursor = cursors.get(terminal);
  if (cursor) cursor.visible = true;
}
