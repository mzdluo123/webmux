export type SessionCacheTier = "active" | "warm" | "cool";

export interface SessionCacheConfig {
  /** Maximum number of session pages kept mounted in the LRU pool. */
  maxEntries: number;
  /** Maximum number of background pages kept in the live `warm` tier. */
  maxWarm: number;
  /** Warm background entry demotes to `cool` after this many ms. */
  warmTtlMs: number;
  /** Inactive background entry is evicted after this many ms. */
  idleTtlMs: number;
  /** Background refresh debounce after a session-list activity signal. */
  refreshDebounceMs: number;
  /** Minimum interval between background refreshes for the same cool session. */
  refreshCooldownMs: number;
}

export interface SessionCacheEntry {
  id: string;
  /** Timestamp (ms) when this session was last active in the foreground. */
  lastActiveAt: number;
  /** Timestamp (ms) when this session last left the foreground (0 while active). */
  deactivatedAt: number;
  /** Current computed tier. */
  tier: SessionCacheTier;
  /** Last observed activity signature from `/ws/sessions`. */
  activitySignature: string;
  /** Timestamp (ms) of the last background refresh trigger. */
  lastRefreshAt: number;
  /** Pending background refresh request due at this timestamp (0 if none). */
  refreshDueAt: number;
}

export interface SessionCacheState {
  activeId: string;
  /** Entries ordered from most-recently-used (index 0) to least-recently-used. */
  entries: SessionCacheEntry[];
}

export const DESKTOP_CACHE_CONFIG: SessionCacheConfig = {
  maxEntries: 5,
  maxWarm: 2,
  warmTtlMs: 3 * 60 * 1000,
  idleTtlMs: 15 * 60 * 1000,
  refreshDebounceMs: 3_000,
  refreshCooldownMs: 20_000,
};

export const MOBILE_CACHE_CONFIG: SessionCacheConfig = {
  maxEntries: 3,
  maxWarm: 1,
  warmTtlMs: 90 * 1000,
  idleTtlMs: 8 * 60 * 1000,
  refreshDebounceMs: 5_000,
  refreshCooldownMs: 30_000,
};

export function resolveSessionCacheConfig(options: {
  compact: boolean;
  touch: boolean;
  deviceMemoryGb?: number;
}): SessionCacheConfig {
  if (options.compact || options.touch) return MOBILE_CACHE_CONFIG;
  if (
    options.deviceMemoryGb !== undefined &&
    options.deviceMemoryGb > 0 &&
    options.deviceMemoryGb <= 4
  ) {
    return {
      ...DESKTOP_CACHE_CONFIG,
      maxEntries: 4,
      maxWarm: 1,
    };
  }
  return DESKTOP_CACHE_CONFIG;
}

export function emptySessionCache(): SessionCacheState {
  return { activeId: "", entries: [] };
}

export function extractSessionActivity(
  sessions: readonly {
    id: string;
    status: string;
    title: string;
    view?: string;
    state?: string;
    lastActiveAt?: string;
    attention?: { kind: string } | null;
    preview?: string;
  }[],
): { runningIds: Set<string>; signatures: Map<string, string> } {
  const runningIds = new Set<string>();
  const signatures = new Map<string, string>();
  for (const session of sessions) {
    if (session.status === "running") runningIds.add(session.id);
    signatures.set(session.id, sessionActivitySignature(session));
  }
  return { runningIds, signatures };
}

export function sessionActivitySignature(session: {
  status: string;
  title: string;
  view?: string;
  state?: string;
  lastActiveAt?: string;
  attention?: { kind: string } | null;
  preview?: string;
}): string {
  return [
    session.status,
    session.view ?? "",
    session.state ?? "",
    session.lastActiveAt ?? "",
    session.attention?.kind ?? "",
    session.title,
    session.preview ?? "",
  ].join("\0");
}

function recomputeTiers(
  entries: SessionCacheEntry[],
  activeId: string,
  config: SessionCacheConfig,
  now: number,
  runningIds?: ReadonlySet<string>,
): SessionCacheEntry[] {
  let warmCount = 0;
  return entries.map((entry) => {
    if (entry.id === activeId) {
      return {
        ...entry,
        tier: "active",
        deactivatedAt: 0,
        refreshDueAt: 0,
      };
    }
    const backgroundAge =
      entry.deactivatedAt > 0 ? Math.max(0, now - entry.deactivatedAt) : 0;
    const isRunning = runningIds?.has(entry.id) ?? false;
    const canBeWarm =
      warmCount < config.maxWarm &&
      (backgroundAge < config.warmTtlMs || isRunning);
    if (canBeWarm) {
      warmCount += 1;
      return { ...entry, tier: "warm", refreshDueAt: 0 };
    }
    return { ...entry, tier: "cool" };
  });
}

export type SessionActivateOutcome =
  | "hit-warm"
  | "hit-cool"
  | "hit-active"
  | "miss";

/**
 * Moves `sessionId` to the foreground and promotes it to the front of the LRU.
 * Passing `""` leaves all cached entries in the background (e.g. navigating home).
 */
export function activateCachedSession(
  state: SessionCacheState,
  sessionId: string,
  config: SessionCacheConfig,
  now: number,
  runningIds?: ReadonlySet<string>,
): {
  state: SessionCacheState;
  outcome: SessionActivateOutcome;
  evicted: string[];
} {
  const prevActive = state.activeId;
  let entries = state.entries.map((entry) =>
    entry.id === prevActive && prevActive !== sessionId
      ? { ...entry, deactivatedAt: entry.deactivatedAt || now }
      : entry,
  );

  if (!sessionId) {
    const next = recomputeTiers(entries, "", config, now, runningIds);
    return {
      state: { activeId: "", entries: next },
      outcome: "hit-active",
      evicted: [],
    };
  }

  const existingIndex = entries.findIndex((entry) => entry.id === sessionId);
  let outcome: SessionActivateOutcome = "miss";
  if (existingIndex >= 0) {
    const existing = entries[existingIndex]!;
    outcome =
      existing.id === prevActive
        ? "hit-active"
        : existing.tier === "warm"
          ? "hit-warm"
          : "hit-cool";
    entries.splice(existingIndex, 1);
    entries.unshift({
      ...existing,
      lastActiveAt: now,
      deactivatedAt: 0,
      tier: "active",
      refreshDueAt: 0,
    });
  } else {
    entries.unshift({
      id: sessionId,
      lastActiveAt: now,
      deactivatedAt: 0,
      tier: "active",
      activitySignature: "",
      lastRefreshAt: 0,
      refreshDueAt: 0,
    });
  }

  const evicted: string[] = [];
  const limit = Math.max(1, config.maxEntries);
  while (entries.length > limit) {
    const removed = entries.pop();
    if (removed) evicted.push(removed.id);
  }

  entries = recomputeTiers(entries, sessionId, config, now, runningIds);
  return {
    state: { activeId: sessionId, entries },
    outcome,
    evicted,
  };
}

/**
 * Evicts a specific session ID from the cache pool (e.g. closed/archived or
 * joined into another page's split panes).
 */
export function evictCachedSession(
  state: SessionCacheState,
  sessionId: string,
  config: SessionCacheConfig,
  now: number,
  runningIds?: ReadonlySet<string>,
): SessionCacheState {
  const filtered = state.entries.filter((entry) => entry.id !== sessionId);
  if (filtered.length === state.entries.length) return state;
  const activeId = state.activeId === sessionId ? "" : state.activeId;
  return {
    activeId,
    entries: recomputeTiers(filtered, activeId, config, now, runningIds),
  };
}

/**
 * Evicts idle entries past `idleTtlMs`, demotes expired warm entries to cool,
 * and trims entries when the capacity limit shrinks (e.g. viewport resize).
 */
export function sweepSessionCache(
  state: SessionCacheState,
  config: SessionCacheConfig,
  now: number,
  runningIds?: ReadonlySet<string>,
): { state: SessionCacheState; evicted: string[] } {
  const evicted: string[] = [];
  let kept = state.entries.filter((entry) => {
    if (entry.id === state.activeId) return true;
    const idleAge =
      entry.deactivatedAt > 0 ? Math.max(0, now - entry.deactivatedAt) : 0;
    if (idleAge >= config.idleTtlMs) {
      evicted.push(entry.id);
      return false;
    }
    return true;
  });

  const limit = Math.max(1, config.maxEntries);
  while (kept.length > limit) {
    const idx = findOldestBackgroundIndex(kept, state.activeId);
    if (idx < 0) break;
    const [removed] = kept.splice(idx, 1);
    if (removed) evicted.push(removed.id);
  }

  kept = recomputeTiers(kept, state.activeId, config, now, runningIds);
  return {
    state: { activeId: state.activeId, entries: kept },
    evicted,
  };
}

/**
 * Shrinks the cache down to only the currently active session (used when the
 * browser tab has been hidden for an extended period).
 */
export function shrinkSessionCacheToActive(state: SessionCacheState): {
  state: SessionCacheState;
  evicted: string[];
} {
  const evicted: string[] = [];
  const entries = state.entries.filter((entry) => {
    if (entry.id === state.activeId) return true;
    evicted.push(entry.id);
    return false;
  });
  return {
    state: { activeId: state.activeId, entries },
    evicted,
  };
}

/**
 * Observes updated activity signatures from `/ws/sessions` and schedules
 * debounced background refreshes for `cool` cached entries that changed.
 */
export function observeCachedSessionActivity(
  state: SessionCacheState,
  signatures: ReadonlyMap<string, string>,
  config: SessionCacheConfig,
  now: number,
): SessionCacheState {
  let changed = false;
  const entries = state.entries.map((entry) => {
    const nextSig = signatures.get(entry.id);
    if (nextSig === undefined || nextSig === entry.activitySignature)
      return entry;
    changed = true;
    const hadPrevious = entry.activitySignature !== "";
    if (!hadPrevious || entry.tier !== "cool") {
      return { ...entry, activitySignature: nextSig };
    }
    const cooldownUntil =
      entry.lastRefreshAt > 0
        ? entry.lastRefreshAt + config.refreshCooldownMs
        : 0;
    const earliest = Math.max(now + config.refreshDebounceMs, cooldownUntil);
    return {
      ...entry,
      activitySignature: nextSig,
      refreshDueAt:
        entry.refreshDueAt > 0
          ? Math.min(entry.refreshDueAt, earliest)
          : earliest,
    };
  });
  return changed ? { ...state, entries } : state;
}

/**
 * Picks at most one due `cool` entry for bounded serial background refresh.
 */
export function takeDueBackgroundRefresh(
  state: SessionCacheState,
  now: number,
): { state: SessionCacheState; sessionId: string | undefined } {
  let chosenIndex = -1;
  for (let i = 0; i < state.entries.length; i++) {
    const entry = state.entries[i]!;
    if (
      entry.tier === "cool" &&
      entry.refreshDueAt > 0 &&
      entry.refreshDueAt <= now
    ) {
      chosenIndex = i;
      break;
    }
  }
  if (chosenIndex < 0) return { state, sessionId: undefined };
  const chosen = state.entries[chosenIndex]!;
  const entries = state.entries.slice();
  entries[chosenIndex] = {
    ...chosen,
    lastRefreshAt: now,
    refreshDueAt: 0,
  };
  return {
    state: { ...state, entries },
    sessionId: chosen.id,
  };
}

function findOldestBackgroundIndex(
  entries: readonly SessionCacheEntry[],
  activeId: string,
): number {
  for (let i = entries.length - 1; i >= 0; i--) {
    if (entries[i]!.id !== activeId) return i;
  }
  return -1;
}
