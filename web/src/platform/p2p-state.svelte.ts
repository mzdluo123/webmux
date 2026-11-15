import type { ManagedStreamStatus } from "./ws";

export type P2PPhase =
  | "idle"
  | "disabled"
  | "unsupported"
  | "skipped_local"
  | "skipped_low_rtt"
  | "probing"
  | "negotiating"
  | "connected"
  | "rejected_relay"
  | "direct_failed"
  | "standby_hysteresis"
  | "fallback";

export interface P2PABTestSummary {
  running: boolean;
  error: string;
  baselineSamples: number[];
  baselineMinMs: number | null;
  baselineMedianMs: number | null;
  p2pSamples: number[];
  p2pMinMs: number | null;
  p2pMedianMs: number | null;
  directSuccess: boolean;
  candidateType: string;
  ipFamily: string;
}

export const p2pState = $state({
  mode: "ws" as "ws" | "p2p",
  phase: "idle" as P2PPhase,
  candidateType: "",
  baselinePath: "wss" as const,
  candidatePath: "unknown" as "unknown" | "webrtc-direct" | "webrtc-turn",
  streams: [] as ManagedStreamStatus[],
  inputUncertain: false,
  reason: "",
  ipFamily: "",
  portMapState: "off" as "off" | "probing" | "mapped" | "unavailable",
  portMapProtocol: "",
  portMapExternalPort: 0,
  baselineRttMs: null as number | null,
  p2pRttMs: null as number | null,
  upgradeDurationMs: null as number | null,
  switchCount: 0,
  firstReadyMs: null as number | null,
  abTest: {
    running: false,
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
  } as P2PABTestSummary,
});

const bootTime = typeof performance !== "undefined" ? performance.now() : 0;

export function markFirstScreenReady() {
  if (p2pState.firstReadyMs === null && typeof performance !== "undefined") {
    p2pState.firstReadyMs = Math.max(
      0,
      Math.round(performance.now() - bootTime),
    );
  }
}
