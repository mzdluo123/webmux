import type { CodePdfState } from "../../contracts/schemas";

export function syncKeptPdfPages(
  visible: ReadonlySet<number>,
  kept: Set<number>,
  pageCount: number,
  renderAhead: number,
  releaseBeyond: number,
) {
  const first = Math.min(...visible);
  const last = Math.max(...visible);
  for (
    let number = Math.max(1, first - renderAhead);
    number <= Math.min(pageCount, last + renderAhead);
    number++
  )
    kept.add(number);
  for (const number of kept)
    if (number < first - releaseBeyond || number > last + releaseBeyond)
      kept.delete(number);
}

export function scrollPdfToPage(
  scroller: HTMLElement | undefined,
  pageCount: number,
  number: number,
  padding: number,
): { page: number; top: number } | undefined {
  if (!scroller || !Number.isInteger(number)) return undefined;
  const target = Math.min(pageCount, Math.max(1, number));
  const element = scroller.querySelector<HTMLElement>(
    `[data-pdf-page="${target}"]`,
  );
  if (!element) return undefined;
  scroller.scrollTop = element.offsetTop - padding;
  return { page: target, top: scroller.scrollTop };
}

export function scrollRestoredPdfPage(
  scroller: HTMLElement,
  state: CodePdfState,
  pageCount: number,
  padding: number,
) {
  const target = scroller.querySelector<HTMLElement>(
    `[data-pdf-page="${Math.min(state.page, pageCount)}"]`,
  );
  if (target)
    scroller.scrollTop =
      target.offsetTop - padding + state.offset * target.offsetHeight;
}
