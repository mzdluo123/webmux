import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vite-plus/test";
import { connect, protocols } from "../src/platform/ws";
import {
  setWsCompression,
  wsCompression,
} from "../src/platform/ws-compression";
import type { SocketConnection } from "../src/platform/ws-contracts";
import {
  applyRelayAuth,
  clearRelayAuth,
  relayAuthRevision,
  resetRelayAuth,
} from "../src/platform/relay-auth";

const sendBudget = 1024 * 1024;

class MockWebSocket {
  static OPEN = 1;
  static instances: MockWebSocket[] = [];
  readyState = MockWebSocket.OPEN;
  bufferedAmount = 0;
  binaryType = "";
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string | ArrayBuffer }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  onerror: (() => void) | null = null;
  sent: Uint8Array[] = [];

  constructor(
    readonly url: string,
    readonly protocols: string[],
  ) {
    MockWebSocket.instances.push(this);
  }
  send(frame: Uint8Array) {
    this.sent.push(frame.slice());
    this.bufferedAmount += frame.byteLength;
  }
  close() {
    this.readyState = 3;
  }
}

describe("WebSocket upstream send budget", () => {
  let connection: SocketConnection;
  let socket: MockWebSocket;

  beforeEach(() => {
    vi.useFakeTimers();
    MockWebSocket.instances = [];
    vi.stubGlobal("WebSocket", MockWebSocket);
    vi.stubGlobal("location", { protocol: "http:", host: "localhost" });
    vi.stubGlobal("localStorage", { getItem: () => "tok en" });
    vi.stubGlobal("document", new EventTarget());
    vi.stubGlobal(
      "window",
      Object.assign(new EventTarget(), { setTimeout: globalThis.setTimeout }),
    );
    connection = connect("/ws/s/test", vi.fn(), vi.fn(), vi.fn());
    socket = MockWebSocket.instances[0]!;
  });

  afterEach(() => {
    connection.close();
    resetRelayAuth();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("authenticates through a subprotocol without putting the token in the URL", () => {
    expect(socket.url).toBe("ws://localhost/ws/s/test");
    expect(socket.protocols).toEqual(["webmux", "webmux.token.dG9rIGVu"]);
  });

  it("encodes any token as a valid base64url subprotocol", () => {
    expect(protocols("")).toEqual(["webmux"]);
    // UTF-8 bytes; "+" "/" become "-" "_" and "==" padding is dropped.
    expect(protocols("终")).toEqual(["webmux", "webmux.token.57uI"]);
    expect(protocols("~~~???a")).toEqual(["webmux", "webmux.token.fn5-Pz8_YQ"]);
  });

  it("rejects input when the socket queue is saturated", () => {
    socket.bufferedAmount = sendBudget;

    expect(connection.send(0, new Uint8Array([65]))).toBe(false);
    expect(socket.sent).toEqual([]);
    expect(socket.bufferedAmount).toBe(sendBudget);
  });

  it("counts the kind byte and rejects a frame that would cross the limit", () => {
    socket.bufferedAmount = sendBudget - 2;

    expect(connection.send(0, new Uint8Array([65, 66]))).toBe(false);
    expect(socket.sent).toEqual([]);
    expect(socket.bufferedAmount).toBe(sendBudget - 2);
  });

  it("accepts a frame that exactly fills the remaining budget", () => {
    socket.bufferedAmount = sendBudget - 3;

    expect(connection.send(0, new Uint8Array([65, 66]))).toBe(true);
    expect(socket.sent).toEqual([new Uint8Array([0, 65, 66])]);
    expect(socket.bufferedAmount).toBe(sendBudget);
    expect(connection.send(0, new Uint8Array([67]))).toBe(false);
    expect(socket.sent).toHaveLength(1);
  });

  it("rejects a single oversized frame even with an empty socket queue", () => {
    expect(connection.send(0, new Uint8Array(sendBudget))).toBe(false);
    expect(socket.sent).toEqual([]);
    expect(socket.bufferedAmount).toBe(0);
  });

  it("preserves input and control frame encoding on a healthy socket", () => {
    const payload = new TextEncoder().encode("你好");

    expect(connection.send(0, payload)).toBe(true);
    expect(connection.send(1, new Uint8Array([123, 125]))).toBe(true);
    expect(connection.send(2, new Uint8Array())).toBe(true);
    expect(socket.sent).toEqual([
      new Uint8Array([0, ...payload]),
      new Uint8Array([1, 123, 125]),
      new Uint8Array([2]),
    ]);
  });

  it("accepts new input after draining without replaying rejected input", () => {
    socket.bufferedAmount = sendBudget;
    expect(connection.send(0, new Uint8Array([65]))).toBe(false);

    socket.bufferedAmount = 0;
    expect(connection.send(0, new Uint8Array([66]))).toBe(true);
    expect(socket.sent).toEqual([new Uint8Array([0, 66])]);
    expect(MockWebSocket.instances).toHaveLength(1);
  });

  it("coalesces controls until capacity returns without replaying rejected input", () => {
    socket.bufferedAmount = sendBudget;
    const latest = new Uint8Array([100, 40]);
    expect(connection.send(1, new Uint8Array([80, 24]))).toBe(true);
    expect(connection.send(1, latest)).toBe(true);
    latest[0] = 0;
    expect(connection.send(2, new Uint8Array())).toBe(true);
    expect(connection.send(2, new Uint8Array())).toBe(true);
    expect(connection.send(0, new Uint8Array([65]))).toBe(false);
    vi.advanceTimersByTime(200);
    expect(socket.sent).toEqual([]);
    expect(vi.getTimerCount()).toBe(1);

    socket.bufferedAmount = sendBudget - 4;
    vi.advanceTimersByTime(50);
    expect(socket.sent).toEqual([
      new Uint8Array([1, 100, 40]),
      new Uint8Array([2]),
    ]);
    expect(socket.bufferedAmount).toBe(sendBudget);
    expect(vi.getTimerCount()).toBe(0);
    socket.bufferedAmount = 0;
    vi.advanceTimersByTime(200);
    expect(socket.sent).toHaveLength(2);
  });

  it("replaces a deferred resize when a newer size can be sent immediately", () => {
    socket.bufferedAmount = sendBudget;
    connection.send(1, new Uint8Array([80, 24]));
    socket.bufferedAmount = 0;
    expect(connection.send(1, new Uint8Array([100, 40]))).toBe(true);
    vi.advanceTimersByTime(100);
    expect(socket.sent).toEqual([new Uint8Array([1, 100, 40])]);
    expect(vi.getTimerCount()).toBe(0);
  });

  it.each(["close", "reconnect"] as const)(
    "discards deferred controls on %s",
    (action) => {
      socket.bufferedAmount = sendBudget;
      connection.send(1, new Uint8Array([80, 24]));
      connection.send(2, new Uint8Array());
      connection[action]();
      socket.bufferedAmount = 0;
      vi.advanceTimersByTime(100);
      expect(socket.sent).toEqual([]);
      expect(
        MockWebSocket.instances.flatMap((instance) => instance.sent),
      ).toEqual([]);
      expect(vi.getTimerCount()).toBe(0);
    },
  );

  it("stops control retries when the server closes the session", () => {
    socket.bufferedAmount = sendBudget;
    connection.send(1, new Uint8Array([80, 24]));
    socket.readyState = 3;
    socket.onclose?.({ code: 4410 });
    vi.advanceTimersByTime(100);
    expect(socket.sent).toEqual([]);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("returns false without sending when the socket is closed", () => {
    connection.close();

    expect(connection.send(0, new Uint8Array([65]))).toBe(false);
    expect(socket.sent).toEqual([]);
  });

  it("suspends owner sockets and resumes the existing controller without replaying input", () => {
    connection.close();
    const session = {
      authenticated: true,
      relay: true,
      csrfToken: "owner",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    };
    applyRelayAuth(session, relayAuthRevision());
    connection = connect("/ws/s/test", vi.fn(), vi.fn(), vi.fn());
    socket = MockWebSocket.instances.at(-1)!;
    expect(connection.send(0, new Uint8Array([65]))).toBe(true);
    clearRelayAuth("reconnecting");
    expect(socket.readyState).toBe(3);
    expect(connection.send(0, new Uint8Array([66]))).toBe(false);
    window.dispatchEvent(new Event("online"));
    vi.advanceTimersByTime(60_000);
    expect(MockWebSocket.instances.at(-1)).toBe(socket);
    applyRelayAuth(session, relayAuthRevision());
    const recovered = MockWebSocket.instances.at(-1)!;
    expect(recovered).not.toBe(socket);
    expect(recovered.sent).toEqual([]);
    expect(connection.send(0, new Uint8Array([67]))).toBe(true);
    expect(recovered.sent).toEqual([new Uint8Array([0, 67])]);
    connection.close();
    clearRelayAuth("reconnecting");
    applyRelayAuth(session, relayAuthRevision());
    expect(MockWebSocket.instances.at(-1)).toBe(recovered);
  });
});

describe("WebSocket compression opt-in", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockWebSocket.instances = [];
    const items = new Map<string, string>();
    vi.stubGlobal("WebSocket", MockWebSocket);
    vi.stubGlobal("location", { protocol: "http:", host: "localhost" });
    vi.stubGlobal("localStorage", {
      getItem: (key: string) => items.get(key) ?? null,
      setItem: (key: string, value: string) => items.set(key, value),
    });
    vi.stubGlobal("document", new EventTarget());
    vi.stubGlobal(
      "window",
      Object.assign(new EventTarget(), { setTimeout: globalThis.setTimeout }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("reattaches live sockets with the opt-in and drops it when disabled", () => {
    const list = connect("/ws/sessions", vi.fn(), vi.fn(), vi.fn());
    const terminal = connect(
      () => "/ws/s/test?client=c",
      vi.fn(),
      vi.fn(),
      vi.fn(),
    );
    const closed = connect("/ws/settings", vi.fn(), vi.fn(), vi.fn());
    closed.close();

    setWsCompression(true);
    expect(MockWebSocket.instances.slice(3).map((s) => s.url)).toEqual([
      "ws://localhost/ws/sessions?compress=1",
      "ws://localhost/ws/s/test?client=c&compress=1",
    ]);

    setWsCompression(false);
    expect(MockWebSocket.instances.slice(5).map((s) => s.url)).toEqual([
      "ws://localhost/ws/sessions",
      "ws://localhost/ws/s/test?client=c",
    ]);
    list.close();
    terminal.close();
  });
});

describe("WebSocket compression default", () => {
  const items = new Map<string, string>();
  const device = (coarse: boolean) =>
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: coarse && query === "(pointer: coarse)",
    }));

  beforeEach(() => {
    items.clear();
    vi.stubGlobal("localStorage", {
      getItem: (key: string) => items.get(key) ?? null,
      setItem: (key: string, value: string) => items.set(key, value),
    });
  });

  afterEach(() => vi.unstubAllGlobals());

  it("is on for coarse-pointer devices and off for fine ones until chosen", () => {
    device(true);
    expect(wsCompression()).toBe(true);
    device(false);
    expect(wsCompression()).toBe(false);
  });

  it("lets an explicit choice override the device default", () => {
    device(true);
    setWsCompression(false);
    expect(wsCompression()).toBe(false);
    device(false);
    setWsCompression(true);
    expect(wsCompression()).toBe(true);
  });
});

describe("Terminal input confirmation and close codes", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockWebSocket.instances = [];
    vi.stubGlobal("WebSocket", MockWebSocket);
    vi.stubGlobal("location", { protocol: "http:", host: "localhost" });
    vi.stubGlobal("localStorage", { getItem: () => "tok" });
    vi.stubGlobal("document", new EventTarget());
    vi.stubGlobal(
      "window",
      Object.assign(new EventTarget(), { setTimeout: globalThis.setTimeout }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  function frame(kind: number, body: string): ArrayBuffer {
    const encoded = new TextEncoder().encode(body);
    const out = new Uint8Array(encoded.length + 1);
    out[0] = kind;
    out.set(encoded, 1);
    return out.buffer;
  }

  it("clears input uncertainty after FIFO query confirmation and follows up trailing keys", () => {
    let probeIndex = 0;
    const errors: string[] = [];
    const conn = connect(
      "/ws/s/test",
      vi.fn(),
      vi.fn(),
      (err) => errors.push(err),
      {
        resume: (send) =>
          send(
            4,
            new TextEncoder().encode(
              JSON.stringify({ ids: [`probe-000${++probeIndex}`] }),
            ),
          ),
      },
    );
    const socket = MockWebSocket.instances[0]!;
    socket.onopen?.();

    expect(conn.send(0, new Uint8Array([97]))).toBe(true);
    conn.probe();
    // A second key arrives while probe-0001 is still in flight.
    expect(conn.send(0, new Uint8Array([103]))).toBe(true);
    conn.probe();
    expect(probeIndex).toBe(1);

    // Output clears resumeTimer early, before probe-0001's receipt arrives.
    socket.onmessage?.({ data: frame(0, "a") });
    socket.onmessage?.({
      data: frame(5, JSON.stringify({ id: "probe-0001", state: "missing" })),
    });
    // Trailing key automatically triggers one follow-up probe.
    expect(probeIndex).toBe(2);

    socket.onmessage?.({
      data: frame(5, JSON.stringify({ id: "probe-0002", state: "missing" })),
    });
    // Once confirmed, reconnect or 4409 view move does not report uncertainty.
    socket.readyState = 3;
    socket.onclose?.({ code: 4409 });
    expect(errors).toEqual([]);
    conn.close();
  });

  it("does not report input uncertainty on clean 4410 session exit", () => {
    const errors: string[] = [];
    const conn = connect("/ws/s/test", vi.fn(), vi.fn(), (err) =>
      errors.push(err),
    );
    const socket = MockWebSocket.instances[0]!;
    socket.onopen?.();
    expect(conn.send(0, new Uint8Array([4]))).toBe(true);
    socket.readyState = 3;
    socket.onclose?.({ code: 4410 });
    expect(errors).toEqual([]);
    conn.close();
  });
});
