import type { Terminal } from "@xterm/xterm";

type Repaint = (...args: unknown[]) => void;

interface RendererInternals {
  clear?: Repaint;
}

interface RenderServiceInternals {
  handleSelectionChanged?: Repaint;
  setRenderer?: (renderer: RendererInternals | undefined) => void;
  _renderer?: { value?: RendererInternals };
}

// xterm 6.0 gates its repaints on DEC 2026 synchronized output, except two
// that reach the DOM renderer directly: the selection refresh queued by every
// scroll repaints all rows, and a buffer reset or switch empties them. A
// synchronized redraw that scrolls, such as a TUI reprinting its transcript,
// or a replay after reset() then showed partial or blank frames anyway.
// Hold both while an update is synchronized: the emptied rows are repainted
// by the full refresh requested instead, and the latest selection is replayed
// right after xterm paints the finished update. The WebGL renderer already
// routes both through the gated refresh, so this only changes the DOM path.
// Internals are feature-detected; without them xterm keeps its own behavior.
export function holdRepaintsDuringSync(term: Terminal): () => void {
  const service = (
    term as unknown as {
      _core?: { _renderService?: RenderServiceInternals };
    }
  )._core?._renderService;
  const selection = service?.handleSelectionChanged;
  const setRenderer = service?.setRenderer;
  if (
    !service ||
    typeof selection !== "function" ||
    typeof setRenderer !== "function"
  )
    return () => {};
  const synchronized = () => term.modes.synchronizedOutputMode;
  const restores: (() => void)[] = [];

  const holdClear = (renderer: RendererInternals | undefined) => {
    const clear = renderer?.clear;
    if (!renderer || typeof clear !== "function") return;
    const held: Repaint = (...args) => {
      if (synchronized()) term.refresh(0, term.rows - 1);
      else clear.apply(renderer, args);
    };
    renderer.clear = held;
    restores.push(() => {
      if (renderer.clear === held) renderer.clear = clear;
    });
  };
  holdClear(service._renderer?.value);
  const heldSetRenderer: typeof setRenderer = (renderer) => {
    holdClear(renderer);
    setRenderer.call(service, renderer);
  };
  service.setRenderer = heldSetRenderer;

  let pending: unknown[] | undefined;
  const heldSelection: Repaint = (...args) => {
    if (synchronized()) {
      pending = args;
      return;
    }
    pending = undefined;
    selection.apply(service, args);
  };
  service.handleSelectionChanged = heldSelection;
  const rendered = term.onRender(() => {
    if (!pending || synchronized()) return;
    const args = pending;
    pending = undefined;
    selection.apply(service, args);
  });

  return () => {
    rendered.dispose();
    if (service.handleSelectionChanged === heldSelection)
      service.handleSelectionChanged = selection;
    if (service.setRenderer === heldSetRenderer)
      service.setRenderer = setRenderer;
    for (const restore of restores.splice(0)) restore();
  };
}
