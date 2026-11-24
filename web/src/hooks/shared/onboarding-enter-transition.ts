import { tick } from "svelte";
import { animate } from "motion/mini";

const EASE_OUT_EXPO: [number, number, number, number] = [0.16, 1, 0.3, 1];
const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

let activeTransition: ViewTransition | undefined;

function reducedMotionPreferred(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia(REDUCED_MOTION_QUERY).matches
  );
}

function isVisibleElement(node: Element | null): node is HTMLElement {
  if (!(node instanceof HTMLElement)) return false;
  const rect = node.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}

function findVisibleSidebar(): HTMLElement | null {
  const full = document.querySelector(".app-shell .app-sidebar-full");
  if (isVisibleElement(full)) return full;
  const rail = document.querySelector(".app-shell .app-sidebar-rail");
  if (isVisibleElement(rail)) return rail;
  return null;
}

function findVisibleSidebarBrand(): HTMLElement | null {
  const mark = document.querySelector(
    ".app-shell .app-sidebar-full .sidebar-header .brand-mark",
  );
  return isVisibleElement(mark) ? mark : null;
}

function findVisibleMain(): HTMLElement | null {
  const main = document.querySelector(".app-shell .app-shell-main");
  return isVisibleElement(main) ? main : null;
}

function runFallbackEntrance(): void {
  const sidebar = findVisibleSidebar();
  const main = findVisibleMain();
  const targets: Array<{
    node: HTMLElement;
    animation: ReturnType<typeof animate>;
    opacity: string;
    transform: string;
  }> = [];

  if (sidebar) {
    const { opacity, transform } = sidebar.style;
    targets.push({
      node: sidebar,
      opacity,
      transform,
      animation: animate(
        sidebar,
        { opacity: [0, 1], transform: ["translateX(-16px)", "translateX(0)"] },
        { duration: 0.42, ease: EASE_OUT_EXPO },
      ),
    });
  }

  if (main) {
    const { opacity, transform } = main.style;
    targets.push({
      node: main,
      opacity,
      transform,
      animation: animate(
        main,
        { opacity: [0, 1], transform: ["translateY(14px)", "translateY(0)"] },
        { duration: 0.46, delay: 0.05, ease: EASE_OUT_EXPO },
      ),
    });
  }

  if (targets.length === 0) return;

  const media =
    typeof window.matchMedia === "function"
      ? window.matchMedia(REDUCED_MOTION_QUERY)
      : undefined;
  let cleaned = false;
  const cleanup = () => {
    if (cleaned) return;
    cleaned = true;
    media?.removeEventListener("change", cleanup);
    for (const item of targets) {
      item.animation.cancel();
      item.node.style.opacity = item.opacity;
      item.node.style.transform = item.transform;
    }
  };
  media?.addEventListener("change", cleanup);
  void Promise.all(
    targets.map((item) => item.animation.then(() => undefined)),
  ).then(cleanup, cleanup);
}

/**
 * Choreographs the transition from the centered onboarding card into the main
 * WebMux workbench: recedes the card, glides the brand mark into the sidebar
 * header when visible, and brings the sidebar and main workspace in smoothly.
 */
export async function runOnboardingEnterTransition(
  commit: () => void,
): Promise<void> {
  if (typeof document === "undefined" || reducedMotionPreferred()) {
    commit();
    return;
  }

  if (!document.startViewTransition) {
    commit();
    await tick();
    runFallbackEntrance();
    return;
  }

  activeTransition?.skipTransition();
  const root = document.documentElement;
  const named: HTMLElement[] = [];
  const assignName = (node: HTMLElement | null, name: string) => {
    if (!node) return;
    node.style.viewTransitionName = name;
    named.push(node);
  };
  const clearNames = () => {
    for (const node of named) {
      node.style.removeProperty("view-transition-name");
    }
    named.length = 0;
  };

  const oldCard = document.querySelector("[data-auth-gate-card]");
  const oldBrand = document.querySelector("[data-auth-gate-brand] .brand-mark");
  assignName(isVisibleElement(oldCard) ? oldCard : null, "onboarding-card");
  assignName(isVisibleElement(oldBrand) ? oldBrand : null, "onboarding-brand");

  root.dataset.onboardingTransition = "";
  const transition = document.startViewTransition(async () => {
    clearNames();
    commit();
    await tick();
    assignName(findVisibleSidebar(), "onboarding-sidebar");
    assignName(findVisibleMain(), "onboarding-main");
    assignName(findVisibleSidebarBrand(), "onboarding-brand");
  });
  activeTransition = transition;
  // A skipped transition still runs the update callback, but its ready promise rejects.
  void transition.ready.catch(() => undefined);

  const cleanup = () => {
    if (activeTransition === transition) activeTransition = undefined;
    delete root.dataset.onboardingTransition;
    clearNames();
  };
  void transition.finished.then(cleanup, cleanup);

  try {
    await transition.updateCallbackDone;
  } catch {
    cleanup();
  }
}
