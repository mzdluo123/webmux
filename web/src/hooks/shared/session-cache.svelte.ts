import { untrack } from "svelte";
import { MediaQuery } from "svelte/reactivity";
import type { SessionInfo } from "../../contracts/schemas";
import {
  activateCachedSession,
  emptySessionCache,
  evictCachedSession,
  extractSessionActivity,
  observeCachedSessionActivity,
  resolveSessionCacheConfig,
  shrinkSessionCacheToActive,
  sweepSessionCache,
  takeDueBackgroundRefresh,
  type SessionCacheConfig,
  type SessionCacheEntry,
  type SessionCacheState,
  type SessionCacheTier,
} from "../../domains/sessions/session-cache";
import { recordSessionSwitch } from "../../platform/perf";
import { hasTouchInput } from "../../platform/touch-input";

const SWEEP_INTERVAL_MS = 15_000;
const REFRESH_POLL_MS = 1_500;
const REFRESH_WINDOW_MS = 4_000;
const HIDDEN_SHRINK_MS = 5 * 60 * 1000;

export interface SessionCacheController {
  readonly entries: readonly SessionCacheEntry[];
  readonly activeId: string;
  tierOf(id: string): SessionCacheTier;
  activate(id: string): void;
  evict(id: string): void;
  clear(): void;
  syncSessions(sessions: readonly SessionInfo[]): void;
  dispose(): void;
}

function deviceMemoryGb(): number | undefined {
  const nav = navigator as Navigator & { deviceMemory?: number };
  return typeof nav.deviceMemory === "number" ? nav.deviceMemory : undefined;
}

/**
 * Manages the LRU pool of mounted session pages, Warm/Cool tiering,
 * hidden-tab memory trimming, and bounded serial background refresh.
 */
export function createSessionCache(options?: {
  config?: () => SessionCacheConfig;
  now?: () => number;
  refreshWindowMs?: number;
  hiddenShrinkMs?: number;
}): SessionCacheController {
  const compact = new MediaQuery("(max-width: 639px)");
  const now = options?.now ?? (() => Date.now());
  const refreshWindowMs = options?.refreshWindowMs ?? REFRESH_WINDOW_MS;
  const hiddenShrinkMs = options?.hiddenShrinkMs ?? HIDDEN_SHRINK_MS;
  const getConfig =
    options?.config ??
    (() =>
      resolveSessionCacheConfig({
        compact: compact.current,
        touch: hasTouchInput(),
        deviceMemoryGb: deviceMemoryGb(),
      }));

  let state = $state.raw<SessionCacheState>(emptySessionCache());
  let refreshingId = $state("");
  let runningIds: ReadonlySet<string> | undefined;
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let hiddenTimer: ReturnType<typeof setTimeout> | undefined;
  let disposed = false;

  function stopRefreshWindow() {
    clearTimeout(refreshTimer);
    refreshTimer = undefined;
    refreshingId = "";
  }

  function runSweep() {
    if (disposed || !state.entries.length) return;
    const { state: next } = sweepSessionCache(
      state,
      getConfig(),
      now(),
      runningIds,
    );
    state = next;
    if (
      refreshingId &&
      !next.entries.some((entry) => entry.id === refreshingId)
    ) {
      stopRefreshWindow();
    }
  }

  function pollBackgroundRefresh() {
    if (
      disposed ||
      refreshingId ||
      (typeof document !== "undefined" && document.hidden)
    ) {
      return;
    }
    const { state: next, sessionId } = takeDueBackgroundRefresh(state, now());
    if (!sessionId) return;
    state = next;
    refreshingId = sessionId;
    refreshTimer = setTimeout(() => {
      refreshTimer = undefined;
      refreshingId = "";
    }, refreshWindowMs);
  }

  const sweepTimer = setInterval(runSweep, SWEEP_INTERVAL_MS);
  const pollTimer = setInterval(pollBackgroundRefresh, REFRESH_POLL_MS);

  const onVisibilityChange = () => {
    clearTimeout(hiddenTimer);
    hiddenTimer = undefined;
    if (document.hidden) {
      stopRefreshWindow();
      hiddenTimer = setTimeout(() => {
        hiddenTimer = undefined;
        if (disposed || !document.hidden) return;
        state = shrinkSessionCacheToActive(state).state;
      }, hiddenShrinkMs);
    } else {
      runSweep();
    }
  };
  if (typeof document !== "undefined") {
    document.addEventListener("visibilitychange", onVisibilityChange);
  }

  return {
    get entries() {
      return state.entries;
    },
    get activeId() {
      return state.activeId;
    },
    tierOf(id: string): SessionCacheTier {
      if (id === state.activeId) return "active";
      if (id === refreshingId) return "warm";
      return state.entries.find((entry) => entry.id === id)?.tier ?? "cool";
    },
    activate(id: string) {
      if (disposed) return;
      untrack(() => {
        const started = performance.now();
        const { state: next, outcome } = activateCachedSession(
          state,
          id,
          getConfig(),
          now(),
          runningIds,
        );
        if (refreshingId && refreshingId === id) stopRefreshWindow();
        state = next;
        if (id && outcome !== "hit-active") {
          recordSessionSwitch(
            outcome,
            performance.now() - started,
            next.entries.length,
          );
        }
      });
    },
    evict(id: string) {
      if (disposed || !id) return;
      untrack(() => {
        if (refreshingId === id) stopRefreshWindow();
        state = evictCachedSession(state, id, getConfig(), now(), runningIds);
      });
    },
    clear() {
      untrack(() => {
        stopRefreshWindow();
        state = emptySessionCache();
      });
    },
    syncSessions(sessions: readonly SessionInfo[]) {
      if (disposed) return;
      untrack(() => {
        if (!state.entries.length) return;
        const extracted = extractSessionActivity(sessions);
        runningIds = extracted.runningIds;
        const updated = observeCachedSessionActivity(
          state,
          extracted.signatures,
          getConfig(),
          now(),
        );
        const swept = sweepSessionCache(
          updated,
          getConfig(),
          now(),
          runningIds,
        );
        state = swept.state;
      });
    },
    dispose() {
      disposed = true;
      clearInterval(sweepTimer);
      clearInterval(pollTimer);
      clearTimeout(hiddenTimer);
      stopRefreshWindow();
      if (typeof document !== "undefined") {
        document.removeEventListener("visibilitychange", onVisibilityChange);
      }
      state = emptySessionCache();
    },
  };
}
