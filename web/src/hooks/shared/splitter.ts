// WAI-ARIA window splitters that set a width or height in pixels: the handle
// follows a primary-button drag, and arrow keys move it by fixed steps, Home
// and End to its bounds. `set` keeps the bounds.

function trackPrimaryDrag(
  event: PointerEvent,
  onDelta: (dx: number, dy: number) => void,
) {
  if (event.button !== 0) return;
  event.preventDefault();
  const handle = event.currentTarget as HTMLElement;
  handle.setPointerCapture(event.pointerId);
  const startX = event.clientX;
  const startY = event.clientY;
  const move = (e: PointerEvent) =>
    onDelta(e.clientX - startX, e.clientY - startY);
  const end = () => {
    handle.removeEventListener("pointermove", move);
    handle.removeEventListener("pointerup", end);
    handle.removeEventListener("pointercancel", end);
  };
  handle.addEventListener("pointermove", move);
  handle.addEventListener("pointerup", end);
  handle.addEventListener("pointercancel", end);
}

/** Follows a drag that starts on the handle at `width` until it ends. */
export function dragWidth(
  event: PointerEvent,
  width: number,
  set: (width: number) => void,
) {
  trackPrimaryDrag(event, (dx) => set(width + dx));
}

/** Applies a splitter key; other keys are left alone. */
export function keyWidth(
  event: KeyboardEvent,
  width: number,
  bounds: { min: number; max: number },
  set: (width: number) => void,
) {
  const step = event.shiftKey ? 64 : 16;
  if (event.key === "ArrowLeft") set(width - step);
  else if (event.key === "ArrowRight") set(width + step);
  else if (event.key === "Home") set(bounds.min);
  else if (event.key === "End") set(bounds.max);
  else return;
  event.preventDefault();
}

/** Follows a vertical drag on a horizontal splitter handle (`invert` for a dock anchored to the bottom edge). */
export function dragHeight(
  event: PointerEvent,
  height: number,
  set: (height: number) => void,
  invert = false,
) {
  const dir = invert ? -1 : 1;
  trackPrimaryDrag(event, (_dx, dy) => set(height + dir * dy));
}

/** Applies vertical splitter keys (`invert` increases height on ArrowUp for a bottom dock). */
export function keyHeight(
  event: KeyboardEvent,
  height: number,
  bounds: { min: number; max: number },
  set: (height: number) => void,
  invert = false,
) {
  const step = event.shiftKey ? 64 : 24;
  const dir = invert ? -1 : 1;
  if (event.key === "ArrowDown") set(height + dir * step);
  else if (event.key === "ArrowUp") set(height - dir * step);
  else if (event.key === "Home") set(bounds.min);
  else if (event.key === "End") set(bounds.max);
  else return;
  event.preventDefault();
}
