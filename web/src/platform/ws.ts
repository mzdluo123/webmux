import { m } from "$lib/paraglide/messages.js";
import { wsCompression } from "./ws-compression";
import {
  liveSockets,
  nextManagedStreamID,
  publishStreams,
  streamStatuses,
} from "./ws-registry";
import {
  authenticationToken,
  relayOwnerAuthenticated,
  relayAuthStatus,
} from "./relay-auth";
import { createManagedHandoff } from "./ws-handoff";
import { InputHandoffFence } from "./ws-input-fence";
import { recordWsMessage } from "./perf";
import type {
  Connection,
  ConnectOptions,
  HandoffOptions,
  SocketConnection,
} from "./ws-contracts";
import {
  currentTransport,
  protocols,
  SOCKET_OPEN,
  interactiveStreamKind,
  type TransportSocket,
} from "./transport";

export { protocols };

const maxBufferedAmount = 1024 * 1024;
const handoffs = new Set<(options: HandoffOptions) => void>();

/**
 * Seamlessly hands off all active managed connections onto `currentTransport()`
 * using make-before-break: the candidate stream opens before the previous
 * socket is closed. Explicitly allowed reconnects recover via the normal
 * snapshot/cursor path if the bounded seamless attempt cannot complete.
 */
export function handoffLiveSockets(options: HandoffOptions = {}) {
  for (const handoff of handoffs) handoff(options);
}

// Opens an authenticated one-shot stream without reconnects (for example a
// single speech-recognition session).
export function openSocket(path: string, compress = true): TransportSocket {
  return currentTransport().openStream(path, {
    token: authenticationToken(),
    compress: compress && wsCompression(),
  });
}

// How long a foregrounded OPEN socket may stay silent after `resume` before
// it is treated as a dead mobile connection and replaced.
export const resumeTimeout = 3000;
export function connect(
  // Evaluated on every attempt, so a reconnect can carry fresh query state.
  path: string | (() => string),
  onMessage: (data: string | ArrayBuffer) => void,
  onState: (state: Connection) => void,
  onError: (message: string) => void,
  options: ConnectOptions = {},
): SocketConnection {
  let socket: TransportSocket | undefined,
    timer: number | undefined,
    stableTimer: number | undefined,
    controlTimer: number | undefined,
    resumeTimer: number | undefined,
    attempt = 0,
    disposed = false,
    finished: "exited" | "missing" | "moved" | undefined;
  const controls = new Map<number, Uint8Array>();
  const id = nextManagedStreamID();
  const resolveTarget = () => (typeof path === "string" ? path : path());
  let targetPath = resolveTarget();
  const ownerStream = relayOwnerAuthenticated() || relayAuthStatus().relay;
  let suspended = relayAuthStatus().relay && !relayOwnerAuthenticated();
  let inputUncertain = false;
  const inputFence = new InputHandoffFence();
  const state = (next: Connection) => {
    streamStatuses.set(id, {
      id,
      kind: interactiveStreamKind(targetPath) ?? "other",
      path: socket?.transportPath ?? "wss",
      state: next,
      inputUncertain,
    });
    publishStreams();
    onState(next);
  };
  function markUncertain() {
    if (!inputFence.consumeUncertain()) return;
    inputUncertain = true;
    const current = streamStatuses.get(id);
    if (current) streamStatuses.set(id, { ...current, inputUncertain });
    publishStreams();
    onError(m.relay_sent_input_uncertain());
  }
  function clearControls() {
    clearTimeout(controlTimer);
    controlTimer = undefined;
    controls.clear();
  }
  const handoffController = createManagedHandoff({
    inputFence,
    isActive: () => !disposed && !finished,
    resolvePath: resolveTarget,
    getSocket: () => socket,
    openCandidate: (transport, target) =>
      transport.openStream(target, {
        token: authenticationToken(),
        compress: wsCompression(),
      }),
    adoptSocket(next, target) {
      clearTimeout(timer);
      clearTimeout(stableTimer);
      clearTimeout(resumeTimer);
      resumeTimer = undefined;
      const previous = socket;
      socket = next;
      targetPath = target;
      if (previous) {
        previous.onopen = null;
        previous.onmessage = null;
        previous.onclose = null;
        previous.onerror = null;
        previous.close(1000, "transport handoff");
      }
      // The terminal fence drained prior input; only genuine loss is uncertain.
      if (interactiveStreamKind(target) === "terminal")
        inputFence.resetUncertain();
      else markUncertain();
      wireActiveSocket(next, () => {});
    },
    reopen: () => open(),
    canHandoff: options.canHandoff,
    limit: maxBufferedAmount,
    fenceTimeout: resumeTimeout,
  });
  function transmit(kind: number, payload: Uint8Array) {
    if (
      socket?.readyState !== SOCKET_OPEN ||
      socket.bufferedAmount + payload.byteLength + 1 > maxBufferedAmount
    )
      return false;
    const frame = new Uint8Array(payload.length + 1);
    frame[0] = kind;
    frame.set(payload, 1);
    try {
      socket.send(frame);
      inputFence.sent(kind, payload);
    } catch {
      // A partially accepted send has an unknown result; keep it unqueued.
      inputFence.failedSend(kind);
      handoffController.clear();
      markUncertain();
      socket.close();
      // A false result means definitely rejected. Keep uncertain submissions
      // eligible for status queries rather than invite duplicate user input.
      return kind === 0 || kind === 3;
    }
    return true;
  }
  function flushControls() {
    clearTimeout(controlTimer);
    controlTimer = undefined;
    if (socket?.readyState !== SOCKET_OPEN) return;
    for (const [kind, payload] of controls) {
      if (transmit(kind, payload)) controls.delete(kind);
    }
    if (controls.size) controlTimer = window.setTimeout(flushControls, 50);
  }
  function wireActiveSocket(next: TransportSocket, onReady: () => void) {
    let closed = false;
    next.onopen = () => {
      if (closed) return;
      onReady();
      if (socket !== next) return;
      stableTimer = window.setTimeout(() => {
        if (socket === next) attempt = 0;
      }, 10000);
      flushControls();
      state("connected");
    };
    next.onmessage = (event) => {
      if (socket !== next || closed) return;
      const hadResumeTimer = resumeTimer !== undefined;
      clearTimeout(resumeTimer);
      resumeTimer = undefined;
      const receipt = inputFence.receive(event.data);
      if (receipt === "handoff") return;
      const started = performance.now();
      const bytes =
        typeof event.data === "string"
          ? event.data.length
          : event.data.byteLength;
      try {
        onMessage(event.data);
      } catch (error) {
        onError(
          error instanceof Error
            ? error.message
            : m.s1_ws_invalid_server_message(),
        );
        next.close(1002, "Invalid server message");
        return;
      } finally {
        recordWsMessage(targetPath, bytes, performance.now() - started);
      }
      if (
        (receipt === "confirm" || hadResumeTimer) &&
        inputFence.needsFollowUpProbe()
      ) {
        api.probe();
      }
    };
    next.onclose = (event) => {
      if (handoffController.isCandidate(next)) {
        handoffController.clear();
        return;
      }
      if (socket !== next || disposed || closed) return;
      closed = true;
      clearTimeout(stableTimer);
      clearTimeout(resumeTimer);
      resumeTimer = undefined;
      clearControls();
      handoffController.clear();
      if (event.code === 4410 || event.code === 4404) {
        inputFence.resetUncertain();
        finished = event.code === 4410 ? "exited" : "missing";
        state(finished);
        return;
      }
      markUncertain();
      if (event.code === 4409) {
        finished = "moved";
        state(finished);
        return;
      }
      state("reconnecting");
      if (ownerStream && relayAuthStatus().relay) {
        window.dispatchEvent(new Event("webmux:relay-check"));
        if (suspended || disposed) return;
      }
      const ceiling = Math.min(8000, 500 * 2 ** Math.min(attempt++, 4));
      const delay = ceiling * (0.5 + Math.random() * 0.5);
      timer = window.setTimeout(open, delay);
    };
    next.onerror = () => next.close();
  }
  function open() {
    if (disposed || finished || suspended) return;
    clearTimeout(timer);
    clearTimeout(stableTimer);
    clearTimeout(resumeTimer);
    resumeTimer = undefined;
    clearControls();
    handoffController.clear();
    markUncertain();
    const previous = socket;
    socket = undefined;
    previous?.close();
    state(attempt ? "reconnecting" : "connecting");
    const target = resolveTarget();
    targetPath = target;
    const next = currentTransport().openStream(target, {
      token: authenticationToken(),
      compress: wsCompression(),
    });
    socket = next;
    wireActiveSocket(next, () => {});
  }
  const visible = () => {
    // A probe left by a foreground that was hidden again must not replace
    // the socket in the background.
    clearTimeout(resumeTimer);
    resumeTimer = undefined;
    if (document.hidden || finished) return;
    // A suspended mobile socket may still claim OPEN after the network died.
    // Callers that can probe it keep a healthy connection.
    const current = socket;
    const probed =
      current?.readyState === SOCKET_OPEN &&
      !!options.resume?.((kind, payload) => api.send(kind, payload));
    if (!probed) {
      open();
      return;
    }
    resumeTimer = window.setTimeout(() => {
      resumeTimer = undefined;
      if (socket === current) open();
    }, resumeTimeout);
  };
  document.addEventListener("visibilitychange", visible);
  const online = () => {
    if (socket?.readyState !== SOCKET_OPEN) open();
  };
  window.addEventListener("online", online);
  const restored = (event: PageTransitionEvent) => {
    // Initial pageshow may follow an already healthy connection when fonts
    // load late. Only bfcache restores require a fresh attachment here.
    if (event.persisted) visible();
  };
  window.addEventListener("pageshow", restored);
  const api: SocketConnection = {
    send(kind: number, payload: Uint8Array) {
      if (socket?.readyState !== SOCKET_OPEN) return false;
      if (payload.byteLength + 1 > maxBufferedAmount) return false;
      // Only idempotent controls are retained: one latest resize and one resync.
      if (kind === 1 || kind === 2) {
        controls.set(kind, payload.slice());
        flushControls();
        return true;
      }
      flushControls();
      // Leave rejected terminal input with the caller; never replay it later.
      return transmit(kind, payload);
    },
    close() {
      disposed = true;
      inputFence.resetUncertain();
      streamStatuses.delete(id);
      publishStreams();
      liveSockets.delete(reopen);
      handoffs.delete(handoffController.handoff);
      clearTimeout(timer);
      clearTimeout(stableTimer);
      clearTimeout(resumeTimer);
      resumeTimer = undefined;
      clearControls();
      handoffController.clear();
      const previous = socket;
      socket = undefined;
      previous?.close();
      document.removeEventListener("visibilitychange", visible);
      window.removeEventListener("online", online);
      window.removeEventListener("pageshow", restored);
      window.removeEventListener("webmux:relay-revoked", revoked);
      window.removeEventListener("webmux:relay-restored", restoredAuth);
    },
    reconnect() {
      if (disposed || finished === "missing") return;
      finished = undefined;
      open();
    },
    probe() {
      if (
        disposed ||
        finished ||
        document.hidden ||
        resumeTimer !== undefined ||
        socket?.readyState !== SOCKET_OPEN
      )
        return;
      const current = socket;
      if (!options.resume?.((kind, payload) => api.send(kind, payload))) return;
      resumeTimer = window.setTimeout(() => {
        resumeTimer = undefined;
        if (socket === current) open();
      }, resumeTimeout);
    },
    clearUncertainty() {
      if (disposed || !inputUncertain) return;
      inputUncertain = false;
      const current = streamStatuses.get(id);
      if (current)
        streamStatuses.set(id, { ...current, inputUncertain: false });
      publishStreams();
    },
  };
  const reopen = () => {
    if (!disposed && !finished) open();
  };
  const revoked = () => {
    if (!ownerStream) return;
    if (relayAuthStatus().reason !== "reconnecting") {
      api.close();
      return;
    }
    suspended = true;
    clearTimeout(timer);
    clearTimeout(stableTimer);
    clearTimeout(resumeTimer);
    resumeTimer = undefined;
    clearControls();
    handoffController.clear();
    markUncertain();
    const previous = socket;
    socket = undefined;
    previous?.close();
    if (!finished) state("reconnecting");
  };
  const restoredAuth = () => {
    if (!suspended || disposed) return;
    suspended = false;
    open();
  };
  window.addEventListener("webmux:relay-revoked", revoked);
  window.addEventListener("webmux:relay-restored", restoredAuth);
  liveSockets.add(reopen);
  handoffs.add(handoffController.handoff);
  open();
  return api;
}
