import { describe, expect, test } from "vite-plus/test";
import {
  DESKTOP_CACHE_CONFIG,
  MOBILE_CACHE_CONFIG,
  activateCachedSession,
  emptySessionCache,
  evictCachedSession,
  observeCachedSessionActivity,
  resolveSessionCacheConfig,
  sessionActivitySignature,
  shrinkSessionCacheToActive,
  sweepSessionCache,
  takeDueBackgroundRefresh,
} from "../src/domains/sessions/session-cache";

describe("session-cache domain", () => {
  test("resolves desktop, low-memory desktop, and mobile configs", () => {
    expect(resolveSessionCacheConfig({ compact: false, touch: false })).toEqual(
      DESKTOP_CACHE_CONFIG,
    );
    expect(
      resolveSessionCacheConfig({
        compact: false,
        touch: false,
        deviceMemoryGb: 4,
      }),
    ).toEqual({ ...DESKTOP_CACHE_CONFIG, maxEntries: 4, maxWarm: 1 });
    expect(resolveSessionCacheConfig({ compact: true, touch: false })).toEqual(
      MOBILE_CACHE_CONFIG,
    );
    expect(resolveSessionCacheConfig({ compact: false, touch: true })).toEqual(
      MOBILE_CACHE_CONFIG,
    );
  });

  test("maintains LRU order, warm/cool tiers, and evicts oldest past capacity", () => {
    const config = { ...DESKTOP_CACHE_CONFIG, maxEntries: 3, maxWarm: 1 };
    let state = emptySessionCache();
    let now = 1_000;

    let res = activateCachedSession(state, "s1", config, now);
    expect(res.outcome).toBe("miss");
    state = res.state;

    now += 1_000;
    res = activateCachedSession(state, "s2", config, now);
    expect(res.outcome).toBe("miss");
    state = res.state;
    expect(state.entries.map((e) => [e.id, e.tier])).toEqual([
      ["s2", "active"],
      ["s1", "warm"],
    ]);

    now += 1_000;
    res = activateCachedSession(state, "s3", config, now);
    state = res.state;
    expect(state.entries.map((e) => [e.id, e.tier])).toEqual([
      ["s3", "active"],
      ["s2", "warm"],
      ["s1", "cool"],
    ]);

    // Returning to s1 hits cool cache and promotes it to active.
    now += 1_000;
    res = activateCachedSession(state, "s1", config, now);
    expect(res.outcome).toBe("hit-cool");
    state = res.state;
    expect(state.entries.map((e) => [e.id, e.tier])).toEqual([
      ["s1", "active"],
      ["s3", "warm"],
      ["s2", "cool"],
    ]);

    // Returning to s3 hits warm cache.
    now += 1_000;
    res = activateCachedSession(state, "s3", config, now);
    expect(res.outcome).toBe("hit-warm");
    state = res.state;

    // Adding s4 evicts least-recently-used s2.
    now += 1_000;
    res = activateCachedSession(state, "s4", config, now);
    expect(res.outcome).toBe("miss");
    expect(res.evicted).toEqual(["s2"]);
    expect(res.state.entries.map((e) => e.id)).toEqual(["s4", "s3", "s1"]);
  });

  test("navigating home keeps cached sessions in background tiers", () => {
    const config = { ...DESKTOP_CACHE_CONFIG, maxEntries: 4, maxWarm: 2 };
    let state = emptySessionCache();
    state = activateCachedSession(state, "s1", config, 1_000).state;
    state = activateCachedSession(state, "s2", config, 2_000).state;

    const home = activateCachedSession(state, "", config, 3_000);
    expect(home.state.activeId).toBe("");
    expect(home.state.entries.map((e) => [e.id, e.tier])).toEqual([
      ["s2", "warm"],
      ["s1", "warm"],
    ]);

    const back = activateCachedSession(home.state, "s2", config, 4_000);
    expect(back.outcome).toBe("hit-warm");
    expect(back.state.activeId).toBe("s2");
  });

  test("demotes expired warm entries to cool and evicts idle entries past TTL", () => {
    const config = {
      ...DESKTOP_CACHE_CONFIG,
      maxEntries: 4,
      maxWarm: 2,
      warmTtlMs: 10_000,
      idleTtlMs: 30_000,
    };
    let state = emptySessionCache();
    state = activateCachedSession(state, "s1", config, 0).state;
    state = activateCachedSession(state, "s2", config, 1_000).state;

    // After warmTtlMs, non-running s1 demotes to cool.
    let swept = sweepSessionCache(state, config, 12_000, new Set());
    expect(swept.evicted).toEqual([]);
    expect(swept.state.entries.find((e) => e.id === "s1")?.tier).toBe("cool");

    // A running session stays warm even past warmTtlMs.
    swept = sweepSessionCache(state, config, 12_000, new Set(["s1"]));
    expect(swept.state.entries.find((e) => e.id === "s1")?.tier).toBe("warm");

    // Past idleTtlMs, s1 is evicted while active s2 remains.
    swept = sweepSessionCache(state, config, 35_000, new Set());
    expect(swept.evicted).toEqual(["s1"]);
    expect(swept.state.entries.map((e) => e.id)).toEqual(["s2"]);
  });

  test("evicts specific sessions and shrinks to active on long hidden tab", () => {
    const config = DESKTOP_CACHE_CONFIG;
    let state = emptySessionCache();
    state = activateCachedSession(state, "s1", config, 1_000).state;
    state = activateCachedSession(state, "s2", config, 2_000).state;
    state = activateCachedSession(state, "s3", config, 3_000).state;

    state = evictCachedSession(state, "s2", config, 4_000);
    expect(state.entries.map((e) => e.id)).toEqual(["s3", "s1"]);

    const shrunk = shrinkSessionCacheToActive(state);
    expect(shrunk.evicted).toEqual(["s1"]);
    expect(shrunk.state.entries.map((e) => e.id)).toEqual(["s3"]);
  });

  test("schedules debounced and cooldown-bounded background refresh for cool entries", () => {
    const config = {
      ...DESKTOP_CACHE_CONFIG,
      maxEntries: 3,
      maxWarm: 1,
      refreshDebounceMs: 3_000,
      refreshCooldownMs: 20_000,
    };
    let state = emptySessionCache();
    state = activateCachedSession(state, "s1", config, 0).state;
    state = activateCachedSession(state, "s2", config, 1_000).state;
    state = activateCachedSession(state, "s3", config, 2_000).state;
    // s1 is now cool.

    const sig1 = sessionActivitySignature({
      status: "idle",
      title: "bash",
      preview: "ready",
    });
    const sig2 = sessionActivitySignature({
      status: "running",
      title: "bash",
      preview: "building...",
    });

    // Initial baseline signature does not trigger a refresh.
    state = observeCachedSessionActivity(
      state,
      new Map([["s1", sig1]]),
      config,
      3_000,
    );
    expect(takeDueBackgroundRefresh(state, 10_000).sessionId).toBeUndefined();

    // Activity change schedules a refresh after debounce.
    state = observeCachedSessionActivity(
      state,
      new Map([["s1", sig2]]),
      config,
      10_000,
    );
    expect(takeDueBackgroundRefresh(state, 12_999).sessionId).toBeUndefined();

    const taken = takeDueBackgroundRefresh(state, 13_000);
    expect(taken.sessionId).toBe("s1");
    state = taken.state;

    // A subsequent change respects refreshCooldownMs from lastRefreshAt (13_000).
    state = observeCachedSessionActivity(
      state,
      new Map([["s1", sig1]]),
      config,
      15_000,
    );
    expect(takeDueBackgroundRefresh(state, 32_999).sessionId).toBeUndefined();
    expect(takeDueBackgroundRefresh(state, 33_000).sessionId).toBe("s1");
  });
});
