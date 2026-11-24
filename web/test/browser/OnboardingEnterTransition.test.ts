import "../../src/styles.css";
import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { runOnboardingEnterTransition } from "../../src/hooks/shared/onboarding-enter-transition";

let root: HTMLElement;

function mountOnboardingDom() {
  root.innerHTML = `
    <main>
      <div data-auth-gate-card style="width: 360px; height: 280px;">
        <p data-auth-gate-brand>
          <span class="brand-mark" style="display:block;width:28px;height:28px;"></span>
        </p>
      </div>
    </main>
  `;
}

function mountWorkbenchDom(showSidebar: boolean) {
  root.innerHTML = `
    <div class="app-shell">
      <aside class="app-sidebar app-sidebar-full" style="${showSidebar ? "width:240px;height:600px;" : "display:none;"}">
        <div class="sidebar-header">
          <span class="brand-mark" style="display:block;width:28px;height:28px;"></span>
        </div>
      </aside>
      <div class="app-shell-main" style="width:800px;height:600px;"></div>
    </div>
  `;
}

beforeEach(() => {
  root = document.createElement("div");
  document.body.append(root);
});

afterEach(() => {
  vi.restoreAllMocks();
  delete document.documentElement.dataset.onboardingTransition;
  root.remove();
});

test("assigns view-transition names for card, brand, sidebar and main, then cleans up", async () => {
  mountOnboardingDom();
  const oldCard = root.querySelector<HTMLElement>("[data-auth-gate-card]")!;
  const oldBrand = root.querySelector<HTMLElement>(
    "[data-auth-gate-brand] .brand-mark",
  )!;

  let oldNamesAtStart: Record<string, string> = {};
  let newNamesAtCommit: Record<string, string> = {};
  const finished = Promise.withResolvers<void>();

  const startViewTransition = vi
    .spyOn(document, "startViewTransition")
    .mockImplementation(((callback?: () => Promise<void> | void) => {
      oldNamesAtStart = {
        card: oldCard.style.viewTransitionName,
        brand: oldBrand.style.viewTransitionName,
      };
      const updateCallbackDone = Promise.resolve()
        .then(() => callback?.())
        .then(() => {
          const sidebar = root.querySelector<HTMLElement>(".app-sidebar-full")!;
          const main = root.querySelector<HTMLElement>(".app-shell-main")!;
          const brand = root.querySelector<HTMLElement>(
            ".sidebar-header .brand-mark",
          )!;
          newNamesAtCommit = {
            sidebar: sidebar.style.viewTransitionName,
            main: main.style.viewTransitionName,
            brand: brand.style.viewTransitionName,
          };
        });
      return {
        ready: Promise.resolve(),
        updateCallbackDone,
        finished: finished.promise,
        skipTransition: vi.fn(),
        types: new Set<string>(),
      } as unknown as ViewTransition;
    }) as typeof document.startViewTransition);

  await runOnboardingEnterTransition(() => {
    mountWorkbenchDom(true);
  });

  expect(startViewTransition).toHaveBeenCalledTimes(1);
  expect(oldNamesAtStart).toEqual({
    card: "onboarding-card",
    brand: "onboarding-brand",
  });
  expect(newNamesAtCommit).toEqual({
    sidebar: "onboarding-sidebar",
    main: "onboarding-main",
    brand: "onboarding-brand",
  });
  expect(
    document.documentElement.hasAttribute("data-onboarding-transition"),
  ).toBe(true);

  finished.resolve();
  await finished.promise;
  await Promise.resolve();

  expect(
    document.documentElement.hasAttribute("data-onboarding-transition"),
  ).toBe(false);
  expect(
    root.querySelector<HTMLElement>(".app-sidebar-full")?.style
      .viewTransitionName,
  ).toBe("");
  expect(
    root.querySelector<HTMLElement>(".app-shell-main")?.style
      .viewTransitionName,
  ).toBe("");
  expect(
    root.querySelector<HTMLElement>(".sidebar-header .brand-mark")?.style
      .viewTransitionName,
  ).toBe("");
});

test("skips transition when prefers-reduced-motion is enabled", async () => {
  mountOnboardingDom();
  const originalMatchMedia = window.matchMedia;
  vi.spyOn(window, "matchMedia").mockImplementation((query: string) => {
    if (query === "(prefers-reduced-motion: reduce)") {
      return {
        matches: true,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      } as unknown as MediaQueryList;
    }
    return originalMatchMedia.call(window, query);
  });
  const startViewTransition = vi.spyOn(document, "startViewTransition");

  let committed = false;
  await runOnboardingEnterTransition(() => {
    committed = true;
    mountWorkbenchDom(true);
  });

  expect(committed).toBe(true);
  expect(startViewTransition).not.toHaveBeenCalled();
  expect(
    document.documentElement.hasAttribute("data-onboarding-transition"),
  ).toBe(false);
});

test("commits cleanly when View Transitions API is unavailable", async () => {
  mountOnboardingDom();
  const original = Object.getOwnPropertyDescriptor(
    Document.prototype,
    "startViewTransition",
  );
  Object.defineProperty(document, "startViewTransition", {
    configurable: true,
    value: undefined,
  });

  try {
    let committed = false;
    await runOnboardingEnterTransition(() => {
      committed = true;
      mountWorkbenchDom(true);
    });

    expect(committed).toBe(true);
    expect(
      document.documentElement.hasAttribute("data-onboarding-transition"),
    ).toBe(false);
    expect(root.querySelector(".app-shell-main")).not.toBeNull();
  } finally {
    if (original) {
      Object.defineProperty(document, "startViewTransition", original);
    } else {
      delete (document as unknown as Record<string, unknown>)
        .startViewTransition;
    }
  }
});
