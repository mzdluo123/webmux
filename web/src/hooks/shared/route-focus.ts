import { onMount, tick } from "svelte";

/** Focuses newly rendered routes without stealing focus from their own inputs. */
export function useRouteFocus() {
  let focusRoute = false;
  const focusMain = () => {
    if (!focusRoute) return;
    const main = document.querySelector("main");
    if (!main) return;
    focusRoute = false;
    // A page that already placed focus (quick start's input) keeps it.
    if (main.contains(document.activeElement)) return;
    main.tabIndex = -1;
    main.focus({ preventScroll: true });
  };

  onMount(() => {
    const routes = new MutationObserver(focusMain);
    routes.observe(document.getElementById("app")!, {
      childList: true,
      subtree: true,
    });
    return () => routes.disconnect();
  });

  return () => {
    focusRoute = true;
    void tick().then(focusMain);
  };
}
