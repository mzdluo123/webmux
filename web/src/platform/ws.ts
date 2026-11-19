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
import { InputHandoffFence, stageHandoff } from "./ws-handoff";
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
  type ClientTransport,
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
    candidateSocket: TransportSocket | undefined,
    timer: number | undefined,
    stableTimer: number | undefined,
    controlTimer: number | undefined,
    resumeTimer: number | undefined,
    attempt = 0,
    disposed = false,
    finished: "exited" | "missing" | "moved" | undefined;
  const controls = new Map<number, Uint8Array>();
  const id = nextManagedStreamID();
  let targetPath = typeof path === "string" ? path : path();
  const ownerStream = relayOwnerAuthenticated() || relayAuthStatus().relay;
  let suspended = relayAuthStatus().relay && !relayOwnerAuthenticated();
  let inputUncertain = false;
  let candidateTimer: number | undefined;
  let candidateTransport: ClientTransport | undefined;
  let candidateAllowReconnect = false;
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
  function clearCandidate() {
    inputFence.clear();
    clearTimeout(candidateTimer);
    candidateTimer = undefined;
    candidateTransport = undefined;
    candidateAllowReconnect = false;
    if (!candidateSocket) return;
    const prev = candidateSocket;
    candidateSocket = undefined;
    prev.onopen = null;
    prev.onmessage = null;
    prev.onclose = null;
    prev.onerror = null;
    prev.close(1000, "cancel handoff");
  }
  function clearControls() {
    clearTimeout(controlTimer);
    controlTimer = undefined;
    controls.clear();
  }
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
      // A partially accepted send has an unknown result. Never queue/replay it.
      inputFence.failedSend(kind);
      clearCandidate();
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
      clearTimeout(resumeTimer);
      resumeTimer = undefined;
      const receipt = inputFence.receive(event.data);
      if (receipt === "handoff") return;
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
      }
      if (receipt === "confirm" && inputFence.needsFollowUpProbe()) api.probe();
    };
    next.onclose = (event) => {
      if (candidateSocket === next) {
        clearCandidate();
        return;
      }
      if (socket !== next || disposed || closed) return;
      closed = true;
      clearTimeout(stableTimer);
      clearTimeout(resumeTimer);
      resumeTimer = undefined;
      clearControls();
      clearCandidate();
      if (event.code === 4410) inputFence.resetUncertain();
      else markUncertain();
      if (event.code === 4410 || event.code === 4404 || event.code === 4409) {
        finished =
          event.code === 4410
            ? "exited"
            : event.code === 4404
              ? "missing"
              : "moved";
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
    clearCandidate();
    markUncertain();
    const previous = socket;
    socket = undefined;
    previous?.close();
    state(attempt ? "reconnecting" : "connecting");
    const target = typeof path === "string" ? path : path();
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
      streamStatuses.delete(id);
      publishStreams();
      liveSockets.delete(reopen);
      handoffs.delete(handoff);
      clearTimeout(timer);
      clearTimeout(stableTimer);
      clearTimeout(resumeTimer);
      resumeTimer = undefined;
      clearControls();
      clearCandidate();
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
  };
  const reopen = () => {
    if (!disposed && !finished) open();
  };
  const handoff = (handoffOptions: HandoffOptions) => {
    if (disposed || finished) return;
    const target = typeof path === "string" ? path : path();
    if (!interactiveStreamKind(target)) return;
    if (
      handoffOptions.targetPath &&
      socket?.transportPath === handoffOptions.targetPath
    )
      return;
    const transport = currentTransport();
    if (
      candidateSocket &&
      handoffOptions.targetPath &&
      candidateSocket.transportPath === handoffOptions.targetPath &&
      candidateTransport === transport
    ) {
      candidateAllowReconnect ||= handoffOptions.allowReconnect === true;
      return;
    }
    if (socket?.readyState !== SOCKET_OPEN) {
      open();
      return;
    }
    clearCandidate();
    const previous = socket;
    const next = transport.openStream(target, {
      token: authenticationToken(),
      compress: wsCompression(),
    });
    candidateSocket = next;
    candidateTransport = transport;
    candidateAllowReconnect = handoffOptions.allowReconnect === true;
    const currentCandidate = () =>
      candidateSocket === next &&
      !disposed &&
      !finished &&
      socket === previous &&
      currentTransport() === transport;
    const failed = () => {
      if (candidateSocket !== next) return;
      const reconnect = candidateAllowReconnect && currentCandidate();
      clearCandidate();
      // open() drops unsent controls, marks uncertain input without replaying
      // it, and lets consumers restore their authoritative snapshot/cursor.
      if (reconnect) open();
    };
    if (
      handoffOptions.targetPath &&
      next.transportPath !== handoffOptions.targetPath
    ) {
      clearCandidate();
      return;
    }
    next.onopen = null;
    next.onerror = failed;
    next.onclose = failed;
    candidateTimer = window.setTimeout(failed, 10_000);
    const commit = () => {
      if (!currentCandidate()) {
        if (candidateSocket === next) clearCandidate();
        return;
      }
      if (next.readyState !== SOCKET_OPEN) {
        failed();
        return;
      }
      const buffered = staged.take();
      if (!buffered) return;
      clearTimeout(candidateTimer);
      candidateTimer = undefined;
      candidateSocket = undefined;
      candidateTransport = undefined;
      candidateAllowReconnect = false;
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
      // Deliver the candidate's authoritative position/snapshot, not baseline
      // output mixed into a new stream. Consumers own generation/cursor dedupe.
      for (const data of buffered) {
        if (socket !== next || next.readyState !== SOCKET_OPEN) return;
        next.onmessage?.({ data });
      }
      next.onopen?.();
    };
    const staged = stageHandoff(
      next,
      target,
      maxBufferedAmount,
      options.canHandoff,
      () => {
        if (!currentCandidate()) {
          if (candidateSocket === next) clearCandidate();
          return;
        }
        if (interactiveStreamKind(target) !== "terminal") return commit();
        inputFence.begin(
          previous,
          () => {
            if (!currentCandidate()) {
              if (candidateSocket === next) clearCandidate();
              return;
            }
            inputFence.resetUncertain();
            commit();
          },
          failed,
          resumeTimeout,
          maxBufferedAmount,
        );
      },
      failed,
    );
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
    clearCandidate();
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
  handoffs.add(handoff);
  open();
  return api;
}
