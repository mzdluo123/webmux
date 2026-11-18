import { animate } from "motion/mini";

// The caller supplies actual readiness; never use animation time as progress.
export function paneEnter(
  node: HTMLElement,
  shown: boolean,
  maxRevealHeight = 0,
) {
  const reduced = matchMedia("(prefers-reduced-motion: reduce)");
  let cleanup: (() => void) | undefined;
  let visible = false;
  function cancel() {
    cleanup?.();
    cleanup = undefined;
  }
  function update(next: boolean) {
    if (next === visible) return;
    visible = next;
    cancel();
    if (!next || reduced.matches) return;
    const { opacity, height, overflow } = node.style;
    const measured = maxRevealHeight ? node.getBoundingClientRect().height : 0;
    const expand = measured > 0 && measured <= maxRevealHeight;
    if (expand) node.style.overflow = "clip";
    const animation = animate(
      node,
      {
        opacity: [0, 1],
        ...(expand ? { height: ["0px", `${measured}px`] } : {}),
      },
      { duration: 0.2, ...(expand ? { ease: [0.22, 1, 0.36, 1] } : {}) },
    );
    const restore = () => {
      animation.cancel();
      node.style.opacity = opacity;
      if (expand) {
        node.style.height = height;
        node.style.overflow = overflow;
      }
    };
    cleanup = restore;
    void animation.then(() => {
      if (cleanup === restore) cancel();
    });
  }
  update(shown);
  reduced.addEventListener("change", cancel);
  if (maxRevealHeight) window.addEventListener("resize", cancel);
  return {
    update,
    destroy() {
      cancel();
      reduced.removeEventListener("change", cancel);
      if (maxRevealHeight) window.removeEventListener("resize", cancel);
    },
  };
}
