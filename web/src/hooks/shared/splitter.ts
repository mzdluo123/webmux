// WAI-ARIA window splitter: drag adjusts width. Arrow keys move it in fixed
// steps; Home and End take it to its bounds.

export function dragWidth(
  event: PointerEvent,
  width: number,
  set: (width: number) => void,
) {
  if (event.button !== 0) return;
  event.preventDefault();
  const handle = event.currentTarget as HTMLElement;
  handle.setPointerCapture(event.pointerId);
  const startX = event.clientX;
  const move = (e: PointerEvent) => set(width + e.clientX - startX);
  const end = () => {
    handle.removeEventListener("pointermove", move);
    handle.removeEventListener("pointerup", end);
    handle.removeEventListener("pointercancel", end);
  };
  handle.addEventListener("pointermove", move);
  handle.addEventListener("pointerup", end);
  handle.addEventListener("pointercancel", end);
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
