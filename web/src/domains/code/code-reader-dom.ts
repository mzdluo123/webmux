import { getFiletypeFromFileName, type CodeView } from "@pierre/diffs";
import type { CodeReaderState } from "../../contracts/schemas";

export function resolvePierreFile(path: string, contents: string) {
  const name = path.split(/[\\/]/).pop() || path;
  const inferred = getFiletypeFromFileName(name);
  // Pierre's filename lookup is case-sensitive; retry lowercase extensions
  // as the previous reader did (`README.MD`, `MAIN.GO`).
  const lang =
    inferred === "text"
      ? getFiletypeFromFileName(name.toLowerCase())
      : inferred;
  return {
    name,
    contents,
    // The old reader highlighted Svelte as JavaScript; keep that reading.
    lang: /\.svelte$/i.test(name) ? "javascript" : lang,
  };
}

// The first visible line and how far (0-1 of its height) it is scrolled
// past. A scroll event fires before the renderer has painted its rows, so
// derive it from the numeric line layout instead of the stale DOM.
export function measureFilePosition(
  viewer: CodeView | undefined,
  host: HTMLElement,
  id: string,
  lineCount: number,
  fallback: () => Pick<CodeReaderState, "line" | "offset">,
): Pick<CodeReaderState, "line" | "offset"> {
  const clampLine = (v: number) => Math.max(1, Math.min(lineCount, v));
  if (!viewer || host.clientHeight === 0) return fallback();
  const item = viewer.getRenderedItems()[0];
  if (!item) return fallback();
  const itemTop = viewer.getTopForItem(id);
  if (itemTop !== undefined) {
    const local = viewer.getScrollTop() - itemTop;
    let low = 1;
    let high = lineCount;
    let block: { top: number; height: number } | undefined;
    let found = 0;
    while (low <= high) {
      const middle = Math.floor((low + high) / 2);
      const candidate = item.instance.getLinePosition(middle);
      if (!candidate) break;
      if (candidate.top <= local) {
        block = candidate;
        found = middle;
        low = middle + 1;
      } else {
        high = middle - 1;
      }
    }
    if (block) {
      return {
        line: clampLine(found),
        offset:
          block.height > 0
            ? Math.min(1, Math.max(0, (local - block.top) / block.height))
            : 0,
      };
    }
  }
  const edge = host.getBoundingClientRect().top;
  for (const row of item.element.shadowRoot?.querySelectorAll<HTMLElement>(
    "[data-line]",
  ) ?? []) {
    const line = Number(row.dataset.line);
    if (!Number.isInteger(line) || line < 1) continue;
    const rect = row.getBoundingClientRect();
    if (rect.bottom <= edge + 0.5) continue;
    const offset =
      rect.height > 0
        ? Math.min(1, Math.max(0, (edge - rect.top) / rect.height))
        : 0;
    return { line, offset };
  }
  return fallback();
}

export function scrollFileToColumn(
  viewer: () => CodeView | undefined,
  targetLine: number,
  column: number,
  scrollTo: (left: number) => void,
) {
  let attempts = 0;
  // CodeView queues its render on a frame, so the target row usually only
  // exists after the scroll request has painted.
  const apply = () => {
    const row = viewer()
      ?.getRenderedItems()[0]
      ?.element.shadowRoot?.querySelector<HTMLElement>(
        `[data-line="${targetLine}"]`,
      );
    const scroller = row?.closest<HTMLElement>("[data-code]");
    if (!row || !scroller) {
      if (++attempts < 20) requestAnimationFrame(apply);
      return;
    }
    const length = row.textContent?.length ?? 0;
    if (!length) return;
    const rect = row.getBoundingClientRect();
    const box = scroller.getBoundingClientRect();
    const x =
      rect.left +
      (rect.width * Math.min(Math.max(column - 1, 0), length)) / length;
    scrollTo(scroller.scrollLeft + x - box.left);
  };
  requestAnimationFrame(apply);
}

export function parseFileGoto(
  value: string,
  current: number,
  totalLines: number,
): { target: number; column?: number } | undefined {
  const match = /^([+-])?(\d+)?(:\d+)?(%)?$/.exec(value.trim());
  if (!match) return undefined;
  const [, sign, digits, column, percent] = match;
  let target = digits ? Number(digits) : current;
  if (digits && percent) {
    let fraction = target / 100;
    if (sign)
      fraction = fraction * (sign === "-" ? -1 : 1) + current / totalLines;
    target = Math.round(totalLines * fraction);
  } else if (digits && sign)
    target = current + target * (sign === "-" ? -1 : 1);
  return {
    target,
    column: column ? Number(column.slice(1)) : undefined,
  };
}

export type VisibleRowBand = {
  top: number;
  bottom: number;
};

/**
 * Measures the bounding vertical band of selected or focused rows inside `host`
 * that are currently visible in `box`.
 */
export function measureVisibleSelectedBand(
  host: HTMLElement,
  box: DOMRect,
): VisibleRowBand | undefined {
  const shadow = host.querySelector("diffs-container")?.shadowRoot;
  if (!shadow || box.height <= 0) return undefined;
  const elements = [
    ...shadow.querySelectorAll<HTMLElement>("[data-line][data-selected-line]"),
  ];
  if (elements.length === 0 && shadow.activeElement instanceof HTMLElement) {
    elements.push(shadow.activeElement);
  }
  let top = Number.POSITIVE_INFINITY;
  let bottom = Number.NEGATIVE_INFINITY;
  for (const element of elements) {
    const rect = element.getBoundingClientRect();
    if (rect.height <= 0) continue;
    if (rect.bottom <= box.top + 0.5 || rect.top >= box.bottom - 0.5) continue;
    if (rect.top < top) top = rect.top;
    if (rect.bottom > bottom) bottom = rect.bottom;
  }
  return top <= bottom ? { top, bottom } : undefined;
}

/**
 * Scrolls `host` down by the overlap when a newly mounted bottom panel shrinks
 * `host` over `band`, keeping upper lines anchored when `band` stays clear.
 */
export function nudgeHostForBottomPanel(
  host: HTMLElement,
  beforeBox: DOMRect,
  band: VisibleRowBand,
) {
  const afterBox = host.getBoundingClientRect();
  if (afterBox.height <= 0 || afterBox.bottom >= beforeBox.bottom - 0.5) return;
  const overlap = Math.min(band.bottom, beforeBox.bottom) - afterBox.bottom;
  const headroom = band.top - afterBox.top;
  const delta = Math.min(Math.ceil(overlap), Math.floor(headroom));
  if (delta > 0) host.scrollTop += delta;
}
