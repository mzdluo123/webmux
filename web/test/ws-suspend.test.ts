import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vite-plus/test";
import { connect } from "../src/platform/ws";
import type {
  Connection,
  SocketConnection,
} from "../src/platform/ws-contracts";
import { resetRelayAuth } from "../src/platform/relay-auth";

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
  closedCode: number | undefined;
  closedReason: string | undefined;

  constructor(
    readonly url: string,
    readonly protocols: string[],
  ) {
    MockWebSocket.instances.push(this);
  }
  send(_frame: Uint8Array) {}
  close(code?: number, reason?: string) {
    this.readyState = 3;
    this.closedCode = code;
    this.closedReason = reason;
  }
}

describe("WebSocket suspend and resume", () => {
  let connection: SocketConnection | undefined;

  beforeEach(() => {
    vi.useFakeTimers();
    MockWebSocket.instances = [];
    vi.stubGlobal("WebSocket", MockWebSocket);
    vi.stubGlobal("location", { protocol: "http:", host: "localhost" });
    vi.stubGlobal("localStorage", { getItem: () => "token" });
    vi.stubGlobal(
      "document",
      Object.assign(new EventTarget(), { hidden: false }),
    );
    vi.stubGlobal(
      "window",
      Object.assign(new EventTarget(), { setTimeout: globalThis.setTimeout }),
    );
  });

  afterEach(() => {
    connection?.close();
    connection = undefined;
    resetRelayAuth();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("closes cleanly on suspend without reconnect backoff and reopens with latest path on resume", () => {
    let offset = 0;
    const states: Connection[] = [];
    connection = connect(
      () => `/ws/s/demo?offset=${offset}`,
      vi.fn(),
      (state) => states.push(state),
      vi.fn(),
    );

    expect(MockWebSocket.instances).toHaveLength(1);
    const first = MockWebSocket.instances[0]!;
    first.onopen?.();
    expect(states.at(-1)).toBe("connected");

    offset = 4096;
    connection.suspend();
    expect(first.closedCode).toBe(1000);
    expect(first.closedReason).toBe("suspended");

    // Timers and visibility/online events must not reopen a suspended stream.
    vi.advanceTimersByTime(60_000);
    window.dispatchEvent(new Event("online"));
    document.dispatchEvent(new Event("visibilitychange"));
    expect(MockWebSocket.instances).toHaveLength(1);

    // Resuming evaluates the path factory again with the preserved offset.
    connection.resume();
    expect(MockWebSocket.instances).toHaveLength(2);
    expect(MockWebSocket.instances[1]!.url).toBe(
      "ws://localhost/ws/s/demo?offset=4096",
    );
  });
});
