import type { NegotiatedSession } from "./p2p-session";
import { defaultBrowserTransport } from "./transport";
import { handoffLiveSockets } from "./ws";

const DRAIN_TIMEOUT_MS = 10_000;

export function baselineTransportPath(): "ws" | "wss" {
  const origin = defaultBrowserTransport().origin;
  if (typeof origin === "string") {
    return origin.startsWith("http:") ? "ws" : "wss";
  }
  return typeof location !== "undefined" && location.protocol === "http:"
    ? "ws"
    : "wss";
}

function hasPeerStreams(streams: readonly { path: string }[]): boolean {
  return streams.some(
    (stream) =>
      stream.path === "webrtc-direct" || stream.path === "webrtc-turn",
  );
}

export function isSessionHealthy(
  session: NegotiatedSession | null,
): session is NegotiatedSession {
  return (
    session !== null &&
    session.pc.connectionState === "connected" &&
    session.control.readyState === "open"
  );
}

// Retains a healthy peer session while live streams complete make-before-break
// migration back onto the baseline WebSocket transport, then closes it.
export function createPeerDrainSlot() {
  let drainingSession: NegotiatedSession | null = null;
  let drainTimer: ReturnType<typeof setTimeout> | undefined;

  function teardown(notifyServer: boolean): boolean {
    clearTimeout(drainTimer);
    drainTimer = undefined;
    const current = drainingSession;
    drainingSession = null;
    if (!current) return false;
    current.close(notifyServer);
    return true;
  }

  function begin(
    session: NegotiatedSession,
    getStreams: () => readonly { path: string }[],
  ) {
    teardown(true);
    drainingSession = session;
    const failDrain = () => {
      if (drainingSession !== session) return;
      teardown(true);
      handoffLiveSockets({
        targetPath: baselineTransportPath(),
        allowReconnect: true,
      });
    };
    session.onFailure = failDrain;
    session.control.onclose = failDrain;
    session.control.onerror = failDrain;
    session.pc.onconnectionstatechange = () => {
      if (drainingSession !== session) return;
      const state = session.pc.connectionState;
      if (
        state === "disconnected" ||
        state === "failed" ||
        state === "closed"
      ) {
        failDrain();
      }
    };
    handoffLiveSockets({
      targetPath: baselineTransportPath(),
      allowReconnect: true,
    });
    if (drainingSession === session) {
      if (!hasPeerStreams(getStreams())) {
        teardown(true);
      } else {
        drainTimer = setTimeout(() => {
          if (drainingSession === session) teardown(true);
        }, DRAIN_TIMEOUT_MS);
      }
    }
  }

  function observeStreams(streams: readonly { path: string }[]) {
    if (drainingSession && !hasPeerStreams(streams)) {
      teardown(true);
    }
  }

  return {
    begin,
    observeStreams,
    teardown,
  };
}
