import type { IMarker, Terminal } from "@xterm/xterm";
import {
  createKittyGraphics,
  type KittyImage,
  type KittyPlacement,
  type KittyPlacementSpec,
} from "../kitty-graphics";

export interface TerminalGraphics {
  write(data: Uint8Array, parsed: () => void): void;
  reset(): void;
  dispose(): void;
}

interface RenderedPlacement {
  canvas: HTMLCanvasElement;
  image: KittyImage;
  spec: KittyPlacementSpec;
  marker?: IMarker;
  dismissed: () => void;
  closed: boolean;
}

export function attachTerminalGraphics(
  term: Terminal,
  reply: (text: string) => void,
): TerminalGraphics {
  const screen = term.element?.querySelector<HTMLElement>(".xterm-screen");
  if (!screen)
    return {
      write: (data, parsed) => term.write(data, parsed),
      reset() {},
      dispose() {},
    };
  const target = screen;
  const layer = document.createElement("div");
  layer.className = "xterm-graphics-layer";
  layer.setAttribute("aria-hidden", "true");
  screen.appendChild(layer);
  let disposed = false;
  let frame = 0;
  let lastDpr = 0;
  let lastCellWidth = 0;
  let lastCellHeight = 0;
  const rendered = new Set<RenderedPlacement>();

  function cellSize() {
    if (term.cols <= 0 || term.rows <= 0) return undefined;
    const bounds = target.getBoundingClientRect();
    if (bounds.width <= 0 || bounds.height <= 0) return undefined;
    return {
      width: bounds.width / term.cols,
      height: bounds.height / term.rows,
    };
  }

  function cursor() {
    const buffer = term.buffer.active;
    return { column: buffer.cursorX, line: buffer.baseY + buffer.cursorY };
  }

  function viewport() {
    const buffer = term.buffer.active;
    return { top: buffer.viewportY, bottom: buffer.viewportY + term.rows };
  }

  function draw(placement: RenderedPlacement) {
    const { canvas, image, spec } = placement;
    const dpr = window.devicePixelRatio || 1;
    const width = Math.max(1, Math.round(spec.drawWidth * dpr));
    const height = Math.max(1, Math.round(spec.drawHeight * dpr));
    if (canvas.width !== width) canvas.width = width;
    if (canvas.height !== height) canvas.height = height;
    canvas.style.width = `${spec.drawWidth}px`;
    canvas.style.height = `${spec.drawHeight}px`;
    const context = canvas.getContext("2d");
    if (!context) return;
    context.clearRect(0, 0, width, height);
    context.imageSmoothingEnabled = true;
    context.drawImage(
      image.source,
      spec.sourceX,
      spec.sourceY,
      spec.sourceWidth,
      spec.sourceHeight,
      0,
      0,
      width,
      height,
    );
  }

  function update() {
    if (disposed) return;
    const cell = cellSize();
    if (!cell) return;
    const dpr = window.devicePixelRatio || 1;
    const redraw = dpr !== lastDpr;
    const resized =
      cell.width !== lastCellWidth || cell.height !== lastCellHeight;
    lastDpr = dpr;
    lastCellWidth = cell.width;
    lastCellHeight = cell.height;
    const buffer = term.buffer.active;
    const viewportY = buffer.viewportY;
    for (const placement of rendered) {
      if (placement.closed) continue;
      if (resized) Object.assign(placement.spec, placement.spec.relayout(cell));
      if (redraw || resized) draw(placement);
      const marker = placement.marker;
      const line =
        marker && !marker.isDisposed ? marker.line : placement.spec.line;
      const left = placement.spec.column * cell.width + placement.spec.offsetX;
      const top = (line - viewportY) * cell.height + placement.spec.offsetY;
      const visible =
        line + placement.spec.rows > viewportY && line < viewportY + term.rows;
      placement.canvas.style.display = visible ? "" : "none";
      placement.canvas.style.transform = `translate(${left}px, ${top}px)`;
    }
  }

  function schedule() {
    if (disposed || frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      update();
    });
  }

  function remove(placement: RenderedPlacement) {
    if (placement.closed) return;
    placement.closed = true;
    rendered.delete(placement);
    placement.marker?.dispose();
    placement.canvas.remove();
    placement.dismissed();
    schedule();
  }

  const graphics = createKittyGraphics({
    cellSize,
    cursor,
    viewport,
    reply,
    write: (data, parsed) => term.write(data, parsed),
    moveCursor(columns, rows) {
      let sequence = "";
      if (rows > 0) sequence += `\x1b[${rows}B`;
      if (columns > 0) sequence += `\x1b[${columns}C`;
      if (!sequence) return;
      return new Promise<void>((resolve) =>
        term.write(sequence, () => resolve()),
      );
    },
    renderer: {
      place(image, spec, dismissed) {
        const canvas = document.createElement("canvas");
        canvas.className = "xterm-graphics-image";
        canvas.style.zIndex = String(spec.z);
        const placement: RenderedPlacement = {
          canvas,
          image,
          spec,
          dismissed,
          closed: false,
        };
        const marker = term.registerMarker(0);
        if (marker) {
          placement.marker = marker;
          marker.onDispose(() => remove(placement));
        }
        layer.appendChild(canvas);
        draw(placement);
        rendered.add(placement);
        schedule();
        const handle: KittyPlacement = {
          dispose: () => remove(placement),
          bounds: () => ({
            column: placement.spec.column,
            line:
              placement.marker && !placement.marker.isDisposed
                ? placement.marker.line
                : placement.spec.line,
            columns: placement.spec.columns,
            rows: placement.spec.rows,
          }),
        };
        return handle;
      },
    },
  });

  const scroll = term.onScroll(schedule);
  const resize = term.onResize(schedule);
  const parsed = term.onWriteParsed(schedule);
  const observer = new ResizeObserver(schedule);
  observer.observe(screen);
  schedule();

  return {
    write: (data, parsedOutput) => graphics.write(data, parsedOutput),
    reset() {
      graphics.reset();
    },
    dispose() {
      if (disposed) return;
      graphics.dispose();
      for (const placement of Array.from(rendered)) remove(placement);
      disposed = true;
      cancelAnimationFrame(frame);
      scroll.dispose();
      resize.dispose();
      parsed.dispose();
      observer.disconnect();
      layer.remove();
    },
  };
}
