import { createSubscriber } from "svelte/reactivity";
import { ThemeSchema, type Theme } from "../../contracts/schemas";
import { resolveStorageKey, store, stored } from "../../platform/storage";

type ThemePreference = "system" | Theme;

const STORAGE_KEY = resolveStorageKey("theme");

function readStoredTheme(): Theme | undefined {
  if (typeof window === "undefined") return undefined;
  const raw = stored("theme");
  return raw ? ThemeSchema.safeParse(raw).data : undefined;
}

function systemPrefersLight(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-color-scheme: light)").matches
  );
}

function reducedMotionPreferred(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

const themeSubscribers = new Set<() => void>();
const trackTheme = createSubscriber((update) => {
  themeSubscribers.add(update);
  return () => {
    themeSubscribers.delete(update);
  };
});

let choice: Theme | undefined = readStoredTheme();
let transition: ViewTransition | undefined;
let pendingTheme: Theme | undefined;
let initialized = false;

function applyTheme(): void {
  if (typeof document === "undefined") return;
  const dark = choice ? choice === "dark" : !systemPrefersLight();
  const root = document.documentElement;
  root.classList.toggle("dark", dark);
  const meta = document.querySelector<HTMLMetaElement>(
    'meta[name="theme-color"]',
  );
  if (meta) {
    meta.content = getComputedStyle(root)
      .getPropertyValue("--background")
      .trim();
  }
}

function refreshTheme(): void {
  const next = readStoredTheme();
  const changed = next !== choice;
  choice = next;
  applyTheme();
  if (changed) {
    for (const notify of themeSubscribers) notify();
  }
}

function setThemePreference(next: ThemePreference): void {
  const nextChoice = next === "system" ? undefined : next;
  store("theme", nextChoice ?? null);
  const changed = nextChoice !== choice;
  choice = nextChoice;
  applyTheme();
  if (changed) {
    for (const notify of themeSubscribers) notify();
  }
}

export const themePreference = {
  get value(): ThemePreference {
    trackTheme();
    return choice ?? "system";
  },
  set: setThemePreference,
  refresh: refreshTheme,
};

export function initializeTheme(): void {
  if (initialized || typeof window === "undefined") return;
  initialized = true;
  refreshTheme();
  if (typeof window.matchMedia === "function") {
    const system = window.matchMedia("(prefers-color-scheme: light)");
    system.addEventListener("change", () => {
      if (!choice) applyTheme();
    });
  }
  window.addEventListener("storage", (event) => {
    if (event.key === null || event.key === STORAGE_KEY) {
      refreshTheme();
    }
  });
}

function commitTheme(): void {
  if (!pendingTheme) return;
  const theme = pendingTheme;
  pendingTheme = undefined;
  setThemePreference(theme);
}

export function toggleTheme(event: MouseEvent): void {
  const root = document.documentElement;
  // Another click can arrive before the browser runs the first update.
  const dark = pendingTheme
    ? pendingTheme === "dark"
    : root.classList.contains("dark");
  pendingTheme = dark ? "light" : "dark";

  const button = event.currentTarget;
  if (
    transition ||
    !document.startViewTransition ||
    reducedMotionPreferred() ||
    !(button instanceof HTMLElement)
  ) {
    transition?.skipTransition();
    commitTheme();
    return;
  }

  const { left, top, width, height } = button.getBoundingClientRect();
  const x = left + width / 2;
  const y = top + height / 2;
  const radius = Math.hypot(
    Math.max(x, innerWidth - x),
    Math.max(y, innerHeight - y),
  );
  root.style.setProperty("--theme-reveal-origin", `${x}px ${y}px`);
  root.style.setProperty("--theme-reveal-radius", `${radius}px`);
  root.dataset.themeTransition = "";
  transition = document.startViewTransition(commitTheme);
  // A skipped transition still runs the update callback, but its ready promise rejects.
  void transition.ready.catch(() => undefined);
  const cleanup = () => {
    transition = undefined;
    delete root.dataset.themeTransition;
    root.style.removeProperty("--theme-reveal-origin");
    root.style.removeProperty("--theme-reveal-radius");
  };
  void transition.finished.then(cleanup, cleanup);
}
