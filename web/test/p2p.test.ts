import { describe, expect, it, vi } from "vite-plus/test";
import {
  StreamFrameReassembler,
  encodeStreamCloseFrame,
  encodeStreamFrames,
  isLocalOrOverlayHost,
  medianSample,
  shouldUpgradeToP2P,
  HybridPathPolicy,
  HYBRID_COOLDOWN_MS,
  HYBRID_STABLE_SAMPLES,
} from "../src/platform/p2p-framing";
import { inspectSelectedCandidatePair } from "../src/platform/p2p-session";
import { createP2PTransport } from "../src/platform/p2p-transport";
import {
  interactiveStreamKind,
  type ClientTransport,
} from "../src/platform/transport";

class MockDataChannel extends EventTarget {
  readyState: RTCDataChannelState = "connecting";
  binaryType: BinaryType = "arraybuffer";
  bufferedAmount = 0;
  bufferedAmountLowThreshold = 0;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string | ArrayBuffer }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  readonly sent: (string | Uint8Array)[] = [];

  constructor(readonly label: string) {
    super();
  }

  open() {
    this.readyState = "open";
    this.onopen?.();
  }

  send(data: string | Uint8Array | ArrayBuffer) {
    if (typeof data === "string") {
      this.sent.push(data);
    } else if (data instanceof Uint8Array) {
      const copy = new Uint8Array(data.byteLength);
      copy.set(data);
      this.sent.push(copy);
    } else {
      this.sent.push(new Uint8Array(data.slice(0)));
    }
  }

  receive(data: string | ArrayBuffer | Uint8Array) {
    if (typeof data === "string") {
      this.onmessage?.({ data });
    } else if (data instanceof Uint8Array) {
      const copy = new Uint8Array(data.byteLength);
      copy.set(data);
      this.onmessage?.({ data: copy.buffer });
    } else {
      this.onmessage?.({ data });
    }
  }

  close() {
    if (this.readyState === "closed") return;
    this.readyState = "closed";
    this.onclose?.();
  }
}

class MockPeerConnection {
  connectionState: RTCPeerConnectionState = "connected";
  readonly channels: MockDataChannel[] = [];

  createDataChannel(label: string) {
    const ch = new MockDataChannel(label);
    this.channels.push(ch);
    return ch as unknown as RTCDataChannel;
  }
}

describe("P2P host locality and hysteresis predicates", () => {
  it("identifies loopback, RFC1918 LAN, IPv6 ULA/link-local, and Tailscale hosts", () => {
    expect(isLocalOrOverlayHost("localhost")).toBe(true);
    expect(isLocalOrOverlayHost("127.0.0.1")).toBe(true);
    expect(isLocalOrOverlayHost("[::1]")).toBe(true);
    expect(isLocalOrOverlayHost("192.168.1.50")).toBe(true);
    expect(isLocalOrOverlayHost("10.0.4.12")).toBe(true);
    expect(isLocalOrOverlayHost("172.20.0.1")).toBe(true);
    expect(isLocalOrOverlayHost("100.101.102.103")).toBe(true);
    expect(isLocalOrOverlayHost("devbox.tail1234.ts.net")).toBe(true);
    expect(isLocalOrOverlayHost("fd7a:115c:a1e0::1")).toBe(true);

    expect(isLocalOrOverlayHost("term.example.com")).toBe(false);
    expect(isLocalOrOverlayHost("203.0.113.10")).toBe(false);
    expect(isLocalOrOverlayHost("172.32.0.1")).toBe(false);
    expect(isLocalOrOverlayHost("100.63.255.1")).toBe(false);
  });

  it("applies strict 15ms floor and 10ms / 15% hysteresis before upgrading", () => {
    expect(shouldUpgradeToP2P(12, 1)).toBe(false);
    expect(shouldUpgradeToP2P(80, 72)).toBe(false);
    expect(shouldUpgradeToP2P(80, 65)).toBe(true);
    expect(shouldUpgradeToP2P(40, 28)).toBe(true);
    expect(medianSample([90, 20, 50])).toBe(50);
  });
});

describe("P2P stream framing & close codes", () => {
  it("fragments and reassembles >64 KiB binary payloads and decodes 4410 close frame", () => {
    const large = new Uint8Array(100 * 1024);
    for (let i = 0; i < large.length; i++) large[i] = (i * 13) & 0xff;

    const frames = encodeStreamFrames(large);
    expect(frames.length).toBe(4); // 32 + 32 + 32 + 4 KiB

    const reassembler = new StreamFrameReassembler();
    expect(reassembler.feed(frames[0]!)).toBeNull();
    expect(reassembler.feed(frames[1]!)).toBeNull();
    expect(reassembler.feed(frames[2]!)).toBeNull();
    const done = reassembler.feed(frames[3]!);
    expect(done?.kind).toBe("message");
    if (done?.kind === "message") {
      expect(new Uint8Array(done.data as ArrayBuffer)).toEqual(large);
    }

    const closeFrame = encodeStreamCloseFrame(4410, "session exited");
    const closed = reassembler.feed(closeFrame);
    expect(closed).toEqual({
      kind: "close",
      code: 4410,
      reason: "session exited",
    });
  });

  it("classifies actual TURN selection without treating it as direct", async () => {
    const fakeStats = new Map<string, Record<string, unknown>>([
      ["t1", { type: "transport", selectedCandidatePairId: "pair1" }],
      [
        "pair1",
        {
          type: "candidate-pair",
          localCandidateId: "loc1",
          remoteCandidateId: "rem1",
          selected: true,
          state: "succeeded",
        },
      ],
      ["loc1", { candidateType: "srflx", address: "198.51.100.5" }],
      ["rem1", { candidateType: "relay", address: "203.0.113.9" }],
    ]);
    const pc = {
      getStats: async () => fakeStats,
    } as unknown as RTCPeerConnection;

    const res = await inspectSelectedCandidatePair(pc);
    expect(res.isRelay).toBe(true);
    expect(res.candidateType).toBe("srflx/relay");
    expect(res.ipFamily).toBe("ipv4");
  });
});

describe("P2P interactive streams and baseline isolation", () => {
  it("strips compress=1 and adds zreplay=1 on terminal streams", () => {
    const mockPC = new MockPeerConnection();
    const fallback: ClientTransport = {
      origin: "https://term.example.com",
      fetch: vi.fn(),
      upload: vi.fn(),
      openStream: vi.fn(),
    };
    const transport = createP2PTransport({
      pc: mockPC as unknown as RTCPeerConnection,
      path: "webrtc-direct",
      interactiveOnly: true,
      fallback,
    });

    const sock = transport.openStream("/ws/s/abc?client=c-1&compress=1", {
      token: "secret",
      compress: true,
    });
    expect(mockPC.channels).toHaveLength(1);
    expect(mockPC.channels[0]!.label).toBe(
      "stream:/ws/s/abc?client=c-1&zreplay=1",
    );

    const opened = vi.fn();
    const received: (string | ArrayBuffer)[] = [];
    const closed = vi.fn();
    sock.onopen = opened;
    sock.onmessage = (ev) => received.push(ev.data);
    sock.onclose = closed;

    const dc = mockPC.channels[0]!;
    dc.open();
    expect(opened).toHaveBeenCalledOnce();

    for (const chunk of encodeStreamFrames("hello p2p")) {
      dc.receive(chunk);
    }
    expect(received).toEqual(["hello p2p"]);

    dc.receive(encodeStreamCloseFrame(4404, "not found"));
    expect(closed).toHaveBeenCalledWith({ code: 4404, reason: "not found" });
  });

  it("keeps every REST request, uploads and unclassified stream on baseline without replay", async () => {
    const pc = new MockPeerConnection();
    const failed = new Error("unknown mutation result");
    const fallback: ClientTransport = {
      origin: "https://term.example.com",
      fetch: vi.fn().mockRejectedValue(failed),
      upload: vi.fn().mockRejectedValue(failed),
      openStream: vi.fn(),
    };
    const transport = createP2PTransport({
      pc: pc as unknown as RTCPeerConnection,
      fallback,
      path: "webrtc-turn",
      interactiveOnly: true,
    });
    for (const path of [
      "/sessions",
      "/sessions/s1/input",
      "/files/download",
      "/relay/ice",
      "/p2p/ping",
      "/settings",
      "/unknown",
    ]) {
      await expect(
        transport.fetch(path, { method: "POST", body: "input" }),
      ).rejects.toBe(failed);
    }
    const request = {
      path: "/uploads",
      form: new FormData(),
      token: "",
      csrfToken: "csrf",
      timeoutMs: 5000,
      signal: new AbortController().signal,
      onProgress: vi.fn(),
    };
    await expect(transport.upload(request)).rejects.toBe(failed);
    expect(fallback.upload).toHaveBeenCalledExactlyOnceWith(request);
    expect(fallback.fetch).toHaveBeenCalledTimes(7);
    for (const path of [
      "/ws/voice",
      "/forward/3000/ws",
      "/ws/s/a/voice",
      "/ws/s/a/download",
      "/ws/settings",
      "/ws/s/a%2Fchat",
    ]) {
      transport.openStream(path, { token: "", compress: true });
    }
    expect(fallback.openStream).toHaveBeenCalledTimes(6);
    expect(pc.channels).toHaveLength(0);
  });

  it("allows only explicit terminal, chat and list streams", () => {
    expect(interactiveStreamKind("/ws/sessions?observe=1")).toBe("list");
    expect(interactiveStreamKind("/ws/s/abc")).toBe("terminal");
    expect(interactiveStreamKind("/ws/s/abc/chat?seq=3")).toBe("chat");
    expect(interactiveStreamKind("/ws/s/abc/chat/extra")).toBeNull();
    const pc = new MockPeerConnection();
    const transport = createP2PTransport({
      pc: pc as unknown as RTCPeerConnection,
      path: "webrtc-turn",
      interactiveOnly: true,
      fallback: {
        origin: "",
        fetch: vi.fn(),
        upload: vi.fn(),
        openStream: vi.fn(),
      },
    });
    const stream = transport.openStream("/ws/sessions", {
      token: "",
      compress: false,
    });
    expect(stream.transportPath).toBe("webrtc-turn");
    pc.channels[0]!.open();
    pc.channels[0]!.bufferedAmount = 1024 * 1024;
    expect(() => stream.send(new Uint8Array([0, 65]))).toThrow("backpressure");
    expect(pc.channels[0]!.sent).toHaveLength(0);
  });

  it("never guesses a direct path from missing, incomplete or ambiguous stats", async () => {
    for (const stats of [
      new Map(),
      new Map([["local", { candidateType: "host" }]]),
      new Map([
        [
          "one",
          {
            type: "candidate-pair",
            nominated: true,
            state: "succeeded",
            localCandidateId: "a",
            remoteCandidateId: "b",
          },
        ],
        [
          "two",
          {
            type: "candidate-pair",
            nominated: true,
            state: "succeeded",
            localCandidateId: "a",
            remoteCandidateId: "b",
          },
        ],
        ["a", { candidateType: "host" }],
        ["b", { candidateType: "srflx" }],
      ]),
    ]) {
      const pc = {
        getStats: async () => stats,
      } as unknown as RTCPeerConnection;
      expect((await inspectSelectedCandidatePair(pc)).path).toBe("unknown");
    }
    expect(
      (await inspectSelectedCandidatePair({} as RTCPeerConnection)).path,
    ).toBe("unknown");
  });

  it("requires consecutive advantage, applies hysteresis and cooldown without hiding failure", () => {
    const policy = new HybridPathPolicy();
    expect(policy.sample(100, 20, false, 0)).toBe("hold");
    expect(policy.sample(100, 95, false, 1)).toBe("hold");
    expect(policy.sample(100, 20, false, 2)).toBe("hold");
    expect(policy.sample(100, 20, false, 3)).toBe("hold");
    expect(policy.sample(100, 20, false, 4)).toBe("upgrade");
    for (let i = 0; i < HYBRID_STABLE_SAMPLES; i++) {
      expect(policy.sample(100, 120, true, 5 + i)).toBe("hold");
    }
    expect(policy.sample(100, 120, true, HYBRID_COOLDOWN_MS + 4)).toBe(
      "fallback",
    );
    expect(policy.sample(100, 1, false, HYBRID_COOLDOWN_MS + 5)).toBe("hold");
    expect(policy.sample(100, 1, false, HYBRID_COOLDOWN_MS + 6)).toBe("hold");
    expect(policy.sample(100, 1, false, HYBRID_COOLDOWN_MS + 7)).toBe("hold");
  });
});

describe("existing Token/local P2P RPC consumers", () => {
  it("keeps JSON mutations and multipart uploads on the legacy RPC protocol", async () => {
    const pc = new MockPeerConnection();
    const baseline = {
      origin: "https://term.example.com",
      fetch: vi.fn(),
      upload: vi.fn(),
      openStream: vi.fn(),
    };
    const transport = createP2PTransport({
      pc: pc as unknown as RTCPeerConnection,
      fallback: baseline,
      path: "webrtc-direct",
      interactiveOnly: false,
    });
    const pending = transport.fetch("/sessions", {
      method: "POST",
      headers: {
        Authorization: "Bearer tok",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ name: "dev" }),
    });
    await vi.waitFor(() => expect(pc.channels).toHaveLength(1));
    const rpc = pc.channels[0]!;
    expect(rpc.label).toBe("rpc:1");
    rpc.open();
    await vi.waitFor(() => expect(rpc.sent.length).toBeGreaterThanOrEqual(3));
    expect(JSON.parse(rpc.sent[0] as string)).toMatchObject({
      method: "POST",
      path: "/api/sessions",
      headers: { authorization: "Bearer tok" },
    });
    rpc.receive(
      JSON.stringify({
        status: 201,
        statusText: "Created",
        headers: { "content-type": "application/json" },
      }),
    );
    rpc.receive(new TextEncoder().encode('{"id":"s1"}'));
    rpc.receive(new Uint8Array(0));
    const response = await pending;
    expect(response.status).toBe(201);
    expect(await response.json()).toEqual({ id: "s1" });

    const form = new FormData();
    form.append(
      "file",
      new Blob(["hello upload"], { type: "text/plain" }),
      "note.txt",
    );
    const progress = vi.fn();
    const upload = transport.upload({
      path: "/uploads",
      form,
      token: "tok",
      timeoutMs: 5000,
      signal: new AbortController().signal,
      onProgress: progress,
    });
    await vi.waitFor(() => expect(pc.channels).toHaveLength(2));
    const bulk = pc.channels[1]!;
    bulk.open();
    await vi.waitFor(() => expect(bulk.sent.length).toBeGreaterThanOrEqual(3));
    const header = JSON.parse(bulk.sent[0] as string);
    expect(header.path).toBe("/api/uploads");
    expect(header.headers["Content-Type"]).toContain(
      "multipart/form-data; boundary=",
    );
    bulk.receive(
      JSON.stringify({
        status: 200,
        statusText: "OK",
        headers: { "content-type": "application/json" },
      }),
    );
    bulk.receive(new TextEncoder().encode('{"path":"/tmp/note.txt"}'));
    bulk.receive(new Uint8Array(0));
    expect(await upload).toMatchObject({
      status: 200,
      body: { path: "/tmp/note.txt" },
    });
    expect(progress).toHaveBeenCalledWith(100);
    expect(baseline.fetch).not.toHaveBeenCalled();
    expect(baseline.upload).not.toHaveBeenCalled();
  });
});
