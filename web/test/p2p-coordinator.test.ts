import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vite-plus/test";
import type { NegotiatedSession } from "../src/platform/p2p-session";
import { startP2P, stopP2P } from "../src/platform/p2p.svelte";
import { HYBRID_SAMPLE_INTERVAL_MS } from "../src/platform/p2p-framing";

const mocks = vi.hoisted(() => ({
  owner: false,
  state: {
    mode: "ws" as "ws" | "p2p",
    phase: "idle",
    reason: "",
    candidatePath: "unknown",
    switchCount: 0,
    streams: [],
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
  observeManagedStreams: () => () => {},
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
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith();
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
      expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith();
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
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith();
  });

  it("does not supplement an expired authorized session", async () => {
    await start(true, "webrtc-turn");
    mocks.handoff.mockClear();
    session.expiresAt = Math.floor(Date.now() / 1_000);
    await rounds();
    expect(mocks.state.mode).toBe("ws");
    expect(mocks.handoff).toHaveBeenCalledExactlyOnceWith();
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
