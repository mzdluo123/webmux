import { dragWidth, keyWidth } from "../shared/splitter";
export const navMin = 120,
  navMax = 640;
export function useNavigationResize(
  container: () => HTMLElement | undefined,
  getDivider: () => HTMLElement | undefined,
  width: { get(): number; set(value: number): void },
) {
  function setNavWidth(value: number) {
    const limit = Math.min(navMax, (container()?.clientWidth ?? navMax) - 160);
    width.set(Math.round(Math.max(navMin, Math.min(limit, value))));
  }
  function resizeStart(event: PointerEvent) {
    dragWidth(event, renderedNavWidth(), setNavWidth);
  }
  function resizeKey(event: KeyboardEvent) {
    keyWidth(
      event,
      renderedNavWidth(),
      { min: navMin, max: navMax },
      setNavWidth,
    );
  }
  $effect(() => {
    const divider = getDivider();
    const host = container();
    if (!divider || !host) return;

    let ownedFocus = document.activeElement === divider;
    let overlayFocus: HTMLElement | undefined;
    function remember() {
      ownedFocus = true;
    }
    const clear = (event: FocusEvent) => {
      if (event.relatedTarget || divider.getClientRects().length)
        ownedFocus = false;
    };
    const update = () => {
      if (divider.getClientRects().length) {
        if (overlayFocus && document.activeElement === overlayFocus)
          divider.focus({ preventScroll: true });
        overlayFocus = undefined;
      } else if (
        ownedFocus &&
        (document.activeElement === divider ||
          document.activeElement === document.body)
      ) {
        const tab = host.querySelector<HTMLElement>(
          '.code-navigation [role="tab"][aria-selected="true"]',
        );
        if (tab?.getClientRects().length) {
          overlayFocus = tab;
          tab.focus({ preventScroll: true });
        }
        ownedFocus = false;
      }
    };
    const resize = new ResizeObserver(update);
    resize.observe(host);
    window.addEventListener("resize", update);
    divider.addEventListener("focus", remember);
    divider.addEventListener("blur", clear);
    return () => {
      resize.disconnect();
      window.removeEventListener("resize", update);
      divider.removeEventListener("focus", remember);
      divider.removeEventListener("blur", clear);
    };
  });

  function renderedNavWidth() {
    return (
      container()?.querySelector(".code-navigation")?.getBoundingClientRect()
        .width || width.get()
    );
  }
  return { resizeStart, resizeKey };
}
