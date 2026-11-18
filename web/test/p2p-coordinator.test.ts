import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vite-plus/test";
import type { NegotiatedSession } from "../src/platform/p2p-session";
import type { P2PStatus } from "../src/contracts/schemas";
import type { ManagedStreamStatus } from "../src/platform/ws-contracts";
import {
  refreshP2PStatus,
  startP2P,
  stopP2P,
} from "../src/platform/p2p.svelte";
import { HYBRID_SAMPLE_INTERVAL_MS } from "../src/platform/p2p-framing";

const mocks = vi.hoisted(() => ({
  owner: false,
  streamListener: undefined as
    | ((streams: ManagedStreamStatus[]) => void)
    | undefined,
  state: {
    mode: "ws" as "ws" | "p2p",
    phase: "idle",
    reason: "",
    candidatePath: "unknown",
    switchCount: 0,
    streams: [] as ManagedStreamStatus[],
    inputUncertain: false,
  },
  status: {
    supported: true,
    enabled: true,
    stunServers: [],
    portMap: { state: "off" },
  },
  baseline: vi.fn<(count: number, signal: AbortSignal) => Promise<number[]>>(),
  inspect: vi.fn<
    () => Promise<{
      path: "webrtc-direct" | "webrtc-turn" | "unknown";
      candidateType: string;
      ipFamily: string;
    }>
  >(),
  negotiate: vi.fn<() => Promise<NegotiatedSession>>(),
  createTransport: vi.fn(() => ({ path: "fast-transport" })),
  setTransport: vi.fn(),
  handoff:
    vi.fn<
      (options?: { targetPath?: string; allowReconnect?: boolean }) => void
    >(),
}));

vi.mock("$lib/paraglide/messages.js", () => ({ m: {} }));
vi.mock("../src/platform/api", () => ({
  api: async () => mocks.status,
}));
vi.mock("../src/platform/p2p-session", () => ({
  negotiateDirectPeer: mocks.negotiate,
  inspectSelectedCandidatePair: mocks.inspect,
}));
vi.mock("../src/platform/p2p-signaling", () => ({
  measureBaselineHttpRtt: mocks.baseline,
}));
vi.mock("../src/platform/p2p-state.svelte", () => ({
  p2pState: mocks.state,
}));
vi.mock("../src/platform/p2p-transport", () => ({
  createP2PTransport: mocks.createTransport,
}));
vi.mock("../src/platform/storage", () => ({
  onActiveHostChange: () => () => {},
}));
vi.mock("../src/platform/relay-auth", () => ({
  relayOwnerAuthenticated: () => mocks.owner,
}));
vi.mock("../src/platform/transport", () => ({
  defaultBrowserTransport: () => ({ path: "wss" }),
  setClientTransport: mocks.setTransport,
}));
vi.mock("../src/platform/ws", () => ({
  handoffLiveSockets: mocks.handoff,
}));
vi.mock("../src/platform/ws-registry", () => ({
  observeManagedStreams: (
    listener: (streams: ManagedStreamStatus[]) => void,
  ) => {
    mocks.streamListener = listener;
    listener(mocks.state.streams);
    return () => {
      if (mocks.streamListener === listener) mocks.streamListener = undefined;
    };
  },
}));

function createSession(path: NegotiatedSession["path"] = "webrtc-direct") {
  return {
    peerId: "peer-1",
    pc: {
      connectionState: "connected",
      onconnectionstatechange: null,
    } as unknown as RTCPeerConnection,
    control: {
      readyState: "open",
      onclose: null,
      onerror: null,
      send: vi.fn(),
    } as unknown as RTCDataChannel,
    path,
    candidateType: path === "webrtc-turn" ? "relay" : "srflx",
    ipFamily: "ipv4",
    expiresAt: null as number | null,
    onFailure: null as NegotiatedSession["onFailure"],
    sendPing: vi.fn<() => Promise<number>>().mockResolvedValue(899),
    close: vi.fn(),
  } satisfies NegotiatedSession;
}

let session = createSession();

async function start(
  owner = false,
  path: NegotiatedSession["path"] = "webrtc-direct",
) {
  mocks.owner = owner;
  session.path = path;
  session.candidateType = path === "webrtc-turn" ? "relay" : "srflx";
  mocks.inspect.mockResolvedValue({
    path,
    candidateType: session.candidateType,
    ipFamily: "ipv4",
  });
  startP2P();
  await vi.advanceTimersByTimeAsync(owner ? 2_000 : 0);
}

async function rounds(count = 1) {
  await vi.advanceTimersByTimeAsync(HYBRID_SAMPLE_INTERVAL_MS * count);
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  vi.stubGlobal("RTCPeerConnection", class {});
  vi.stubGlobal("window", new EventTarget());
  vi.stubGlobal(
    "document",
    Object.assign(new EventTarget(), { visibilityState: "visible" }),
  );
  vi.stubGlobal("navigator", {});
  vi.stubGlobal("location", { hostname: "term.example.com", search: "" });
  mocks.owner = false;
  mocks.status.enabled = true;
  Object.assign(mocks.state, {
    mode: "ws",
    phase: "idle",
    reason: "",
    candidatePath: "unknown",
    switchCount: 0,
    streams: [],
    inputUncertain: false,
  });
  mocks.baseline.mockReset().mockResolvedValue([1_000]);
  mocks.inspect.mockReset();
  session = createSession();
  mocks.negotiate.mockReset().mockResolvedValue(session);
});

afterEach(() => {
  stopP2P();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("paired application RTT live-stream migration", () => {
  it.each([false, true])(
    "supplements active streams without replacing transport (owner=%s)",
    async (owner) => {
      await start(owner);
      expect(mocks.state.mode).toBe("p2p");
      expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
        targetPath: "webrtc-direct",
        allowReconnect: true,
      });
      expect(mocks.negotiate).toHaveBeenCalledTimes(1);
      expect(mocks.createTransport).toHaveBeenCalledTimes(1);
      const installations = mocks.setTransport.mock.calls.length;
      const switches = mocks.state.switchCount;

      await rounds(2);

      expect(mocks.handoff).toHaveBeenCalledTimes(3);
      expect(
        mocks.handoff.mock.calls.every(
          ([options]) =>
            options?.targetPath === "webrtc-direct" &&
            options.allowReconnect === true,
        ),
      ).toBe(true);
      expect(mocks.createTransport).toHaveBeenCalledTimes(1);
      expect(mocks.negotiate).toHaveBeenCalledTimes(1);
      expect(mocks.setTransport).toHaveBeenCalledTimes(installations);
      expect(mocks.state.switchCount).toBe(switches);
      expect(session.close).not.toHaveBeenCalled();
    },
  );

  it.each([120, 100])(
    "keeps a soft upgrade at %sms seamless until three strong active rounds",
    async (candidate) => {
      mocks.baseline.mockResolvedValue([200]);
      session.sendPing.mockResolvedValue(candidate);
      await start();
      expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
        targetPath: "webrtc-direct",
        allowReconnect: false,
      });
      mocks.handoff.mockClear();
      mocks.baseline.mockResolvedValue([1_000]);
      session.sendPing.mockResolvedValue(899);

      await rounds(2);
      expect(mocks.handoff).not.toHaveBeenCalled();
      await rounds();
      expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
        targetPath: "webrtc-direct",
        allowReconnect: true,
      });
      await rounds();
      expect(mocks.handoff).toHaveBeenCalledTimes(2);
    },
  );

  it.each([
    [1_000, 900],
    [1_000, NaN],
    [1_000, Infinity],
    [1_000, -1],
    [Infinity, 1],
    [-1, 0],
  ])(
    "resets strong rounds for baseline=%s candidate=%s",
    async (baseline, candidate) => {
      await start();
      mocks.handoff.mockClear();
      await rounds(2);
      expect(mocks.handoff).toHaveBeenCalledTimes(2);
      mocks.handoff.mockClear();
      mocks.baseline.mockResolvedValue([baseline]);
      session.sendPing.mockResolvedValue(candidate);
      await rounds();
      expect(mocks.handoff).not.toHaveBeenCalled();

      mocks.baseline.mockResolvedValue([1_000]);
      session.sendPing.mockResolvedValue(899);
      await rounds(2);
      expect(mocks.handoff).not.toHaveBeenCalled();
      await rounds();
      expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
        targetPath: "webrtc-direct",
        allowReconnect: true,
      });
    },
  );

  it("starts strong-round tracking anew for a replacement session", async () => {
    await start(true);
    stopP2P();
    mocks.handoff.mockClear();
    session = createSession();
    mocks.negotiate.mockResolvedValue(session);
    startP2P();
    await vi.advanceTimersByTimeAsync(0);
    expect(mocks.handoff).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1_000);
    expect(mocks.handoff).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1_000);
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
      targetPath: "webrtc-direct",
      allowReconnect: true,
    });
  });

  it("supplements an authorized owner TURN session with server P2P disabled", async () => {
    mocks.status.enabled = false;
    await start(true, "webrtc-turn");
    expect(mocks.createTransport).toHaveBeenCalledWith(
      expect.objectContaining({
        path: "webrtc-turn",
        interactiveOnly: true,
      }),
    );
    await rounds();
    expect(mocks.handoff).toHaveBeenLastCalledWith({
      targetPath: "webrtc-turn",
      allowReconnect: true,
    });
    expect(mocks.createTransport).toHaveBeenCalledTimes(1);
  });

  it("does not negotiate disabled Token access", async () => {
    mocks.status.enabled = false;
    await start();
    expect(mocks.negotiate).not.toHaveBeenCalled();
    expect(mocks.handoff).not.toHaveBeenCalled();
  });

  it("stops supplementary migration when a health request fails", async () => {
    await start();
    mocks.handoff.mockClear();
    session.sendPing.mockRejectedValueOnce(new Error("unhealthy"));
    await rounds();
    expect(mocks.state.mode).toBe("ws");
    expect(session.close).toHaveBeenCalledWith(true);
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
      targetPath: "wss",
      allowReconnect: true,
    });
    expect(session.close.mock.invocationCallOrder[0]).toBeLessThan(
      mocks.handoff.mock.invocationCallOrder[0]!,
    );
    await vi.advanceTimersByTimeAsync(HYBRID_SAMPLE_INTERVAL_MS - 1);
    expect(mocks.handoff).toHaveBeenCalledTimes(1);
  });

  it.each(["unknown", "webrtc-turn"] as const)(
    "falls back rather than migrating when selected path becomes %s",
    async (path) => {
      await start();
      mocks.handoff.mockClear();
      mocks.inspect.mockResolvedValue({
        path,
        candidateType: "relay",
        ipFamily: "ipv4",
      });
      await rounds();
      expect(mocks.state.mode).toBe("ws");
      expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
        targetPath: "wss",
        allowReconnect: true,
      });
      expect(session.close).toHaveBeenCalledWith(true);
    },
  );

  it("rejects a changed negotiated path even if the selected pair matches it", async () => {
    await start();
    mocks.handoff.mockClear();
    session.path = "webrtc-turn";
    mocks.inspect.mockResolvedValue({
      path: "webrtc-turn",
      candidateType: "relay",
      ipFamily: "ipv4",
    });
    await rounds();
    expect(mocks.state.mode).toBe("ws");
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
      targetPath: "wss",
      allowReconnect: true,
    });
  });

  it("does not supplement an expired authorized session", async () => {
    await start(true, "webrtc-turn");
    mocks.handoff.mockClear();
    session.expiresAt = Math.floor(Date.now() / 1_000);
    await rounds();
    expect(mocks.state.mode).toBe("ws");
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
      targetPath: "wss",
      allowReconnect: true,
    });
  });

  it("drains active peer streams gracefully on standby_hysteresis before closing the peer session", async () => {
    await start();
    mocks.streamListener?.([
      {
        id: 1,
        kind: "terminal",
        path: "webrtc-direct",
        state: "connected",
        inputUncertain: false,
      },
    ]);
    mocks.handoff.mockClear();
    mocks.setTransport.mockClear();

    // Degrade RTT across the 30s hysteresis cooldown window (6 * 5s rounds).
    mocks.baseline.mockResolvedValue([80]);
    session.sendPing.mockResolvedValue(120);
    await rounds(6);

    expect(mocks.state.mode).toBe("ws");
    expect(mocks.state.phase).toBe("standby_hysteresis");
    expect(mocks.setTransport).toHaveBeenCalledExactlyOnceWith(null);
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
      targetPath: "wss",
      allowReconnect: true,
    });
    // Peer session stays alive while stream 1 is still on `webrtc-direct`.
    expect(session.close).not.toHaveBeenCalled();

    // Once stream 1 finishes migrating to `wss`, the draining session closes immediately.
    mocks.streamListener?.([
      {
        id: 1,
        kind: "terminal",
        path: "wss",
        state: "connected",
        inputUncertain: false,
      },
    ]);
    expect(session.close).toHaveBeenCalledExactlyOnceWith(true);
  });

  it("drains gracefully when settings disable P2P on a healthy session and enforces the 10s drain ceiling", async () => {
    await start();
    mocks.streamListener?.([
      {
        id: 2,
        kind: "terminal",
        path: "webrtc-direct",
        state: "connected",
        inputUncertain: false,
      },
    ]);
    mocks.handoff.mockClear();

    refreshP2PStatus({
      supported: true,
      udpPort: 0,
      udpBindings: [],
      capabilities: { networks: [], iceMode: "full", turn: false },
      enabled: false,
      portMapping: false,
      stunServers: [],
      source: "web",
      storageAvailable: true,
      portMap: { state: "off" },
      activePeers: 1,
      peers: [],
    } satisfies P2PStatus);

    expect(mocks.state.mode).toBe("ws");
    expect(mocks.state.phase).toBe("disabled");
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
      targetPath: "wss",
      allowReconnect: true,
    });
    expect(session.close).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(9_999);
    expect(session.close).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(session.close).toHaveBeenCalledExactlyOnceWith(true);
  });

  it("re-hands off live peer streams when a network change interrupts an active graceful drain", async () => {
    const netConnection = new EventTarget();
    vi.stubGlobal("navigator", { connection: netConnection });
    await start();
    mocks.streamListener?.([
      {
        id: 1,
        kind: "terminal",
        path: "webrtc-direct",
        state: "connected",
        inputUncertain: false,
      },
      {
        id: 2,
        kind: "other",
        path: "webrtc-direct",
        state: "connected",
        inputUncertain: false,
      },
    ]);
    mocks.handoff.mockClear();

    // Enter graceful drain via standby_hysteresis; p2pState.mode is now "ws".
    mocks.baseline.mockResolvedValue([80]);
    session.sendPing.mockResolvedValue(120);
    await rounds(6);
    expect(mocks.state.mode).toBe("ws");
    expect(session.close).not.toHaveBeenCalled();
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
      targetPath: "wss",
      allowReconnect: true,
    });
    mocks.handoff.mockClear();

    // Ungraceful network change arrives mid-drain while p2pState.mode is already "ws".
    netConnection.dispatchEvent(new Event("change"));

    expect(session.close).toHaveBeenCalledExactlyOnceWith(true);
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith({
      targetPath: "wss",
      allowReconnect: true,
    });
  });

  it.each([false, true])(
    "ignores a pending healthy round after authorization revocation (owner=%s)",
    async (owner) => {
      await start(owner);
      mocks.handoff.mockClear();
      const pending = Promise.withResolvers<number>();
      session.sendPing.mockReturnValueOnce(pending.promise);
      await rounds();
      window.dispatchEvent(
        new Event(owner ? "webmux:relay-revoked" : "webmux:auth"),
      );
      expect(session.close).toHaveBeenCalledWith(true);
      pending.resolve(899);
      await vi.advanceTimersByTimeAsync(HYBRID_SAMPLE_INTERVAL_MS * 2);
      expect(mocks.state.reason).toBe("authorization_revoked");
      expect(mocks.handoff).not.toHaveBeenCalled();
      expect(mocks.negotiate).toHaveBeenCalledTimes(1);
      expect(mocks.createTransport).toHaveBeenCalledTimes(1);
    },
  );
});
