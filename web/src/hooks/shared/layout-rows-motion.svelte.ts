import { animate } from "motion/mini";
import { onMount, tick } from "svelte";

type Row = { node: HTMLElement; key: string; box: DOMRect };

function visibleRows(container: HTMLElement, selector: string): Row[] {
  const clip = container
    .closest('[data-slot="scroll-area-viewport"]')
    ?.getBoundingClientRect();
  const top = Math.max(0, clip?.top ?? 0);
  const bottom = Math.min(
    window.innerHeight,
    clip?.bottom ?? window.innerHeight,
  );
  const rows: Row[] = [];
  for (const node of container.querySelectorAll<HTMLElement>(selector)) {
    const box = node.getBoundingClientRect();
    if (!box.width || !box.height || box.bottom <= top || box.top >= bottom)
      continue;
    const key =
      node.dataset.sessionId ??
      node.dataset.sidebarSessionId ??
      node.dataset.layoutRow;
    if (key) rows.push({ node, key, box });
  }
  return rows;
}

/** Move rows shared by two layouts from their previous viewport positions. */
export function useLayoutRowsMotion(
  element: () => HTMLElement | null,
  layout: () => string,
  selector: string,
) {
  const reduced = () => matchMedia("(prefers-reduced-motion: reduce)");
  let previous: string | undefined;
  let generation = 0;
  let finish: (() => void) | undefined;

  function cancel() {
    generation++;
    finish?.();
    finish = undefined;
  }

  $effect.pre(() => {
    const node = element();
    const next = layout();
    if (!node) return;
    const changed = Boolean(previous && next && previous !== next);
    previous = next;
    if (!changed) {
      if (!next) cancel();
      return;
    }
    if (reduced().matches || !node.getClientRects().length) return cancel();
    const from = visibleRows(node, selector);
    cancel();
    const current = generation;
    void tick().then(() => {
      if (current !== generation || !node.isConnected || reduced().matches)
        return;
      const rows = visibleRows(node, selector);
      const cleanups: (() => void)[] = [];
      const targets = rows.length
        ? rows
        : [{ node, key: "", box: node.getBoundingClientRect() }];
      const controls: ReturnType<typeof animate>[] = [];
      for (const [index, { node: target, key, box }] of targets.entries()) {
        const before = from.find((row) => row.key === key)?.box;
        const x = before ? before.left - box.left : 0;
        const y = before ? before.top - box.top : 7;
        if (before && Math.abs(x) < 1 && Math.abs(y) < 1) continue;
        const { transform, opacity } = target.style;
        const animation = animate(
          target,
          {
            transform: [`translate(${x}px, ${y}px)`, "translate(0px, 0px)"],
            ...(before ? {} : { opacity: [0.35, 1] }),
          },
          {
            duration: before ? 0.32 : 0.22,
            delay: before ? 0 : Math.min(index, 4) * 0.012,
            ease: [0.22, 1, 0.36, 1],
          },
        );
        cleanups.push(() => {
          animation.cancel();
          target.style.transform = transform;
          target.style.opacity = opacity;
        });
        controls.push(animation);
      }
      finish = () => cleanups.forEach((cleanup) => cleanup());
      void Promise.all(controls).then(() => {
        if (current === generation) cancel();
      });
    });
  });

  onMount(() => {
    const media = reduced();
    media.addEventListener("change", cancel);
    window.addEventListener("resize", cancel);
    return () => {
      cancel();
      media.removeEventListener("change", cancel);
      window.removeEventListener("resize", cancel);
    };
  });
}
