import { createSubscriber } from "svelte/reactivity";
import { overwriteGetLocale, overwriteSetLocale } from "./paraglide/runtime.js";

export type SupportedLocale = "zh-CN" | "en";
export type LocalePreference = "system" | SupportedLocale;

const STORAGE_KEY = "webmux.locale";

/**
 * Resolves a browser language preference list to a supported UI locale.
 * Non-Chinese explicit tags fall back to English; an empty list uses zh-CN.
 */
export function resolveSystemLocale(
  languages?: readonly string[],
): SupportedLocale {
  const candidates =
    languages ??
    (typeof navigator !== "undefined"
      ? navigator.languages?.length
        ? navigator.languages
        : navigator.language
          ? [navigator.language]
          : []
      : []);
  let sawCandidate = false;
  for (const entry of candidates) {
    const tag = entry.trim().toLowerCase();
    if (!tag) continue;
    sawCandidate = true;
    if (tag === "zh" || tag.startsWith("zh-") || tag.startsWith("zh_")) {
      return "zh-CN";
    }
    if (tag === "en" || tag.startsWith("en-") || tag.startsWith("en_")) {
      return "en";
    }
  }
  return sawCandidate ? "en" : "zh-CN";
}

function hasBrowserStorage(): boolean {
  return typeof window !== "undefined" && typeof localStorage !== "undefined";
}

function readStoredPreference(): LocalePreference {
  if (!hasBrowserStorage()) return "system";
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw === "zh-CN" || raw === "en") return raw;
  } catch {
    // Private-browsing or disabled storage keeps the system locale usable.
  }
  return "system";
}

function writeStoredPreference(next: LocalePreference): void {
  if (!hasBrowserStorage()) return;
  try {
    if (next === "system") localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // Private-browsing storage is optional; reactive state still updates.
  }
}

function syncDocumentLang(value: SupportedLocale): void {
  if (typeof document !== "undefined") {
    document.documentElement.lang = value;
  }
}

const listeners = new Set<() => void>();
const subscribe = createSubscriber((update) => {
  listeners.add(update);
  return () => {
    listeners.delete(update);
  };
});

function notify(): void {
  for (const listener of listeners) listener();
}

let preferenceState: LocalePreference = readStoredPreference();
let systemState: SupportedLocale = resolveSystemLocale();

function effectiveLocale(): SupportedLocale {
  return preferenceState === "system" ? systemState : preferenceState;
}

function applyState(
  nextPreference: LocalePreference,
  nextSystem: SupportedLocale,
): void {
  const changed =
    nextPreference !== preferenceState || nextSystem !== systemState;
  preferenceState = nextPreference;
  systemState = nextSystem;
  syncDocumentLang(effectiveLocale());
  if (changed) notify();
}

function setPreference(next: LocalePreference): void {
  writeStoredPreference(next);
  applyState(next, next === "system" ? resolveSystemLocale() : systemState);
}

function refreshFromEnvironment(): void {
  applyState(readStoredPreference(), resolveSystemLocale());
}

overwriteGetLocale(() => {
  subscribe();
  return effectiveLocale();
});
overwriteSetLocale((next) => {
  if (next === "zh-CN" || next === "en") setPreference(next);
});
syncDocumentLang(effectiveLocale());

let initialized = false;

export function initializeLocale(): void {
  refreshFromEnvironment();
  if (initialized || typeof window === "undefined") return;
  initialized = true;
  window.addEventListener("languagechange", () => {
    applyState(preferenceState, resolveSystemLocale());
  });
  window.addEventListener("storage", (event) => {
    if (event.key === null || event.key === STORAGE_KEY) {
      refreshFromEnvironment();
    }
  });
}

export const locale = {
  get preference(): LocalePreference {
    subscribe();
    return preferenceState;
  },
  get current(): SupportedLocale {
    subscribe();
    return effectiveLocale();
  },
  setPreference,
  refresh: refreshFromEnvironment,
};
