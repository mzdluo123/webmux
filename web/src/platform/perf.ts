import { authenticationHeaders } from "./relay-auth";
import { currentTransport } from "./transport";

export interface PerfClientConfig {
  enabled: boolean;
  slowMs: number;
}

interface PerfEventPayload {
  ts: string;
  kind: "loaf" | "longtask" | "interaction" | "cls" | "api" | "ws" | "memory";
  op: string;
  durationMs?: number;
  serverMs?: number;
  value?: number;
  bytes?: number;
  status?: number;
  route?: string;
  detail?: string;
}

interface LoAFScriptEntry {
  sourceURL?: string;
  sourceFunctionName?: string;
  invoker?: string;
  duration?: number;
}

interface LoAFEntry extends PerformanceEntry {
  blockingDuration?: number;
  renderStart?: number;
  styleAndLayoutStart?: number;
  scripts?: LoAFScriptEntry[];
}

interface EventTimingEntry extends PerformanceEntry {
  processingStart?: number;
  target?: Element | null;
}

interface LayoutShiftEntry extends PerformanceEntry {
  hadRecentInput?: boolean;
  value?: number;
}

interface ChromeMemoryInfo {
  usedJSHeapSize?: number;
}

const FLUSH_INTERVAL_MS = 15_000;
const MEMORY_INTERVAL_MS = 30_000;
const MAX_BUFFER = 80;
const FLUSH_BATCH_SIZE = 40;
const WS_SLOW_HANDLE_MS = 16;

let active = false;
let slowThresholdMs = 100;
let buffer: PerfEventPayload[] = [];
let observers: PerformanceObserver[] = [];
let flushTimer: ReturnType<typeof setInterval> | undefined;
let memoryTimer: ReturnType<typeof setInterval> | undefined;
let flushing = false;
let visibilityBound = false;

// Periodic WS throughput accumulator so bursty streams surface even when
// individual frames are fast.
const wsWindow = new Map<
  string,
  { frames: number; bytes: number; totalMs: number; maxMs: number }
>();

function round2(n: number): number {
  return Math.round(n * 100) / 100;
}

function currentRoute(): string {
  const path = typeof location === "undefined" ? "/" : location.pathname;
  return normalizePath(path);
}

export function normalizePath(raw: string): string {
  const path = raw.split("?", 1)[0] ?? raw;
  return path
    .replace(/\/sessions\/[^/]+/g, "/sessions/:id")
    .replace(/\/s\/[^/]+/g, "/s/:id")
    .replace(/\/repositories\/[^/]+/g, "/repositories/:id")
    .replace(/\/worktrees\/[^/]+/g, "/worktrees/:id")
    .replace(/\/projects\/[^/]+/g, "/projects/:id")
    .replace(/\/operations\/[^/]+/g, "/operations/:id");
}

export function parseServerTimingMs(header: string | null): number {
  if (!header) return 0;
  let total = 0;
  for (const part of header.split(",")) {
    const trimmed = part.trim();
    if (!trimmed.startsWith("app")) continue;
    const match = /;\s*dur=([0-9]+(?:\.[0-9]+)?)/.exec(trimmed);
    if (match) {
      total = Number(match[1]) || 0;
      break;
    }
  }
  return round2(total);
}

function shortScriptName(url: string | undefined): string {
  if (!url) return "";
  const clean = url.split("?", 1)[0] ?? url;
  const idx = clean.lastIndexOf("/");
  return idx >= 0 ? clean.slice(idx + 1) : clean;
}

function describeTarget(el: Element | null | undefined): string {
  if (!el) return "";
  const tag = el.tagName.toLowerCase();
  const slot = el.getAttribute("data-slot");
  const role = el.getAttribute("role");
  if (slot) return `${tag}[data-slot=${slot}]`;
  if (role) return `${tag}[role=${role}]`;
  return tag;
}

function enqueue(event: PerfEventPayload) {
  if (!active) return;
  if (buffer.length >= MAX_BUFFER) buffer.shift();
  buffer.push(event);
  if (buffer.length >= FLUSH_BATCH_SIZE) void flushPerfEvents();
}

export function recordApiTiming(
  method: string,
  path: string,
  durationMs: number,
  status: number,
  serverTimingHeader: string | null,
) {
  if (!active || path === "/perf/events") return;
  if (durationMs < slowThresholdMs && status < 500) return;
  const serverMs = parseServerTimingMs(serverTimingHeader);
  enqueue({
    ts: new Date().toISOString(),
    kind: "api",
    op: `${method.toUpperCase()} ${normalizePath(path)}`,
    durationMs: round2(durationMs),
    serverMs: serverMs > 0 ? serverMs : undefined,
    status,
    route: currentRoute(),
  });
}

export function recordWsMessage(path: string, bytes: number, handleMs: number) {
  if (!active) return;
  const op = normalizePath(path);
  const acc = wsWindow.get(op) ?? { frames: 0, bytes: 0, totalMs: 0, maxMs: 0 };
  acc.frames += 1;
  acc.bytes += bytes;
  acc.totalMs += handleMs;
  if (handleMs > acc.maxMs) acc.maxMs = handleMs;
  wsWindow.set(op, acc);
  if (handleMs >= WS_SLOW_HANDLE_MS) {
    enqueue({
      ts: new Date().toISOString(),
      kind: "ws",
      op,
      durationMs: round2(handleMs),
      bytes,
      route: currentRoute(),
      detail: "slow-frame",
    });
  }
}

function flushWsWindow() {
  if (!active || wsWindow.size === 0) return;
  const route = currentRoute();
  const ts = new Date().toISOString();
  for (const [op, acc] of wsWindow) {
    if (acc.totalMs >= 50 || acc.bytes >= 256 * 1024 || acc.frames >= 100) {
      enqueue({
        ts,
        kind: "ws",
        op,
        durationMs: round2(acc.totalMs),
        bytes: acc.bytes,
        value: acc.frames,
        route,
        detail: `window max=${round2(acc.maxMs)}ms frames=${acc.frames}`,
      });
    }
  }
  wsWindow.clear();
}

function sampleMemoryAndDom() {
  if (!active || typeof document === "undefined" || document.hidden) return;
  const ts = new Date().toISOString();
  const route = currentRoute();
  const mem = (performance as Performance & { memory?: ChromeMemoryInfo })
    .memory;
  if (mem?.usedJSHeapSize) {
    enqueue({
      ts,
      kind: "memory",
      op: "js_heap_mb",
      value: round2(mem.usedJSHeapSize / (1024 * 1024)),
      route,
    });
  }
  enqueue({
    ts,
    kind: "memory",
    op: "dom_nodes",
    value: document.getElementsByTagName("*").length,
    route,
  });
}

export async function flushPerfEvents(): Promise<void> {
  flushWsWindow();
  if (!active || flushing || buffer.length === 0) return;
  flushing = true;
  const batch = buffer.splice(0, FLUSH_BATCH_SIZE);
  try {
    await currentTransport().fetch("/perf/events", {
      method: "POST",
      headers: {
        ...authenticationHeaders(),
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ events: batch }),
      keepalive: true,
    });
  } catch {
    // Best-effort diagnostics must never surface UI errors or retry forever.
  } finally {
    flushing = false;
  }
}

function handleVisibilityChange() {
  if (document.visibilityState === "hidden") {
    void flushPerfEvents();
  }
}

function observeIfSupported(
  type: string,
  callback: (entries: PerformanceEntry[]) => void,
  extra?: Record<string, unknown>,
): boolean {
  if (typeof PerformanceObserver === "undefined") return false;
  const supported = PerformanceObserver.supportedEntryTypes ?? [];
  if (!supported.includes(type)) return false;
  try {
    const observer = new PerformanceObserver((list) => {
      callback(list.getEntries());
    });
    observer.observe({
      type,
      buffered: false,
      ...extra,
    } as PerformanceObserverInit);
    observers.push(observer);
    return true;
  } catch {
    return false;
  }
}

function startObservers() {
  const loafThreshold = Math.max(50, Math.round(slowThresholdMs / 2));
  const hasLoaf = observeIfSupported("long-animation-frame", (entries) => {
    for (const raw of entries) {
      const entry = raw as LoAFEntry;
      if (entry.duration < loafThreshold) continue;
      const topScript = (entry.scripts ?? []).reduce<
        LoAFScriptEntry | undefined
      >(
        (best, s) => ((s.duration ?? 0) > (best?.duration ?? 0) ? s : best),
        undefined,
      );
      const fn = topScript?.sourceFunctionName || topScript?.invoker || "frame";
      const file = shortScriptName(topScript?.sourceURL);
      const op = file ? `${file}:${fn}` : fn;
      const detailParts: string[] = [];
      if (entry.blockingDuration) {
        detailParts.push(`blocking=${round2(entry.blockingDuration)}ms`);
      }
      if (entry.renderStart && entry.styleAndLayoutStart) {
        const layoutMs =
          entry.startTime + entry.duration - entry.styleAndLayoutStart;
        if (layoutMs > 1) detailParts.push(`layout=${round2(layoutMs)}ms`);
      }
      enqueue({
        ts: new Date().toISOString(),
        kind: "loaf",
        op,
        durationMs: round2(entry.duration),
        route: currentRoute(),
        detail: detailParts.join(" "),
      });
    }
  });
  if (!hasLoaf) {
    observeIfSupported("longtask", (entries) => {
      for (const entry of entries) {
        if (entry.duration < loafThreshold) continue;
        enqueue({
          ts: new Date().toISOString(),
          kind: "longtask",
          op: entry.name || "longtask",
          durationMs: round2(entry.duration),
          route: currentRoute(),
        });
      }
    });
  }

  observeIfSupported(
    "event",
    (entries) => {
      const threshold = Math.max(80, slowThresholdMs);
      for (const raw of entries) {
        const entry = raw as EventTimingEntry;
        if (entry.duration < threshold) continue;
        enqueue({
          ts: new Date().toISOString(),
          kind: "interaction",
          op: entry.name || "event",
          durationMs: round2(entry.duration),
          route: currentRoute(),
          detail: describeTarget(entry.target),
        });
      }
    },
    { durationThreshold: Math.max(80, slowThresholdMs) },
  );

  observeIfSupported("layout-shift", (entries) => {
    for (const raw of entries) {
      const entry = raw as LayoutShiftEntry;
      if (entry.hadRecentInput || !entry.value || entry.value < 0.01) continue;
      enqueue({
        ts: new Date().toISOString(),
        kind: "cls",
        op: "layout-shift",
        value: round2(entry.value),
        route: currentRoute(),
      });
    }
  });
}

function stopCollectors() {
  for (const obs of observers) obs.disconnect();
  observers = [];
  clearInterval(flushTimer);
  clearInterval(memoryTimer);
  flushTimer = undefined;
  memoryTimer = undefined;
  if (visibilityBound && typeof document !== "undefined") {
    document.removeEventListener("visibilitychange", handleVisibilityChange);
    visibilityBound = false;
  }
  buffer = [];
  wsWindow.clear();
}

export function configurePerf(config?: PerfClientConfig) {
  if (!config?.enabled) {
    if (active) {
      active = false;
      stopCollectors();
    }
    return;
  }
  const nextSlow = Math.max(10, Math.round(config.slowMs || 100));
  if (active && slowThresholdMs === nextSlow) return;
  stopCollectors();
  active = true;
  slowThresholdMs = nextSlow;
  startObservers();
  flushTimer = setInterval(() => void flushPerfEvents(), FLUSH_INTERVAL_MS);
  memoryTimer = setInterval(sampleMemoryAndDom, MEMORY_INTERVAL_MS);
  if (typeof document !== "undefined" && !visibilityBound) {
    document.addEventListener("visibilitychange", handleVisibilityChange);
    visibilityBound = true;
  }
}

export function resetPerfForTests() {
  active = false;
  slowThresholdMs = 100;
  flushing = false;
  stopCollectors();
}
