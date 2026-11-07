// Tracks whether a chat composer container has active focus (including focus
// moved into a portaled popover or dropdown menu opened from the composer) and
// whether it can collapse into its mobile single-line bar when idle.
export function useComposerCompact(
  getContainer: () => HTMLElement | null,
  isBusy: () => boolean,
) {
  let expanded = $state(false);
  const compact = $derived(!expanded && !isBusy());

  function hasOpenPortal(): boolean {
    return !!document.querySelector(
      '[data-slot="popover-content"][data-state="open"], [data-slot="dropdown-menu-content"][data-state="open"]',
    );
  }

  function checkFocus() {
    requestAnimationFrame(() => {
      const container = getContainer();
      if (container?.contains(document.activeElement) || hasOpenPortal()) {
        return;
      }
      expanded = false;
    });
  }

  $effect(() => {
    if (!expanded) return;
    function onPointerDown(event: PointerEvent) {
      const target = event.target as Node | null;
      const container = getContainer();
      if (target && container?.contains(target)) return;
      if (
        target instanceof Element &&
        target.closest(
          '[data-slot="popover-content"], [data-slot="dropdown-menu-content"]',
        )
      ) {
        return;
      }
      checkFocus();
    }
    window.addEventListener("pointerdown", onPointerDown, true);
    return () => window.removeEventListener("pointerdown", onPointerDown, true);
  });

  return {
    get compact() {
      return compact;
    },
    containerProps: {
      onfocusin: () => {
        expanded = true;
      },
      onfocusout: checkFocus,
    },
  };
}
