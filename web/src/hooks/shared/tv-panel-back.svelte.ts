import { tvMode } from "./tv-mode.svelte";

/** Each visible panel consumes Back only for its own focused local layer. */
export function useTvPanelBack(options: {
  host(): HTMLElement | undefined;
  hasLayer?(): boolean;
  closeLayer(): boolean | void;
}) {
  $effect(() => {
    if (!tvMode.enabled) return;
    function back(event: Event) {
      if (event.defaultPrevented || (event as CustomEvent).detail !== "panel")
        return;
      const host = options.host();
      if (!host || !host.getClientRects().length) return;
      const active = document.activeElement;
      if (active && active !== document.body && !host.contains(active)) return;
      if (options.hasLayer && !options.hasLayer()) return;
      if (options.closeLayer() !== false) event.preventDefault();
    }
    window.addEventListener("webmux:tv-back", back);
    return () => window.removeEventListener("webmux:tv-back", back);
  });
}
