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

/**
 * Steps out of an inline composite (radiogroup/tablist/listbox) at its first or
 * last enabled item when a sibling control sits in the same visual lane.
 */
function compositeEdgeCandidate<T>(
  current: SpatialRect,
  groupRect: SpatialRect,
  atStart: boolean,
  atEnd: boolean,
  vertical: boolean,
  direction: SpatialDirection,
  candidates: readonly { target: T; rect: SpatialRect }[],
): T | undefined {
  const exitStart = vertical
    ? direction === "ArrowUp"
    : direction === "ArrowLeft";
  const exitEnd = vertical
    ? direction === "ArrowDown"
    : direction === "ArrowRight";
  if (!(atStart && exitStart) && !(atEnd && exitEnd)) return undefined;
  const aligned = candidates.filter(({ rect }) =>
    vertical
      ? rect.left < groupRect.right && rect.right > groupRect.left
      : rect.top < groupRect.bottom && rect.bottom > groupRect.top,
  );
  return spatialCandidate(current, aligned, direction);
}

export function compositeBoundaryTarget(
  group: HTMLElement,
  vertical: boolean,
  direction: SpatialDirection,
  overlays: string,
  visible: (element: HTMLElement) => boolean,
  candidates: (root: ParentNode) => HTMLElement[],
): HTMLElement | undefined {
  const active = document.activeElement;
  if (!(active instanceof HTMLElement) || !visible(active)) return undefined;
  const selector = '[role="tab"], [role="radio"], [role="option"]';
  const item = active.closest<HTMLElement>(selector);
  if (!item || !group.contains(item)) return undefined;
  const items = Array.from(
    group.querySelectorAll<HTMLElement>(selector),
  ).filter(
    (node) =>
      visible(node) && !node.matches(':disabled, [aria-disabled="true"]'),
  );
  if (!items.length) return undefined;
  const scope = active.closest<HTMLElement>(overlays) ?? document.body;
  const external = candidates(scope)
    .filter((target) => !group.contains(target))
    .map((target) => ({ target, rect: target.getBoundingClientRect() }));
  return compositeEdgeCandidate(
    active.getBoundingClientRect(),
    group.getBoundingClientRect(),
    item === items[0],
    item === items.at(-1),
    vertical,
    direction,
    external,
  );
}
