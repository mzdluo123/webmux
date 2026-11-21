import { afterEach, describe, expect, it, vi } from "vitest";
import {
  configurePerf,
  flushPerfEvents,
  normalizePath,
  parseServerTimingMs,
  recordApiTiming,
  recordWsMessage,
  resetPerfForTests,
} from "../src/platform/perf";
import * as transport from "../src/platform/transport";

describe("platform/perf", () => {
  afterEach(() => {
    resetPerfForTests();
    vi.restoreAllMocks();
  });

  it("normalizes dynamic path segments and strips query strings", () => {
    expect(normalizePath("/api/sessions/abc123/chat?limit=20")).toBe(
      "/api/sessions/:id/chat",
    );
    expect(
      normalizePath("/api/repositories/r1/worktrees/w2/git/status?x=1"),
    ).toBe("/api/repositories/:id/worktrees/:id/git/status");
    expect(normalizePath("/s/xyz987")).toBe("/s/:id");
  });

  it("parses Server-Timing app duration", () => {
    expect(parseServerTimingMs("app;dur=123.45, git;dur=80.10")).toBe(123.45);
    expect(parseServerTimingMs("git;dur=12.0")).toBe(0);
    expect(parseServerTimingMs(null)).toBe(0);
  });

  it("records slow API and WS events and flushes them in a batch", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.spyOn(transport, "currentTransport").mockReturnValue({
      fetch: fetchMock,
    } as unknown as ReturnType<typeof transport.currentTransport>);

    configurePerf({ enabled: true, slowMs: 100 });
    recordApiTiming("GET", "/health", 20, 200, "app;dur=5.00");
    recordApiTiming(
      "GET",
      "/sessions/abc/git",
      180.4,
      200,
      "app;dur=150.25, git;dur=120.00",
    );
    recordWsMessage("/ws/s/abc/chat", 4096, 24.5);

    await flushPerfEvents();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const call = fetchMock.mock.calls[0];
    expect(call).toBeDefined();
    const [path, init] = call as [string, RequestInit];
    expect(path).toBe("/perf/events");
    const parsed = JSON.parse(String(init.body)) as {
      events: Array<{
        kind: string;
        op: string;
        durationMs?: number;
        serverMs?: number;
      }>;
    };
    expect(parsed.events).toHaveLength(2);
    expect(parsed.events[0]).toMatchObject({
      kind: "api",
      op: "GET /sessions/:id/git",
      durationMs: 180.4,
      serverMs: 150.25,
    });
    expect(parsed.events[1]).toMatchObject({
      kind: "ws",
      op: "/ws/s/:id/chat",
      durationMs: 24.5,
    });
  });

  it("is a no-op when disabled", async () => {
    const fetchMock = vi.fn();
    vi.spyOn(transport, "currentTransport").mockReturnValue({
      fetch: fetchMock,
    } as unknown as ReturnType<typeof transport.currentTransport>);

    configurePerf(undefined);
    recordApiTiming("GET", "/sessions/abc", 500, 200, null);
    await flushPerfEvents();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
