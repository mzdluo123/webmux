import {
  P2PControlMessageSchema,
  type P2PICECandidate,
} from "../contracts/schemas";
import { relayOwnerAuthenticated, relayAuthRevision } from "./relay-auth";
import {
  requestOwnerICE,
  sendSignalHttp,
  nextP2PPingSequence,
  PING_TIMEOUT_MS,
  NEGOTIATION_TIMEOUT_MS,
} from "./p2p-signaling";

const P2P_PROTO_VERSION = 1;
const CONTROL_LABEL = "webmux:control";
const CANDIDATE_INTERVAL_MS = 1_000;

export interface NegotiatedSession {
  peerId: string;
  pc: RTCPeerConnection;
  control: RTCDataChannel;
  candidateType: string;
  ipFamily: string;
  path: "webrtc-direct" | "webrtc-turn";
  expiresAt: number | null;
  onFailure: (() => void) | null;
  sendPing: () => Promise<number>;
  close: (notifyServer?: boolean) => void;
}

function delay(ms: number, signal: AbortSignal): Promise<void> {
  signal.throwIfAborted();
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", abort);
      resolve();
    }, ms);
    const abort = () => {
      clearTimeout(timer);
      reject(signal.reason);
    };
    signal.addEventListener("abort", abort, { once: true });
  });
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export async function inspectSelectedCandidatePair(
  pc: RTCPeerConnection,
): Promise<{
  isRelay: boolean;
  path: "unknown" | "webrtc-direct" | "webrtc-turn";
  candidateType: string;
  ipFamily: string;
}> {
  if (typeof pc.getStats !== "function") {
    return {
      isRelay: false,
      path: "unknown",
      candidateType: "unknown",
      ipFamily: "",
    };
  }
  const { promise, reject } = Promise.withResolvers<never>();
  const timer = setTimeout(
    () => reject(new Error("ICE statistics timeout")),
    PING_TIMEOUT_MS,
  );
  let stats: RTCStatsReport;
  try {
    stats = await Promise.race([pc.getStats(), promise]);
  } finally {
    clearTimeout(timer);
  }
  let selectedLocalId = "";
  let selectedRemoteId = "";

  stats.forEach((report: Record<string, unknown>) => {
    if (
      report.type === "transport" &&
      typeof report.selectedCandidatePairId === "string"
    ) {
      const pair = stats.get(report.selectedCandidatePairId) as
        | Record<string, unknown>
        | undefined;
      if (pair) {
        selectedLocalId = asString(pair.localCandidateId);
        selectedRemoteId = asString(pair.remoteCandidateId);
      }
    }
  });

  if (!selectedLocalId) {
    let nominated = 0;
    stats.forEach((report: Record<string, unknown>) => {
      if (
        report.type === "candidate-pair" &&
        (report.selected === true || report.nominated === true) &&
        report.state === "succeeded"
      ) {
        selectedLocalId = asString(report.localCandidateId);
        nominated++;
        selectedRemoteId = asString(report.remoteCandidateId);
      }
    });
    if (nominated !== 1) {
      selectedLocalId = "";
      selectedRemoteId = "";
    }
  }

  const localCand = selectedLocalId
    ? (stats.get(selectedLocalId) as Record<string, unknown> | undefined)
    : undefined;
  const remoteCand = selectedRemoteId
    ? (stats.get(selectedRemoteId) as Record<string, unknown> | undefined)
    : undefined;

  const localType = asString(localCand?.candidateType);
  const remoteType = asString(remoteCand?.candidateType);
  const isRelay = localType === "relay" || remoteType === "relay";
  const addr =
    asString(remoteCand?.address) ||
    asString(remoteCand?.ip) ||
    asString(localCand?.address);
  const ipFamily = addr.includes(":") ? "ipv6" : addr ? "ipv4" : "";
  const candidateType =
    localType && remoteType
      ? `${localType}/${remoteType}`
      : localType || remoteType || "unknown";

  const knownTypes: Record<string, true> = {
    host: true,
    srflx: true,
    prflx: true,
    relay: true,
  };
  const path = isRelay
    ? "webrtc-turn"
    : knownTypes[localType] && knownTypes[remoteType]
      ? "webrtc-direct"
      : "unknown";
  return { isRelay, path, candidateType, ipFamily };
}

export async function negotiateDirectPeer(
  stunServers: string[],
  signal?: AbortSignal,
): Promise<NegotiatedSession> {
  signal?.throwIfAborted();
  if (typeof RTCPeerConnection === "undefined") {
    throw new Error("RTCPeerConnection unavailable");
  }
  const owner = relayOwnerAuthenticated();
  const authorizationRevision = relayAuthRevision();
  const peerConnectionId = owner
    ? btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(32))))
        .replaceAll("+", "-")
        .replaceAll("/", "_")
        .replace(/=+$/, "")
    : undefined;
  let iceServers: RTCIceServer[] =
    stunServers.length > 0 ? [{ urls: stunServers }] : [];
  let expiresAt: number | null = null;
  if (peerConnectionId !== undefined) {
    const endpoint = await requestOwnerICE(
      peerConnectionId,
      authorizationRevision,
      signal,
    );
    iceServers = endpoint.servers;
    expiresAt = endpoint.expiresAt;
  }
  const pc = new RTCPeerConnection({ iceServers });
  let control: RTCDataChannel;
  try {
    control = pc.createDataChannel(CONTROL_LABEL, { ordered: true });
  } catch (err) {
    pc.close();
    throw err;
  }
  const localCandidates: P2PICECandidate[] = [];
  const work = new AbortController();
  let peerId = "";
  let closed = false;
  let negotiating = true;
  let negotiatedSession: NegotiatedSession | null = null;
  let expiryTimer: ReturnType<typeof setTimeout> | undefined;
  let resolveHello!: (meta: {
    candidateType: string;
    ipFamily: string;
  }) => void;
  let rejectHello!: (err: unknown) => void;
  const helloDone = new Promise<{ candidateType: string; ipFamily: string }>(
    (resolve, reject) => {
      resolveHello = resolve;
      rejectHello = reject;
    },
  );
  // A control error can arrive while offer/answer HTTP is still pending.
  void helloDone.catch(() => undefined);
  let rejectFailure!: (err: unknown) => void;
  const failure = new Promise<never>((_, reject) => {
    rejectFailure = reject;
  });
  const timer = setTimeout(
    () => fail(new Error("P2P handshake timeout")),
    NEGOTIATION_TIMEOUT_MS,
  );
  const abort = () => fail(signal?.reason);
  const stopNegotiating = () => {
    negotiating = false;
    clearTimeout(timer);
    signal?.removeEventListener("abort", abort);
    work.abort();
    pc.onicecandidate = null;
    control.onopen = null;
    localCandidates.length = 0;
  };
  const pendingPongs = new Map<
    number,
    {
      resolve: (rttMs: number) => void;
      reject: (err: Error) => void;
      sentAt: number;
      timer: ReturnType<typeof setTimeout>;
    }
  >();
  const notifyClose = () => {
    if (
      peerId &&
      (!owner ||
        (relayOwnerAuthenticated() &&
          relayAuthRevision() === authorizationRevision))
    ) {
      void sendSignalHttp({ action: "close", peerId, peerConnectionId }).catch(
        () => undefined,
      );
    }
  };
  const closeSession = (notifyServer = true) => {
    if (closed) return;
    closed = true;
    stopNegotiating();
    clearTimeout(expiryTimer);
    if (owner && typeof window !== "undefined")
      window.removeEventListener("webmux:relay-revoked", revokeOwner);
    // Drop the short-lived TURN material before closing the allocation.
    try {
      pc.setConfiguration({ iceServers: [] });
    } catch {
      // A failed/closed peer can reject configuration updates.
    }
    for (const pending of pendingPongs.values()) {
      clearTimeout(pending.timer);
      pending.reject(new Error("Control channel closed"));
    }
    pendingPongs.clear();
    control.onmessage = null;
    control.onclose = null;
    control.onerror = null;
    pc.onconnectionstatechange = null;
    try {
      control.close();
    } catch {
      // Already closed.
    }
    try {
      pc.close();
    } catch {
      // Already closed.
    }
    if (notifyServer) notifyClose();
    negotiatedSession?.onFailure?.();
  };
  function fail(err: unknown) {
    if (!negotiating) return;
    rejectHello(err);
    rejectFailure(err);
    closeSession();
  }
  function revokeOwner() {
    if (negotiating) fail(new Error("P2P authorization revoked"));
    else closeSession(false);
  }
  const ensureNegotiating = () => {
    work.signal.throwIfAborted();
    if (
      owner &&
      (!relayOwnerAuthenticated() ||
        relayAuthRevision() !== authorizationRevision)
    ) {
      throw new Error("P2P authorization revoked");
    }
  };
  const sendPing = (): Promise<number> => {
    if (closed || control.readyState !== "open") {
      return Promise.reject(new Error("Control channel not open"));
    }
    const seq = nextP2PPingSequence();
    const sentAt = performance.now();
    return new Promise<number>((resolve, reject) => {
      const timer = setTimeout(() => {
        pendingPongs.delete(seq);
        reject(new Error("P2P ping timeout"));
      }, PING_TIMEOUT_MS);
      pendingPongs.set(seq, { resolve, reject, sentAt, timer });
      try {
        control.send(
          JSON.stringify({
            type: "ping",
            seq,
            clientTs: Math.round(Date.now()),
          }),
        );
      } catch (err) {
        clearTimeout(timer);
        pendingPongs.delete(seq);
        reject(err);
      }
    });
  };
  pc.onicecandidate = (ev) => {
    if (!ev.candidate?.candidate || !negotiating) return;
    localCandidates.push({
      candidate: ev.candidate.candidate,
      sdpMid: ev.candidate.sdpMid,
      sdpMLineIndex: ev.candidate.sdpMLineIndex,
    });
  };
  pc.onconnectionstatechange = () => {
    if (pc.connectionState === "failed" || pc.connectionState === "closed") {
      if (negotiating) fail(new Error("P2P connection closed"));
      else closeSession();
    }
  };
  control.onclose = () => {
    if (negotiating) fail(new Error("P2P control channel closed"));
    else closeSession();
  };
  control.onerror = () => {
    if (negotiating) fail(new Error("P2P control channel error"));
    else closeSession();
  };
  control.onopen = () => {
    try {
      control.send(
        JSON.stringify({
          type: "hello",
          proto: P2P_PROTO_VERSION,
          appVersion: "1",
          features: ["stream-v1", "rpc-v1"],
        }),
      );
    } catch (err) {
      fail(err);
    }
  };
  control.onmessage = (ev) => {
    if (typeof ev.data !== "string") return;
    let parsed;
    try {
      parsed = P2PControlMessageSchema.parse(JSON.parse(ev.data));
    } catch {
      return;
    }
    if (parsed.type === "hello" && negotiating) {
      if (parsed.proto !== P2P_PROTO_VERSION) {
        fail(new Error(`Incompatible P2P proto ${parsed.proto}`));
        return;
      }
      resolveHello({
        candidateType: parsed.candidateType ?? "",
        ipFamily: parsed.ipFamily ?? "",
      });
    } else if (parsed.type === "pong") {
      const pending = pendingPongs.get(parsed.seq);
      if (pending) {
        clearTimeout(pending.timer);
        pendingPongs.delete(parsed.seq);
        pending.resolve(
          Math.max(0, Math.round(performance.now() - pending.sentAt)),
        );
      }
    }
  };
  signal?.addEventListener("abort", abort, { once: true });
  if (owner && typeof window !== "undefined")
    window.addEventListener("webmux:relay-revoked", revokeOwner);
  const applyCandidates = async (candidates: P2PICECandidate[]) => {
    for (const candidate of candidates) {
      ensureNegotiating();
      if (candidate.candidate) {
        await pc.addIceCandidate({
          candidate: candidate.candidate,
          sdpMid: candidate.sdpMid ?? null,
          sdpMLineIndex: candidate.sdpMLineIndex ?? null,
        });
      }
    }
    ensureNegotiating();
  };
  const exchangeCandidates = async () => {
    while (negotiating) {
      await delay(CANDIDATE_INTERVAL_MS, work.signal);
      const batch = localCandidates.splice(0);
      const response = await sendSignalHttp(
        {
          action: "candidate",
          peerId,
          peerConnectionId,
          ...(batch.length > 0 ? { candidates: batch } : {}),
        },
        work.signal,
      );
      await applyCandidates(response.candidates);
    }
  };
  const negotiate = async (): Promise<NegotiatedSession> => {
    ensureNegotiating();
    const offer = await pc.createOffer();
    ensureNegotiating();
    await pc.setLocalDescription(offer);
    ensureNegotiating();
    const response = await sendSignalHttp(
      { action: "offer", sdp: offer.sdp ?? "", peerConnectionId },
      work.signal,
    );
    peerId = response.peerId;
    if (!negotiating) {
      // A transport may finish an offer despite aborting its request.
      notifyClose();
      ensureNegotiating();
    }
    await pc.setRemoteDescription({
      type: "answer",
      sdp: response.sdp ?? "",
    });
    await applyCandidates(response.candidates);
    // One serial pump owns both upload batches and every response's candidates.
    // Empty uploads keep fetching server candidates even after local gathering ends.
    void exchangeCandidates().catch((err: unknown) => {
      if (negotiating) fail(err);
    });
    await helloDone;
    ensureNegotiating();
    const statsMeta = await inspectSelectedCandidatePair(pc);
    ensureNegotiating();
    if (statsMeta.path === "unknown") {
      throw new Error("Selected ICE pair unavailable");
    }
    if (statsMeta.isRelay && !owner) {
      const err = new Error("Relay candidate rejected");
      err.name = "RelayRejectedError";
      throw err;
    }
    stopNegotiating();
    if (expiresAt !== null) {
      const remaining = expiresAt * 1000 - Date.now();
      if (remaining <= 0) throw new Error("ICE authorization expired");
      expiryTimer = setTimeout(() => closeSession(), remaining);
    }
    negotiatedSession = {
      peerId,
      pc,
      control,
      candidateType: statsMeta.candidateType,
      ipFamily: statsMeta.ipFamily,
      path: statsMeta.path,
      expiresAt,
      onFailure: null,
      sendPing,
      close: closeSession,
    };
    return negotiatedSession;
  };
  try {
    if (signal?.aborted) abort();
    return await Promise.race([negotiate(), failure]);
  } catch (err) {
    closeSession();
    throw err;
  }
}
