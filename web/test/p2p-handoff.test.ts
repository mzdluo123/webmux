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
} from "../src/platform/transport";
import { connect, handoffLiveSockets } from "../src/platform/ws";
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

function makeTransport(name: string, sink: MockSocket[]): ClientTransport {
  return {
    origin: "https://term.example.com",
    fetch: vi.fn(),
    upload: vi.fn(),
    openStream(path, options) {
      const sock = new MockSocket(name, path, options);
      sink.push(sock);
      return sock;
    },
  };
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

  it("has exactly one terminal writer and never replays unknown input after a validated handoff", () => {
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
    expect(baseline[0]!.closed).toBe(true);
    expect(error).toHaveBeenCalledWith(expect.stringContaining("无法确认"));
    expect(conn.send(0, new Uint8Array([67]))).toBe(true);
    expect(candidate[0]!.sent).toEqual([new Uint8Array([0, 67])]);
    expect(baseline[0]!.sent).toEqual([
      new Uint8Array([0, 65]),
      new Uint8Array([0, 66]),
    ]);
  });

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
