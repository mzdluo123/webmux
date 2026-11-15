import { tvMode } from "../shared/tv-mode.svelte";

/** Each pane consumes only events belonging to its currently focused surface. */
export function useTvTerminalBack(options: {
  host(): HTMLElement | undefined;
  codeShown(): boolean;
  closeCode(): void;
  leave(): void;
}) {
  $effect(() => {
    if (!tvMode.enabled) return;
    function back(event: Event) {
      if (event.defaultPrevented) return;
      if (
        !options
          .host()
          ?.closest(".session-terminal")
          ?.contains(document.activeElement)
      )
        return;
      const phase = (event as CustomEvent).detail;
      if (phase === "panel" && options.codeShown()) {
        event.preventDefault();
        options.closeCode();
      } else if (phase === "route") {
        event.preventDefault();
        options.leave();
      }
    }
    window.addEventListener("webmux:tv-back", back);
    return () => window.removeEventListener("webmux:tv-back", back);
  });
}
