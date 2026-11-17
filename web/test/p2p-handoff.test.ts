import { z } from "zod";
import {
  ChatFrameSchema,
  SessionEventSchema,
  StreamPositionSchema,
  type SessionInfo,
  type StreamPosition,
} from "../src/contracts/schemas";
import {
  applyFrame,
  emptyChat,
  resumePoint,
} from "../src/domains/chat/chat-model";
import { applySessionEvent } from "../src/domains/sessions/session-events";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vite-plus/test";
import {
  resolveAppendOverlap,
  trimOverlapChunk,
} from "../src/domains/terminal/terminal-protocol";
import {
  SOCKET_CLOSED,
  SOCKET_CONNECTING,
  SOCKET_OPEN,
  setClientTransport,
  type ClientTransport,
  type StreamOptions,
  type TransportSocket,
  type TransportPath,
} from "../src/platform/transport";
import { connect, handoffLiveSockets, resumeTimeout } from "../src/platform/ws";
import type { HandoffOptions } from "../src/platform/ws-contracts";
import {
  applyRelayAuth,
  clearRelayAuth,
  relayAuthRevision,
} from "../src/platform/relay-auth";

class MockSocket implements TransportSocket {
  readyState = SOCKET_CONNECTING;
  bufferedAmount = 0;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string | ArrayBuffer }) => void) | null = null;
  onclose: ((event: { code: number; reason?: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  readonly sent: (string | Uint8Array)[] = [];
  closed = false;

  constructor(
    readonly label: string,
    readonly path: string,
    readonly options: StreamOptions,
    readonly transportPath: TransportPath,
  ) {}

  open() {
    this.readyState = SOCKET_OPEN;
    this.onopen?.();
  }

  fail(code = 1006) {
    this.readyState = SOCKET_CLOSED;
    this.onerror?.();
    this.onclose?.({ code });
  }

  deliverText(text: string) {
    this.onmessage?.({ data: text });
  }

  deliverFrame(kind: number, body: string) {
    const payload = new TextEncoder().encode(body);
    const frame = new Uint8Array(payload.length + 1);
    frame[0] = kind;
    frame.set(payload, 1);
    this.onmessage?.({ data: frame.buffer });
  }

  send(data: string | Uint8Array) {
    this.sent.push(typeof data === "string" ? data : data.slice());
  }

  close() {
    this.readyState = SOCKET_CLOSED;
    this.closed = true;
  }
}

function makeTransport(
  name: string,
  sink: MockSocket[],
  transportPath: TransportPath = name === "ws" ? "wss" : "webrtc-direct",
): ClientTransport {
  return {
    origin: "https://term.example.com",
    fetch: vi.fn(),
    upload: vi.fn(),
    openStream(path, options) {
      const sock = new MockSocket(name, path, options, transportPath);
      sink.push(sock);
      return sock;
    },
  };
}

function queryIds(socket: MockSocket) {
  return socket.sent.flatMap((frame) => {
    if (typeof frame === "string" || frame[0] !== 4) return [];
    return z
      .object({ ids: z.array(z.string()) })
      .parse(JSON.parse(new TextDecoder().decode(frame.subarray(1)))).ids;
  });
}

function acknowledgeFence(socket: MockSocket, id = queryIds(socket).at(-1)!) {
  socket.deliverFrame(5, JSON.stringify({ id, state: "missing" }));
}

describe("Make-before-break transport handoff", () => {
  const cleanups: (() => void)[] = [];

  beforeEach(() => {
    vi.stubGlobal("localStorage", {
      getItem: () => "token-1",
      setItem: () => undefined,
      removeItem: () => undefined,
    });
    vi.stubGlobal("document", new EventTarget());
    vi.stubGlobal(
      "window",
      Object.assign(new EventTarget(), { setTimeout: globalThis.setTimeout }),
    );
  });

  afterEach(() => {
    clearRelayAuth();
    while (cleanups.length > 0) cleanups.pop()?.();
    setClientTransport(null);
    vi.unstubAllGlobals();
  });

  it("keeps baseline until the candidate supplies an authoritative list snapshot", () => {
    const wsSockets: MockSocket[] = [];
    const p2pSockets: MockSocket[] = [];
    setClientTransport(makeTransport("ws", wsSockets));

    const states: string[] = [];
    const messages: string[] = [];
    const conn = connect(
      "/ws/sessions",
      (data) => messages.push(String(data)),
      (st) => states.push(st),
      vi.fn(),
    );
    cleanups.push(() => conn.close());

    expect(wsSockets).toHaveLength(1);
    wsSockets[0]!.open();
    wsSockets[0]!.deliverText("from-ws-1");
    expect(states).toEqual(["connecting", "connected"]);

    // Switch active transport to P2P and trigger make-before-break handoff.
    setClientTransport(makeTransport("p2p", p2pSockets));
    handoffLiveSockets();
    expect(p2pSockets).toHaveLength(1);
    expect(wsSockets[0]!.closed).toBe(false);
    wsSockets[0]!.deliverText("from-ws-during-handoff");

    // Channel open is not restoration. The old writer stays authoritative.
    p2pSockets[0]!.open();
    expect(wsSockets[0]!.closed).toBe(false);
    p2pSockets[0]!.deliverText(
      JSON.stringify({ type: "snapshot", sessions: [], forwards: [] }),
    );
    expect(wsSockets[0]!.closed).toBe(true);
    expect(states).toEqual(["connecting", "connected", "connected"]);
    expect(states).not.toContain("reconnecting");

    p2pSockets[0]!.deliverText("from-p2p-1");
    expect(messages).toEqual([
      "from-ws-1",
      "from-ws-during-handoff",
      '{"type":"snapshot","sessions":[],"forwards":[]}',
      "from-p2p-1",
    ]);

    conn.close();
  });

  it("retains the healthy baseline socket when a candidate P2P stream fails before opening", () => {
    const wsSockets: MockSocket[] = [];
    const p2pSockets: MockSocket[] = [];
    setClientTransport(makeTransport("ws", wsSockets));

    const states: string[] = [];
    const messages: string[] = [];
    const conn = connect(
      "/ws/sessions",
      (data) => messages.push(String(data)),
      (st) => states.push(st),
      vi.fn(),
    );
    cleanups.push(() => conn.close());

    wsSockets[0]!.open();
    setClientTransport(makeTransport("p2p", p2pSockets));
    handoffLiveSockets();

    expect(p2pSockets).toHaveLength(1);
    p2pSockets[0]!.fail(1006);

    expect(wsSockets[0]!.closed).toBe(false);
    expect(states).toEqual(["connecting", "connected"]);
    wsSockets[0]!.deliverText("still-alive-on-ws");
    expect(messages).toEqual(["still-alive-on-ws"]);

    conn.close();
  });

  it("drains the sole terminal input writer before handoff without replay or false uncertainty", () => {
    const baseline: MockSocket[] = [];
    const candidate: MockSocket[] = [];
    setClientTransport(makeTransport("ws", baseline));
    const error = vi.fn();
    const conn = connect(
      "/ws/s/s1?generation=0123456789abcdef&offset=4",
      vi.fn(),
      vi.fn(),
      error,
    );
    cleanups.push(() => conn.close());
    baseline[0]!.open();
    expect(conn.send(0, new Uint8Array([65]))).toBe(true);
    setClientTransport(makeTransport("p2p", candidate));
    handoffLiveSockets();
    candidate[0]!.open();
    expect(conn.send(0, new Uint8Array([66]))).toBe(true);
    expect(candidate[0]!.sent).toHaveLength(0);
    expect(baseline[0]!.sent).toHaveLength(2);
    candidate[0]!.deliverFrame(
      4,
      JSON.stringify({
        mode: "append",
        generation: "0123456789abcdef",
        start: 4,
        end: 6,
        truncated: false,
      }),
    );
    expect(baseline[0]!.closed).toBe(false);
    acknowledgeFence(baseline[0]!);
    expect(baseline[0]!.closed).toBe(true);
    expect(error).not.toHaveBeenCalled();
    expect(conn.send(0, new Uint8Array([67]))).toBe(true);
    expect(candidate[0]!.sent).toEqual([new Uint8Array([0, 67])]);
    expect(
      baseline[0]!.sent.filter(
        (frame) => typeof frame !== "string" && frame[0] === 0,
      ),
    ).toEqual([new Uint8Array([0, 65]), new Uint8Array([0, 66])]);
  });

  function terminalHandoff(
    canHandoff?: (position: StreamPosition) => boolean,
    handoffOptions?: HandoffOptions,
  ) {
    const baseline: MockSocket[] = [];
    const candidate: MockSocket[] = [];
    const error = vi.fn();
    setClientTransport(makeTransport("ws", baseline));
    const conn = connect("/ws/s/s1", vi.fn(), vi.fn(), error, { canHandoff });
    cleanups.push(() => conn.close());
    const old = baseline[0]!;
    old.open();
    setClientTransport(makeTransport("p2p", candidate));
    handoffLiveSockets(handoffOptions);
    const next = candidate[0]!;
    next.open();
    const ready = () =>
      next.deliverFrame(
        4,
        JSON.stringify({
          mode: "append",
          generation: "0123456789abcdef",
          start: 0,
          end: 4,
          truncated: false,
        }),
      );
    return { conn, old, next, ready, error, candidate };
  }

  describe("Explicit fast-path reconnect recovery", () => {
    const force: HandoffOptions = {
      targetPath: "webrtc-direct",
      allowReconnect: true,
    };

    beforeEach(() => {
      vi.useFakeTimers();
      window.setTimeout = globalThis.setTimeout;
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("retains the baseline after the default seamless staging deadline", () => {
      const { old, next, candidate, error } = terminalHandoff();
      vi.advanceTimersByTime(10_000);
      expect(old.closed).toBe(false);
      expect(next.closed).toBe(true);
      expect(candidate).toHaveLength(1);
      expect(error).not.toHaveBeenCalled();
    });

    it("reattaches an unopened interactive stream once through the existing open path", () => {
      const baseline: MockSocket[] = [];
      const fast: MockSocket[] = [];
      setClientTransport(makeTransport("ws", baseline));
      const conn = connect("/ws/sessions", vi.fn(), vi.fn(), vi.fn());
      cleanups.push(() => conn.close());
      setClientTransport(makeTransport("p2p", fast));
      handoffLiveSockets(force);
      expect(baseline[0]!.closed).toBe(true);
      expect(fast).toHaveLength(1);
      handoffLiveSockets(force);
      expect(fast).toHaveLength(1);
      expect(fast[0]!.closed).toBe(false);
    });

    it.each(["terminal", "chat", "list"] as const)(
      "recovers the %s consumer through a fresh cursor/snapshot after bounded staging",
      (kind) => {
        const baseline: MockSocket[] = [];
        const fast: MockSocket[] = [];
        const route =
          kind === "list"
            ? "/ws/sessions"
            : kind === "chat"
              ? "/ws/s/s1/chat?generation=chat-run&seq="
              : "/ws/s/s1?generation=0123456789abcdef&offset=";
        let cursor = 4;
        const messages: (string | ArrayBuffer)[] = [];
        const states: string[] = [];
        let output = "";
        let chat = emptyChat();
        let sessions: SessionInfo[] | undefined;
        const error = vi.fn();
        setClientTransport(makeTransport("ws", baseline));
        const conn = connect(
          () => (kind === "list" ? route : route + cursor),
          (data) => {
            messages.push(data);
            if (kind === "terminal" && typeof data !== "string") {
              const frame = new Uint8Array(data);
              const body = new TextDecoder().decode(frame.subarray(1));
              if (
                frame[0] === 4 &&
                StreamPositionSchema.parse(JSON.parse(body)).mode === "reset"
              )
                output = "";
              else if (frame[0] === 1) output += body;
            } else if (typeof data === "string") {
              if (kind === "chat")
                chat = applyFrame(
                  chat,
                  ChatFrameSchema.parse(JSON.parse(data)),
                );
              else
                sessions = applySessionEvent(
                  sessions ?? [],
                  SessionEventSchema.parse(JSON.parse(data)),
                );
            }
          },
          (state) => states.push(state),
          error,
        );
        cleanups.push(() => conn.close());
        const old = baseline[0]!;
        old.open();
        const transport = makeTransport("p2p", fast);
        setClientTransport(transport);
        handoffLiveSockets(force);
        expect(fast).toHaveLength(1);
        // The baseline can advance while a candidate remains stuck opening.
        cursor = 8;
        vi.advanceTimersByTime(9_999);
        expect(old.closed).toBe(false);
        vi.advanceTimersByTime(1);
        expect(old.closed).toBe(true);
        expect(fast[0]!.closed).toBe(true);
        expect(fast).toHaveLength(2);
        const recovered = fast[1]!;
        expect(recovered.path).toBe(kind === "list" ? route : route + "8");
        expect(recovered.options.token).toBe("token-1");
        expect(recovered.transportPath).toBe("webrtc-direct");
        expect(recovered.sent).toEqual([]);
        recovered.open();
        if (kind === "terminal") {
          recovered.deliverFrame(
            4,
            JSON.stringify({
              mode: "reset",
              generation: "0123456789abcdef",
              start: 0,
              end: 9,
              truncated: false,
            }),
          );
          recovered.deliverFrame(1, "recovered");
          expect(
            new TextDecoder().decode(
              new Uint8Array(messages[1] as ArrayBuffer).subarray(1),
            ),
          ).toBe("recovered");
          expect(output).toBe("recovered");
        } else {
          const snapshot =
            kind === "list"
              ? { type: "snapshot", sessions: [], forwards: [] }
              : {
                  type: "snapshot",
                  generation: "chat-run",
                  from: 8,
                  state: {
                    outdated: [],
                    eligibility: { available: true },
                    turn: { running: false },
                    requests: [],
                    config: [],
                    commands: [],
                    plan: null,
                    usage: null,
                    queue: { paused: false, items: [] },
                    steering: true,
                    forceSwitchNote: "recovered",
                    authRequired: false,
                    permissionMode: "auto",
                    unsupported: [],
                    idChange: null,
                    recovery: null,
                  },
                };
          recovered.deliverText(JSON.stringify(snapshot));
          expect(messages).toEqual([JSON.stringify(snapshot)]);
          if (kind === "list") expect(sessions).toEqual([]);
          else {
            expect(resumePoint(chat)).toEqual({
              generation: "chat-run",
              seq: 7,
            });
            const reply = JSON.stringify({
              type: "events",
              generation: "chat-run",
              items: [
                {
                  seq: 8,
                  at: "2026-11-15T00:00:00Z",
                  kind: "message.append",
                  message: {
                    messageId: "reply",
                    role: "agent",
                    text: "recovered",
                  },
                },
              ],
            });
            recovered.deliverText(reply);
            recovered.deliverText(reply);
            expect(chat.entries).toMatchObject([
              { type: "message", text: "recovered" },
            ]);
            expect(resumePoint(chat)?.seq).toBe(8);
          }
        }
        expect(states).toEqual([
          "connecting",
          "connected",
          "connecting",
          "connected",
        ]);
        expect(error).not.toHaveBeenCalled();
        // A later policy round must not reopen a stream already on target.
        handoffLiveSockets(force);
        expect(fast).toHaveLength(2);
      },
    );

    it("coalesces policy rounds and upgrades permission without resetting the staging deadline", () => {
      const { conn, old, next, candidate, error } = terminalHandoff(undefined, {
        targetPath: "webrtc-direct",
      });
      conn.send(0, new Uint8Array([65]));
      vi.advanceTimersByTime(5_000);
      handoffLiveSockets(force);
      expect(candidate).toHaveLength(1);
      expect(next.closed).toBe(false);
      vi.advanceTimersByTime(4_999);
      expect(old.closed).toBe(false);
      vi.advanceTimersByTime(1);
      expect(old.closed).toBe(true);
      expect(candidate).toHaveLength(2);
      expect(candidate[1]!.sent).toEqual([]);
      expect(error).toHaveBeenCalledTimes(1);
    });

    it("interrupts a stalled input fence, reports uncertainty, and never replays input or submissions", () => {
      const { conn, old, next, ready, candidate, error } = terminalHandoff(
        undefined,
        force,
      );
      conn.send(0, new Uint8Array([65]));
      conn.send(3, new Uint8Array([66]));
      ready();
      const lateReceipt = old.onmessage!;
      const id = queryIds(old)[0]!;
      vi.advanceTimersByTime(resumeTimeout);
      expect(old.closed).toBe(true);
      expect(next.closed).toBe(true);
      expect(error).toHaveBeenCalledTimes(1);
      expect(error).toHaveBeenCalledWith(expect.stringContaining("无法确认"));
      expect(conn.send(0, new Uint8Array([67]))).toBe(false);
      const recovered = candidate[1]!;
      recovered.open();
      lateReceipt({
        data: new Uint8Array([
          5,
          ...new TextEncoder().encode(JSON.stringify({ id, state: "missing" })),
        ]).buffer,
      });
      expect(recovered.sent).toEqual([]);
      expect(conn.send(0, new Uint8Array([68]))).toBe(true);
      expect(recovered.sent).toEqual([new Uint8Array([0, 68])]);
      expect(
        old.sent.filter((frame) => typeof frame !== "string" && frame[0] !== 4),
      ).toEqual([new Uint8Array([0, 65]), new Uint8Array([3, 66])]);
    });

    it.each(["invalid", "incompatible", "closed"] as const)(
      "recovers a failed %s staging candidate only when explicitly allowed",
      (failure) => {
        const { old, next, ready, candidate } = terminalHandoff(
          () => false,
          force,
        );
        if (failure === "invalid") next.deliverFrame(4, "{}");
        else if (failure === "closed") next.fail();
        else {
          ready();
          expect(next.sent).toEqual([new Uint8Array([2])]);
          expect(old.closed).toBe(false);
          vi.advanceTimersByTime(10_000);
        }
        expect(old.closed).toBe(true);
        expect(next.closed).toBe(true);
        expect(candidate).toHaveLength(2);
        expect(candidate[1]!.sent).toEqual([]);
      },
    );

    it("keeps an explicitly forced healthy candidate make-before-break with the normal terminal fence", () => {
      const { conn, old, next, ready, candidate, error } = terminalHandoff(
        undefined,
        force,
      );
      conn.send(0, new Uint8Array([65]));
      ready();
      expect(old.closed).toBe(false);
      acknowledgeFence(old);
      expect(old.closed).toBe(true);
      expect(next.closed).toBe(false);
      expect(candidate).toHaveLength(1);
      expect(error).not.toHaveBeenCalled();
      vi.advanceTimersByTime(10_000);
      handoffLiveSockets(force);
      expect(candidate).toHaveLength(1);
    });

    it.each(["staging", "fence", "snapshot"] as const)(
      "cancels a stale %s migration when current transport identity changes",
      (phase) => {
        const { conn, old, next, ready, candidate, error } = terminalHandoff(
          undefined,
          force,
        );
        conn.send(0, new Uint8Array([65]));
        if (phase === "fence") ready();
        const replacement: MockSocket[] = [];
        // Even another transport with the same path is a different session.
        setClientTransport(makeTransport("p2p-replacement", replacement));
        if (phase === "snapshot") ready();
        vi.advanceTimersByTime(10_000);
        expect(old.closed).toBe(false);
        expect(next.closed).toBe(true);
        expect(candidate).toHaveLength(1);
        expect(replacement).toHaveLength(0);
        expect(conn.send(0, new Uint8Array([66]))).toBe(true);
        expect(old.sent.at(-1)).toEqual(new Uint8Array([0, 66]));
        expect(error).not.toHaveBeenCalled();
      },
    );

    it.each(["close", "revoke"] as const)(
      "cancels forced timers and late callbacks after %s",
      (action) => {
        applyRelayAuth(
          {
            authenticated: true,
            csrfToken: "csrf",
            expiresAt: Math.floor(Date.now() / 1000) + 60,
          },
          relayAuthRevision(),
        );
        const { conn, old, next, ready, candidate } = terminalHandoff(
          undefined,
          force,
        );
        ready();
        const lateFailure = next.onerror!;
        if (action === "close") conn.close();
        else clearRelayAuth();
        lateFailure();
        vi.advanceTimersByTime(10_000);
        expect(old.closed).toBe(true);
        expect(next.closed).toBe(true);
        expect(candidate).toHaveLength(1);
        expect(conn.send(0, new Uint8Array([65]))).toBe(false);
      },
    );

    it.each([SOCKET_CONNECTING, SOCKET_OPEN, SOCKET_CLOSED])(
      "skips already-target and bulk streams before reconnecting (state %i)",
      (readyState) => {
        const baseline: MockSocket[] = [];
        const fast: MockSocket[] = [];
        setClientTransport(makeTransport("ws", baseline));
        for (const route of [
          "/ws/s/s1/download",
          "/ws/s/s1/upload",
          "/ws/files",
        ]) {
          const conn = connect(route, vi.fn(), vi.fn(), vi.fn());
          cleanups.push(() => conn.close());
          baseline.at(-1)!.readyState = readyState;
        }
        setClientTransport(makeTransport("p2p", fast));
        const conn = connect("/ws/sessions", vi.fn(), vi.fn(), vi.fn());
        cleanups.push(() => conn.close());
        fast[0]!.readyState = readyState;
        handoffLiveSockets(force);
        expect(baseline).toHaveLength(3);
        expect(fast).toHaveLength(1);
        expect(baseline.every((socket) => !socket.closed)).toBe(true);
        expect(fast[0]!.closed).toBe(false);
      },
    );

    it("never forces onto a transport whose actual path differs from the requested target", () => {
      const baseline: MockSocket[] = [];
      const wrong: MockSocket[] = [];
      setClientTransport(makeTransport("ws", baseline));
      const conn = connect("/ws/sessions", vi.fn(), vi.fn(), vi.fn());
      cleanups.push(() => conn.close());
      baseline[0]!.open();
      setClientTransport(makeTransport("turn", wrong, "webrtc-turn"));
      handoffLiveSockets(force);
      vi.advanceTimersByTime(10_000);
      expect(baseline[0]!.closed).toBe(false);
      expect(wrong).toHaveLength(1);
      expect(wrong[0]!.closed).toBe(true);
    });
  });

  it.each([0, 3, 6])(
    "coalesces delayed fences behind every newer input-lane send (kind %i)",
    (kind) => {
      const { conn, old, next, ready, error } = terminalHandoff();
      const encode = (text: string) => new TextEncoder().encode(text);
      expect(conn.send(0, encode("TURN_"))).toBe(true);
      expect(queryIds(old)).toEqual([]);
      ready();
      const first = queryIds(old)[0]!;
      expect(first).toMatch(/^hf-[a-f0-9]{32}$/);
      for (const key of "ACTUAL_")
        expect(conn.send(kind, encode(key))).toBe(true);
      expect(queryIds(old)).toEqual([first]);
      expect(next.sent).toEqual([]);
      old.deliverFrame(0, "unrelated output");
      acknowledgeFence(old, "unrelated-receipt");
      next.deliverFrame(5, JSON.stringify({ id: first, state: "missing" }));
      old.deliverFrame(5, JSON.stringify({ id: first, state: "written" }));
      expect(old.closed).toBe(false);
      acknowledgeFence(old, first);
      const second = queryIds(old)[1]!;
      expect(second).not.toBe(first);
      expect(old.closed).toBe(false);
      for (const key of "OK") expect(conn.send(kind, encode(key))).toBe(true);
      acknowledgeFence(old, first);
      expect(queryIds(old)).toHaveLength(2);
      acknowledgeFence(old, second);
      expect(queryIds(old)).toHaveLength(3);
      expect(old.closed).toBe(false);
      acknowledgeFence(old);
      expect(old.closed).toBe(true);
      expect(error).not.toHaveBeenCalled();
      expect(conn.send(0, new Uint8Array([10]))).toBe(true);
      const input = old.sent.filter(
        (frame): frame is Uint8Array =>
          frame instanceof Uint8Array && frame[0] !== 4,
      );
      expect(
        input
          .map((frame) => new TextDecoder().decode(frame.subarray(1)))
          .join(""),
      ).toBe("TURN_ACTUAL_OK");
      expect(next.sent).toEqual([new Uint8Array([0, 10])]);
    },
  );

  it("cancels an unacknowledged fence despite output without disturbing healthy input", () => {
    vi.useFakeTimers();
    try {
      const { conn, old, next, ready, error } = terminalHandoff();
      ready();
      const stale = queryIds(old)[0]!;
      old.deliverFrame(0, "healthy output");
      vi.advanceTimersByTime(resumeTimeout);
      expect(next.closed).toBe(true);
      expect(old.closed).toBe(false);
      acknowledgeFence(old, stale);
      expect(conn.send(0, new Uint8Array([65]))).toBe(true);
      expect(old.sent.at(-1)).toEqual(new Uint8Array([0, 65]));
      expect(error).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  it("cancels a saturated fence without closing the baseline", () => {
    const { conn, old, next, ready } = terminalHandoff();
    old.bufferedAmount = 1024 * 1024;
    ready();
    expect(next.closed).toBe(true);
    expect(old.closed).toBe(false);
    old.bufferedAmount = 0;
    expect(conn.send(0, new Uint8Array([65]))).toBe(true);
    expect(old.sent).toEqual([new Uint8Array([0, 65])]);
  });

  it("revalidates replay against baseline advancement while the input fence waits", () => {
    let accepts = true;
    const { old, next, ready, error } = terminalHandoff(() => accepts);
    ready();
    accepts = false;
    acknowledgeFence(old);
    expect(old.closed).toBe(false);
    expect(next.sent).toEqual([new Uint8Array([2])]);
    next.deliverFrame(
      4,
      JSON.stringify({
        mode: "reset",
        generation: "0123456789abcdef",
        start: 0,
        end: 4,
        truncated: false,
      }),
    );
    next.deliverFrame(1, "safe");
    expect(old.closed).toBe(false);
    acknowledgeFence(old);
    expect(old.closed).toBe(true);
    expect(error).not.toHaveBeenCalled();
  });

  it("retains genuine disconnect uncertainty and discards the pending fence", () => {
    const { conn, old, next, ready, error } = terminalHandoff();
    conn.send(0, new Uint8Array([65]));
    ready();
    const receipt = old.onmessage!;
    const fence = queryIds(old)[0]!;
    old.fail();
    expect(next.closed).toBe(true);
    expect(error).toHaveBeenCalledWith(expect.stringContaining("无法确认"));
    receipt({
      data: new Uint8Array([
        5,
        ...new TextEncoder().encode(
          JSON.stringify({
            id: fence,
            state: "missing",
          }),
        ),
      ]).buffer,
    });
    expect(conn.send(0, new Uint8Array([66]))).toBe(false);
    expect(next.sent).toEqual([]);
  });

  it("discards a candidate that exceeds its replay budget while the fence waits", () => {
    const { conn, old, next, ready } = terminalHandoff();
    ready();
    next.deliverText("x".repeat(512 * 1024 + 1));
    expect(next.closed).toBe(true);
    acknowledgeFence(old);
    expect(old.closed).toBe(false);
    expect(conn.send(0, new Uint8Array([65]))).toBe(true);
    expect(old.sent.at(-1)).toEqual(new Uint8Array([0, 65]));
  });

  it("cancels a failed fence send without closing or replaying onto the healthy baseline", () => {
    const { conn, old, next, ready, error } = terminalHandoff();
    const send = old.send;
    old.send = () => {
      throw new Error("query send failed");
    };
    ready();
    expect(next.closed).toBe(true);
    expect(old.closed).toBe(false);
    old.send = send;
    expect(conn.send(0, new Uint8Array([65]))).toBe(true);
    expect(old.sent).toEqual([new Uint8Array([0, 65])]);
    expect(error).not.toHaveBeenCalled();
  });

  it("keeps the old writer if candidate closure precedes its close event", () => {
    const { conn, old, next, ready, error } = terminalHandoff();
    ready();
    next.readyState = SOCKET_CLOSED;
    acknowledgeFence(old);
    expect(old.closed).toBe(false);
    expect(next.closed).toBe(true);
    expect(conn.send(0, new Uint8Array([65]))).toBe(true);
    expect(old.sent.at(-1)).toEqual(new Uint8Array([0, 65]));
    expect(error).not.toHaveBeenCalled();
  });

  it.each(["close", "revoke"] as const)(
    "discards late fence receipts after %s",
    (action) => {
      applyRelayAuth(
        {
          authenticated: true,
          csrfToken: "csrf",
          expiresAt: Math.floor(Date.now() / 1000) + 60,
        },
        relayAuthRevision(),
      );
      const { conn, old, next, ready } = terminalHandoff();
      ready();
      const late = old.onmessage!;
      const id = queryIds(old)[0]!;
      if (action === "close") conn.close();
      else clearRelayAuth();
      late({
        data: new Uint8Array([
          5,
          ...new TextEncoder().encode(JSON.stringify({ id, state: "missing" })),
        ]).buffer,
      });
      expect(old.closed).toBe(true);
      expect(next.closed).toBe(true);
      expect(conn.send(0, new Uint8Array([65]))).toBe(false);
      expect(next.sent).toEqual([]);
    },
  );

  it("keeps baseline on invalid recovery metadata and waits for a reset snapshot before switching", () => {
    const baseline: MockSocket[] = [];
    const candidate: MockSocket[] = [];
    setClientTransport(makeTransport("ws", baseline));
    const conn = connect("/ws/s/s1", vi.fn(), vi.fn(), vi.fn());
    cleanups.push(() => conn.close());
    baseline[0]!.open();
    setClientTransport(makeTransport("p2p", candidate));
    handoffLiveSockets();
    candidate[0]!.open();
    candidate[0]!.deliverFrame(4, "{}");
    expect(baseline[0]!.closed).toBe(false);
    expect(candidate[0]!.closed).toBe(true);
    handoffLiveSockets();
    candidate[1]!.open();
    candidate[1]!.deliverFrame(
      4,
      JSON.stringify({
        mode: "reset",
        generation: "0123456789abcdef",
        start: 0,
        end: 4,
        truncated: false,
      }),
    );
    expect(baseline[0]!.closed).toBe(false);
    candidate[1]!.deliverFrame(1, "safe");
    expect(baseline[0]!.closed).toBe(false);
    acknowledgeFence(baseline[0]!);
    expect(baseline[0]!.closed).toBe(true);
  });

  it("revokes Cookie-derived streams without closing independent Token streams", () => {
    const sockets: MockSocket[] = [];
    setClientTransport(makeTransport("ws", sockets));
    const local = connect("/ws/sessions", vi.fn(), vi.fn(), vi.fn());
    cleanups.push(() => local.close());
    applyRelayAuth(
      {
        authenticated: true,
        csrfToken: "csrf",
        expiresAt: Math.floor(Date.now() / 1000) + 60,
      },
      relayAuthRevision(),
    );
    const owner = connect("/ws/sessions", vi.fn(), vi.fn(), vi.fn());
    cleanups.push(() => owner.close());
    expect(sockets[0]!.options.token).toBe("token-1");
    expect(sockets[1]!.options.token).toBe("");
    clearRelayAuth();
    expect(sockets[0]!.closed).toBe(false);
    expect(sockets[1]!.closed).toBe(true);
  });

  it("computes overlap skip and trims duplicate bytes across multi-chunk append replay", () => {
    const generation = "0123456789abcdef";

    // Exact match: no bytes to skip, tail exists.
    expect(
      resolveAppendOverlap(true, generation, 5, {
        generation,
        start: 5,
        end: 11,
      }),
    ).toEqual({ accept: true, skipBytes: 0, hasTail: true });

    // Partial overlap during make-before-break: baseline advanced to 8 while candidate started at 5.
    const decision = resolveAppendOverlap(true, generation, 8, {
      generation,
      start: 5,
      end: 11,
    });
    expect(decision).toEqual({ accept: true, skipBytes: 3, hasTail: true });

    const encoder = new TextEncoder();
    const decoder = new TextDecoder();
    // Replay arrives in two chunks: "_W" (2 bytes, fully skipped) then "ORLD" (1 byte skipped -> "RLD")
    const c1 = trimOverlapChunk(encoder.encode("_W"), 3);
    expect(c1).toEqual({ remainingSkip: 1, slice: null });
    const c2 = trimOverlapChunk(encoder.encode("ORLD"), c1.remainingSkip);
    expect(c2.remainingSkip).toBe(0);
    expect(decoder.decode(c2.slice!)).toBe("RLD");

    // Full overlap (baseline caught up to candidate's end=11): accept, skip 6, no tail.
    expect(
      resolveAppendOverlap(true, generation, 11, {
        generation,
        start: 5,
        end: 11,
      }),
    ).toEqual({ accept: true, skipBytes: 6, hasTail: false });

    // Rejects mismatched generation, unsynced state, or gap (start > currentOffset).
    expect(
      resolveAppendOverlap(false, generation, 5, {
        generation,
        start: 5,
        end: 11,
      }),
    ).toEqual({ accept: false });
    expect(
      resolveAppendOverlap(true, "other-generation", 5, {
        generation,
        start: 5,
        end: 11,
      }),
    ).toEqual({ accept: false });
    expect(
      resolveAppendOverlap(true, generation, 4, {
        generation,
        start: 5,
        end: 11,
      }),
    ).toEqual({ accept: false });
  });
});

it("does not invite replay when a terminal send throws after accepting bytes", () => {
  vi.stubGlobal("document", new EventTarget());
  vi.stubGlobal("localStorage", { getItem: () => "token-1" });
  vi.stubGlobal(
    "window",
    Object.assign(new EventTarget(), { setTimeout: globalThis.setTimeout }),
  );
  const sockets: MockSocket[] = [];
  setClientTransport(makeTransport("ws", sockets));
  const error = vi.fn();
  const conn = connect("/ws/s/s1", vi.fn(), vi.fn(), error);
  try {
    const old = sockets[0]!;
    old.open();
    old.send = (data) => {
      old.sent.push(typeof data === "string" ? data : data.slice());
      throw new Error("unknown accepted prefix");
    };
    expect(conn.send(0, new Uint8Array([65]))).toBe(true);
    expect(error).toHaveBeenCalledWith(expect.stringContaining("无法确认"));
    conn.reconnect();
    sockets[1]!.open();
    expect(sockets[1]!.sent).toEqual([]);
    expect(conn.send(0, new Uint8Array([66]))).toBe(true);
    expect(sockets[1]!.sent).toEqual([new Uint8Array([0, 66])]);
  } finally {
    conn.close();
    setClientTransport(null);
    vi.unstubAllGlobals();
  }
});
