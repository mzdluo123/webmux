import { m } from "$lib/paraglide/messages.js";
import { medianSample } from "./p2p-framing";
import { negotiateDirectPeer, type NegotiatedSession } from "./p2p-session";
import { measureBaselineHttpRtt } from "./p2p-signaling";
import { p2pState } from "./p2p-state.svelte";

export async function executeP2PEchoABTest(
  stunServers: string[],
  activeSession: NegotiatedSession | null,
  signal: AbortSignal,
): Promise<void> {
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
  signal.addEventListener("abort", onAbort, { once: true });
  try {
    const baseSamples = await measureBaselineHttpRtt(8, signal);
    signal.throwIfAborted();
    p2pState.abTest.baselineSamples = baseSamples;
    p2pState.abTest.baselineMinMs = Math.min(...baseSamples);
    p2pState.abTest.baselineMedianMs = medianSample(baseSamples);

    let session = activeSession;
    if (!session || session.control.readyState !== "open") {
      tempSession = await negotiateDirectPeer(stunServers, signal);
      session = tempSession;
    }
    signal.throwIfAborted();

    const p2pSamples: number[] = [];
    for (let i = 0; i < 8; i++) {
      p2pSamples.push(await session.sendPing());
      signal.throwIfAborted();
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
    signal.removeEventListener("abort", onAbort);
    tempSession?.close(true);
    p2pState.abTest.running = false;
  }
}
