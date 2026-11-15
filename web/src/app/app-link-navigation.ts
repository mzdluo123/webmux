import { matchRoute } from "../hooks/shared/route";
import { navigate } from "../platform/api";

export interface AppLinkNavigationTarget {
  sessionId: () => string;
  activateSession: (id: string) => boolean;
  reopenSession: (id: string) => boolean;
  setPendingSessionOpen: (id: string) => void;
}

// Plain anchors to app routes would reload the whole document, which blanks
// an iOS standalone PWA; route them through history instead.
export function createAppLinkHandler(target: AppLinkNavigationTarget) {
  return (event: MouseEvent) => {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    )
      return;
    const anchor =
      event.target instanceof Element ? event.target.closest("a") : null;
    if (!anchor || anchor.hasAttribute("download")) return;
    if (anchor.target && anchor.target !== "_self") return;
    const url = new URL(anchor.href, location.href);
    if (url.origin !== location.origin) return;
    const targetRoute = matchRoute(url.pathname);
    const appRoute = url.pathname === "/" || targetRoute.name !== "sessions";
    if (!appRoute) return;
    // References to an open split pane select it without replacing the
    // layout, including when its ID is the current route's anchor.
    if (targetRoute.name === "terminal" && !url.search && !url.hash) {
      event.preventDefault();
      if (target.activateSession(targetRoute.id)) return;
      if (
        targetRoute.id === target.sessionId() &&
        target.reopenSession(targetRoute.id)
      ) {
        return;
      }
      target.setPendingSessionOpen(targetRoute.id);
      navigate(url.pathname);
      return;
    }
    if (url.pathname === location.pathname && url.search === location.search)
      return;
    event.preventDefault();
    navigate(url.pathname + url.search + url.hash);
  };
}
