import { onMount, tick } from "svelte";
import { shellUi } from "./shell.svelte";

/** Focuses newly rendered routes without stealing focus from their own inputs. */
export function useRouteFocus() {
  let pending: string | undefined;
  const focusTarget = () => {
    if (!pending) return;
    const target = document.querySelector<HTMLElement>(pending);
    if (!target) return;
    const main = pending === "main";
    const active = document.activeElement;
    // Keep a page's input autofocus or focus the user moved as it loaded.
    if (
      main
        ? target.contains(active)
        : active &&
          active !== document.body &&
          !active.closest('[data-slot="drawer-content"]')
    ) {
      pending = undefined;
      return;
    }
    if (main) target.tabIndex = -1;
    target.focus({ preventScroll: true });
    if (main || document.activeElement === target) pending = undefined;
  };

  onMount(() => {
    const routes = new MutationObserver(focusTarget);
    routes.observe(document.getElementById("app")!, {
      childList: true,
      subtree: true,
    });
    return () => routes.disconnect();
  });

  return (target: "main" | "sidebar-opener" = "main") => {
    // The open drawer owns focus while changing the route underneath it.
    if (shellUi.drawerOpen) return;
    pending =
      target === "main" ? "main" : ".app-shell-main [data-sidebar-opener]";
    void tick().then(focusTarget);
  };
}
