export type SpatialDirection =
  | "ArrowUp"
  | "ArrowDown"
  | "ArrowLeft"
  | "ArrowRight";

interface SpatialRect {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

/** Stable geometric ordering, favoring candidates in the same visual lane. */
export function spatialCandidate<T>(
  current: SpatialRect,
  candidates: readonly { target: T; rect: SpatialRect }[],
  direction: SpatialDirection,
): T | undefined {
  const horizontal = direction === "ArrowLeft" || direction === "ArrowRight";
  const sign = direction === "ArrowLeft" || direction === "ArrowUp" ? -1 : 1;
  const x = (current.left + current.right) / 2;
  const y = (current.top + current.bottom) / 2;
  let best: T | undefined;
  let score = Infinity;
  for (const { target, rect } of candidates) {
    const dx = (rect.left + rect.right) / 2 - x;
    const dy = (rect.top + rect.bottom) / 2 - y;
    const forward = sign * (horizontal ? dx : dy);
    if (forward <= 1) continue;
    const gap = horizontal
      ? sign > 0
        ? rect.left - current.right
        : current.left - rect.right
      : sign > 0
        ? rect.top - current.bottom
        : current.top - rect.bottom;
    const offset = horizontal
      ? Math.max(0, rect.top - current.bottom, current.top - rect.bottom)
      : Math.max(0, rect.left - current.right, current.left - rect.right);
    const next = Math.hypot(Math.max(0, gap), offset) + offset * 2;
    if (next < score) {
      score = next;
      best = target;
    }
  }
  return best;
}
