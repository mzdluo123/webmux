import { animate } from "motion/mini";
import { onMount, tick } from "svelte";

export function useManualDisclosureMotion(
  element: () => HTMLDivElement | null,
  open: () => boolean,
) {
  let moving = $state(false);
  let generation = 0;
  let expectedOpen: boolean | undefined;
  let animation: ReturnType<typeof animate> | undefined;
  const motionQuery = "(prefers-reduced-motion: reduce)";

  function settle() {
    generation++;
    animation?.cancel();
    animation = undefined;
    element()?.style.removeProperty("height");
    moving = false;
  }

  // The search can open groups without using the trigger.
  $effect.pre(() => {
    if (open() !== expectedOpen) settle();
  });

  function toggle(next: boolean, apply: (value: boolean) => void) {
    const panel = element();
    const from = panel?.getBoundingClientRect().height ?? 0;
    settle();
    expectedOpen = next;
    apply(next);
    if (!panel || matchMedia(motionQuery).matches) return;
    moving = true;
    const version = generation;
    void tick().then(() => {
      if (version !== generation || !panel.isConnected) return;
      animation = animate(
        panel,
        { height: [`${from}px`, `${next ? panel.scrollHeight : 0}px`] },
        { duration: 0.24, ease: [0.22, 1, 0.36, 1] },
      );
      void animation.then(() => {
        if (version === generation) settle();
      });
    });
  }

  onMount(() => {
    const media = matchMedia(motionQuery);
    media.addEventListener("change", settle);
    window.addEventListener("resize", settle);
    return () => {
      media.removeEventListener("change", settle);
      window.removeEventListener("resize", settle);
      settle();
    };
  });
  return {
    get moving() {
      return moving;
    },
    toggle,
  };
}
