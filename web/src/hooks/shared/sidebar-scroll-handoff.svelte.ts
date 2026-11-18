export function useSidebarScrollHandoff(
  drawer: () => HTMLElement | null,
  list: () => HTMLElement | null,
) {
  $effect(() => {
    const viewport = list();
    const outer = drawer();
    if (!viewport || !outer) return;
    // Keep the list's scroll offset when the short drawer hands scrolling
    // over to the outer nav. The other scroller may already reset to zero.
    const short = matchMedia("(width < 40rem) and (height <= 600px)");
    const currentScroller = () => (short.matches ? outer : viewport);
    let offset = currentScroller().scrollTop;
    const remember = (event: Event) => {
      const scroller = currentScroller();
      if (event.currentTarget === scroller) offset = scroller.scrollTop;
    };
    const restore = () => {
      currentScroller().scrollTop = offset;
    };
    outer.addEventListener("scroll", remember);
    viewport.addEventListener("scroll", remember);
    short.addEventListener("change", restore);
    return () => {
      outer.removeEventListener("scroll", remember);
      viewport.removeEventListener("scroll", remember);
      short.removeEventListener("change", restore);
    };
  });
}
