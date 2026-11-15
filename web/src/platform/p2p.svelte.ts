import { m } from "$lib/paraglide/messages.js";
import { api } from "./api";
import {
  isLocalOrOverlayHost,
  hasStrongP2PAdvantage,
  medianSample,
  HybridPathPolicy,
  HYBRID_SAMPLE_INTERVAL_MS,
  HYBRID_STABLE_SAMPLES,
  HYBRID_INITIAL_SAMPLE_INTERVAL_MS,
} from "./p2p-framing";
import {
  inspectSelectedCandidatePair,
  negotiateDirectPeer,
  type NegotiatedSession,
} from "./p2p-session";
import { measureBaselineHttpRtt } from "./p2p-signaling";
import { p2pState, type P2PPhase } from "./p2p-state.svelte";
import { createP2PTransport } from "./p2p-transport";
import { P2PStatusSchema, type P2PStatus } from "../contracts/schemas";
import { onActiveHostChange } from "./storage";
import { relayOwnerAuthenticated } from "./relay-auth";
import { defaultBrowserTransport, setClientTransport } from "./transport";
import { handoffLiveSockets, observeManagedStreams } from "./ws";

const BACKOFF_STEPS_MS = [5_000, 15_000, 60_000];

let activeEpoch = 0;
let activeSession: NegotiatedSession | null = null;
let keepaliveTimer: ReturnType<typeof setInterval> | undefined;
let retryTimer: ReturnType<typeof setTimeout> | undefined;
let retryIndex = 0;
let forceMode = false;
let started = false;
let probeController: AbortController | null = null;
let healthController: AbortController | null = null;
let abTestController: AbortController | null = null;
let networkConnection: EventTarget | undefined;
let disconnectHost: (() => void) | undefined;
let disconnectStreams: (() => void) | undefined;
let ownerMode = false;
let pathPolicy = new HybridPathPolicy();

function hasForceFlag(): boolean {
  if (
    typeof location !== "undefined" &&
    new URLSearchParams(location.search).get("p2p") === "force"
  ) {
    forceMode = true;
  }
  return forceMode;
}

function teardownActiveSession(notifyServer: boolean) {
  clearInterval(keepaliveTimer);
  keepaliveTimer = undefined;
  const current = activeSession;
  activeSession = null;
  healthController?.abort();
  healthController = null;
  current?.close(notifyServer);
}

function fallbackToBaseline(nextPhase: P2PPhase, scheduleRetry: boolean) {
  const wasP2P = p2pState.mode === "p2p";
  setClientTransport(null);
  p2pState.mode = "ws";
  p2pState.phase = nextPhase;
  p2pState.reason = nextPhase;
  if (wasP2P) {
    if (ownerMode) pathPolicy.reset(performance.now());
    else pathPolicy = new HybridPathPolicy();
    p2pState.switchCount++;
    handoffLiveSockets();
  }
  teardownActiveSession(true);
  p2pState.candidatePath = "unknown";
  if (scheduleRetry && started) {
    clearTimeout(retryTimer);
    const delay =
      BACKOFF_STEPS_MS[Math.min(retryIndex++, BACKOFF_STEPS_MS.length - 1)]!;
    const epoch = activeEpoch;
    retryTimer = setTimeout(() => {
      if (epoch === activeEpoch) void probeAndMaybeUpgrade(epoch);
    }, delay);
  }
}

function applyServerP2PStatus(status: P2PStatus) {
  p2pState.portMapState = status.portMap.state;
  p2pState.portMapProtocol = status.portMap.protocol ?? "";
  p2pState.portMapExternalPort = status.portMap.externalPort ?? 0;
  if (!status.supported) {
    fallbackToBaseline("unsupported", false);
    return;
  }
  if (!status.enabled && !ownerMode) {
    fallbackToBaseline("disabled", false);
  }
}

async function waitForSample(signal: AbortSignal) {
  signal.throwIfAborted();
  const { promise, resolve, reject } = Promise.withResolvers<void>();
  const timer = setTimeout(resolve, HYBRID_INITIAL_SAMPLE_INTERVAL_MS);
  const aborted = () => reject(signal.reason);
  signal.addEventListener("abort", aborted, { once: true });
  try {
    await promise;
  } finally {
    clearTimeout(timer);
    signal.removeEventListener("abort", aborted);
  }
}
async function probeAndMaybeUpgrade(epoch: number) {
  if (!started || epoch !== activeEpoch || probeController) return;
  const controller = new AbortController();
  probeController = controller;
  try {
    await probeDirectPeer(epoch, controller.signal);
  } catch (error) {
    if (epoch === activeEpoch && !controller.signal.aborted) {
      if (ownerMode) {
        fallbackToBaseline("fallback", true);
        p2pState.reason = "optimization_unavailable";
      } else if (
        error instanceof Error &&
        error.name === "RelayRejectedError"
      ) {
        fallbackToBaseline("rejected_relay", false);
      } else {
        fallbackToBaseline("direct_failed", true);
      }
    }
  } finally {
    if (probeController === controller) probeController = null;
  }
}

async function probeDirectPeer(epoch: number, signal: AbortSignal) {
  if (epoch !== activeEpoch) return;
  if (typeof RTCPeerConnection === "undefined") {
    p2pState.phase = "unsupported";
    return;
  }
  const status = await api("/p2p", P2PStatusSchema);
  if (epoch !== activeEpoch) return;
  ownerMode = relayOwnerAuthenticated();
  if (!ownerMode) pathPolicy = new HybridPathPolicy();
  applyServerP2PStatus(status);
  if (!status.supported || (!status.enabled && !ownerMode)) return;
  const forced = !ownerMode && hasForceFlag();
  if (
    !ownerMode &&
    !forced &&
    typeof location !== "undefined" &&
    isLocalOrOverlayHost(location.hostname)
  ) {
    p2pState.phase = "skipped_local";
    return;
  }
  p2pState.phase = "probing";
  p2pState.reason = "probing";
  const initial = await measureBaselineHttpRtt(1, signal);
  if (epoch !== activeEpoch) return;
  p2pState.baselineRttMs = medianSample(initial);
  if (!ownerMode && !forced && (p2pState.baselineRttMs ?? 0) < 15) {
    p2pState.phase = "skipped_low_rtt";
    return;
  }
  p2pState.phase = "negotiating";
  const upgradeStart = performance.now();
  const session = await negotiateDirectPeer(status.stunServers, signal);
  if (epoch !== activeEpoch || signal.aborted) {
    session.close(true);
    return;
  }
  teardownActiveSession(true);
  activeSession = session;
  const sessionPath = session.path;
  const health = new AbortController();
  healthController = health;
  retryIndex = 0;
  p2pState.candidatePath = session.path;
  p2pState.candidateType = session.candidateType;
  p2pState.ipFamily = session.ipFamily;
  session.onFailure = () => {
    if (activeSession === session) fallbackToBaseline("fallback", true);
  };
  session.pc.onconnectionstatechange = () => {
    if (activeSession !== session) return;
    const state = session.pc.connectionState;
    if (state === "disconnected" || state === "failed" || state === "closed") {
      fallbackToBaseline("fallback", true);
    }
  };
  session.control.onclose = session.onFailure;
  session.control.onerror = session.onFailure;
  let sampling = false;
  let strongAdvantageSamples = 0;
  const sample = async () => {
    if (
      sampling ||
      activeSession !== session ||
      epoch !== activeEpoch ||
      health.signal.aborted
    )
      return;
    sampling = true;
    try {
      // Both application requests start in the same round, never sequential A/B.
      const [baseline, candidate] = await Promise.all([
        measureBaselineHttpRtt(1, health.signal),
        session.sendPing(),
      ]);
      if (activeSession !== session || epoch !== activeEpoch) return;
      const pair = await inspectSelectedCandidatePair(session.pc);
      if (activeSession !== session || epoch !== activeEpoch) return;
      if (
        pair.path === "unknown" ||
        pair.path !== sessionPath ||
        session.path !== sessionPath ||
        (session.expiresAt !== null && session.expiresAt * 1000 <= Date.now())
      ) {
        fallbackToBaseline("fallback", true);
        return;
      }
      const base = medianSample(baseline);
      if (base === null) throw new Error("Missing baseline application sample");
      p2pState.baselineRttMs = base;
      p2pState.p2pRttMs = candidate;
      strongAdvantageSamples = hasStrongP2PAdvantage(base, candidate)
        ? Math.min(strongAdvantageSamples + 1, HYBRID_STABLE_SAMPLES)
        : 0;
      const decision = pathPolicy.sample(
        base,
        candidate,
        p2pState.mode === "p2p",
        performance.now(),
      );
      session.control.send(
        JSON.stringify({
          type: "peer_meta",
          candidateType: pair.candidateType,
          ipFamily: pair.ipFamily,
          rttMs: candidate,
        }),
      );
      if (decision === "fallback") {
        fallbackToBaseline("standby_hysteresis", true);
      } else if (
        decision === "upgrade" ||
        (forced && p2pState.mode !== "p2p")
      ) {
        setClientTransport(
          createP2PTransport({
            pc: session.pc,
            path: session.path,
            interactiveOnly: ownerMode,
            fallback: defaultBrowserTransport(),
            onChannelFailure: () => {
              if (
                activeSession === session &&
                (ownerMode ||
                  session.pc.connectionState === "disconnected" ||
                  session.pc.connectionState === "failed" ||
                  session.pc.connectionState === "closed")
              )
                fallbackToBaseline("fallback", true);
            },
          }),
        );
        p2pState.mode = "p2p";
        p2pState.phase = "connected";
        p2pState.reason = "stable_advantage";
        p2pState.upgradeDurationMs = Math.max(
          1,
          Math.round(performance.now() - upgradeStart),
        );
        p2pState.switchCount++;
        handoffLiveSockets({
          targetPath: sessionPath,
          allowReconnect: strongAdvantageSamples >= HYBRID_STABLE_SAMPLES,
        });
      } else if (
        p2pState.mode === "p2p" &&
        strongAdvantageSamples >= HYBRID_STABLE_SAMPLES
      ) {
        // Retry remaining baseline streams without replacing the active peer.
        handoffLiveSockets({ targetPath: sessionPath, allowReconnect: true });
      } else if (p2pState.mode !== "p2p") {
        p2pState.phase = "standby_hysteresis";
        p2pState.reason = "sampling_advantage";
      }
    } catch {
      if (activeSession === session && epoch === activeEpoch) {
        fallbackToBaseline("fallback", true);
      }
    } finally {
      sampling = false;
    }
  };
  // Initial consecutive rounds, then one ongoing paired round every 5 seconds.
  for (let i = 0; i < HYBRID_STABLE_SAMPLES && activeSession === session; i++) {
    if (i > 0 && ownerMode) await waitForSample(signal);
    await sample();
  }
  if (activeSession === session && p2pState.mode !== "p2p" && !ownerMode) {
    teardownActiveSession(true);
    p2pState.candidatePath = "unknown";
  }
  if (activeSession === session) {
    keepaliveTimer = setInterval(() => {
      void sample();
    }, HYBRID_SAMPLE_INTERVAL_MS);
  }
}

function handleNetworkChange() {
  if (!started) return;
  activeEpoch++;
  abTestController?.abort();
  probeController?.abort();
  probeController = null;
  clearTimeout(retryTimer);
  retryIndex = 0;
  fallbackToBaseline("fallback", false);
  const epoch = activeEpoch;
  retryTimer = setTimeout(() => void probeAndMaybeUpgrade(epoch), 1_000);
}

function handleForegroundEvent() {
  if (!started || document.visibilityState === "hidden") return;
  const session = activeSession;
  if (session) {
    void session.sendPing().catch(() => {
      if (activeSession === session) handleNetworkChange();
    });
  } else if (!probeController) {
    clearTimeout(retryTimer);
    void probeAndMaybeUpgrade(activeEpoch);
  }
}

export function startP2P() {
  stopP2P();
  started = true;
  hasForceFlag();
  pathPolicy = new HybridPathPolicy();
  const epoch = ++activeEpoch;
  disconnectHost = onActiveHostChange(() => stopP2P());
  ownerMode = relayOwnerAuthenticated();
  disconnectStreams = observeManagedStreams((streams) => {
    p2pState.streams = streams;
    p2pState.inputUncertain = streams.some((stream) => stream.inputUncertain);
  });
  if (typeof window !== "undefined") {
    window.addEventListener("online", handleNetworkChange);
    window.addEventListener("webmux:relay-revoked", revokeOwnerAccess);
    window.addEventListener("webmux:auth", revokeP2PAccess);
    networkConnection = (navigator as Navigator & { connection?: EventTarget })
      .connection;
    networkConnection?.addEventListener("change", handleNetworkChange);
    document.addEventListener("visibilitychange", handleForegroundEvent);
  }
  void probeAndMaybeUpgrade(epoch);
}

export function stopP2P() {
  started = false;
  disconnectHost?.();
  disconnectHost = undefined;
  disconnectStreams?.();
  disconnectStreams = undefined;
  forceMode = false;
  abTestController?.abort();
  probeController?.abort();
  probeController = null;
  activeEpoch++;
  clearTimeout(retryTimer);
  retryTimer = undefined;
  retryIndex = 0;
  if (typeof window !== "undefined") {
    window.removeEventListener("online", handleNetworkChange);
    window.removeEventListener("webmux:relay-revoked", revokeOwnerAccess);
    window.removeEventListener("webmux:auth", revokeP2PAccess);
    networkConnection?.removeEventListener("change", handleNetworkChange);
    networkConnection = undefined;
    document.removeEventListener("visibilitychange", handleForegroundEvent);
  }
  fallbackToBaseline("idle", false);
  ownerMode = false;
  p2pState.streams = [];
  p2pState.inputUncertain = false;
}

function revokeP2PAccess() {
  // Revocation closes access; it must not open replacement baseline streams.
  p2pState.mode = "ws";
  stopP2P();
  p2pState.reason = "authorization_revoked";
}

function revokeOwnerAccess() {
  if (ownerMode) revokeP2PAccess();
}

export function refreshP2PStatus(status: P2PStatus) {
  if (!status.supported || (!status.enabled && !ownerMode)) {
    refreshP2PAfterSettingsChange(status);
    return;
  }
  applyServerP2PStatus(status);
}

export function refreshP2PAfterSettingsChange(status: P2PStatus) {
  activeEpoch++;
  abTestController?.abort();
  probeController?.abort();
  probeController = null;
  clearTimeout(retryTimer);
  applyServerP2PStatus(status);
  if (started && status.supported && (status.enabled || ownerMode)) {
    fallbackToBaseline("idle", false);
    retryIndex = 0;
    void probeAndMaybeUpgrade(activeEpoch);
  }
}

export async function runP2PEchoABTest(stunServers: string[]): Promise<void> {
  if (p2pState.abTest.running) return;
  const controller = new AbortController();
  abTestController = controller;
  p2pState.abTest = {
    running: true,
    error: "",
    baselineSamples: [],
    baselineMinMs: null,
    baselineMedianMs: null,
    p2pSamples: [],
    p2pMinMs: null,
    p2pMedianMs: null,
    directSuccess: false,
    candidateType: "",
    ipFamily: "",
  };

  let tempSession: NegotiatedSession | null = null;
  const onAbort = () => tempSession?.close(true);
  controller.signal.addEventListener("abort", onAbort, { once: true });
  try {
    const baseSamples = await measureBaselineHttpRtt(8, controller.signal);
    controller.signal.throwIfAborted();
    p2pState.abTest.baselineSamples = baseSamples;
    p2pState.abTest.baselineMinMs = Math.min(...baseSamples);
    p2pState.abTest.baselineMedianMs = medianSample(baseSamples);

    let session = activeSession;
    if (!session || session.control.readyState !== "open") {
      tempSession = await negotiateDirectPeer(stunServers, controller.signal);
      session = tempSession;
    }
    controller.signal.throwIfAborted();

    const p2pSamples: number[] = [];
    for (let i = 0; i < 8; i++) {
      p2pSamples.push(await session.sendPing());
      controller.signal.throwIfAborted();
    }
    p2pState.abTest.p2pSamples = p2pSamples;
    p2pState.abTest.p2pMinMs = Math.min(...p2pSamples);
    p2pState.abTest.p2pMedianMs = medianSample(p2pSamples);
    p2pState.abTest.directSuccess = session.path === "webrtc-direct";
    p2pState.abTest.candidateType = session.candidateType;
    p2pState.abTest.ipFamily = session.ipFamily;
  } catch (err) {
    p2pState.abTest.error =
      err instanceof Error ? err.message : m.s1_p2p_ab_test_failed();
  } finally {
    controller.signal.removeEventListener("abort", onAbort);
    abTestController = null;
    tempSession?.close(true);
    p2pState.abTest.running = false;
  }
}
