import type { DockRail, DockRect } from "$lib/dock-layout";

const overlay: Record<DockRail, DockRect> = {
  left: { x: 0, y: 0, width: 0.45, height: 1 },
  right: { x: 0.55, y: 0, width: 0.45, height: 1 },
  bottom: { x: 0, y: 0.55, width: 1, height: 0.45 },
};

export function useDockPeek(
  enabled: () => boolean,
  available: (id: string) => boolean,
  side: (id: string) => DockRail,
) {
  let id = $state("");
  let trigger: HTMLElement | undefined;
  function close(restore = false) {
    id = "";
    if (restore && trigger?.isConnected) trigger.focus();
    trigger = undefined;
  }
  $effect(() => {
    if (!id) return;
    if (!enabled() || !available(id)) {
      close();
      return;
    }
    function outside(event: Event) {
      if (!(event.target instanceof Element)) return;
      if (
        !event.target.closest(
          '[data-dock-peek], [data-dock-tab], [data-dock-command-menu], [data-dock-session-commands], [role="menu"], [role="dialog"], [role="alertdialog"], [role="listbox"]',
        )
      )
        close();
    }
    function escape(event: KeyboardEvent) {
      if (event.key === "Escape" && !event.defaultPrevented) {
        event.preventDefault();
        close(true);
      }
    }
    function terminalEscape(event: KeyboardEvent) {
      if (
        event.key !== "Escape" ||
        !(event.target instanceof Element) ||
        !event.target.closest("[data-dock-peek] .xterm")
      )
        return;
      event.stopPropagation();
      escape(event);
    }
    document.addEventListener("keydown", terminalEscape, true);
    document.addEventListener("pointerdown", outside, true);
    document.addEventListener("focusin", outside, true);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("keydown", terminalEscape, true);
      document.removeEventListener("pointerdown", outside, true);
      document.removeEventListener("focusin", outside, true);
      document.removeEventListener("keydown", escape);
    };
  });
  return {
    get id() {
      return id;
    },
    close,
    toggle(next: string, source: HTMLElement) {
      if (id === next) close();
      else {
        trigger = source;
        id = next;
      }
    },
    rect(panel: string) {
      return panel === id && enabled() ? overlay[side(panel)] : undefined;
    },
  };
}
