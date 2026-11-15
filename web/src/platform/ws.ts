import { store, stored } from "./storage";
import { authenticationToken, relayOwnerAuthenticated } from "./relay-auth";
import { InputHandoffFence, stageHandoff } from "./ws-handoff";
import type { StreamPosition } from "../contracts/schemas";
import {
  currentTransport,
  protocols,
  SOCKET_OPEN,
  interactiveStreamKind,
  type InteractiveStreamKind,
  type TransportPath,
  type TransportSocket,
} from "./transport";

export { protocols };

const maxBufferedAmount = 1024 * 1024;
const compressionKey = "wsCompression";
// Every live connection, so a compression change can reattach them at once.
const live = new Set<() => void>();
const handoffs = new Set<() => void>();

export interface ManagedStreamStatus {
  id: number;
  kind: InteractiveStreamKind | "other";
  path: TransportPath;
  state: Connection;
  inputUncertain: boolean;
}
let streamCounter = 0;
const streamStatuses = new Map<number, ManagedStreamStatus>();
const streamObservers = new Set<(streams: ManagedStreamStatus[]) => void>();
function publishStreams() {
  for (const observer of streamObservers)
    observer([...streamStatuses.values()]);
}
export function observeManagedStreams(
  observer: (streams: ManagedStreamStatus[]) => void,
) {
  streamObservers.add(observer);
  observer([...streamStatuses.values()]);
  return () => {
    streamObservers.delete(observer);
  };
}

// Per-device permessage-deflate preference. Browsers always offer the
// extension, so the server negotiates it only when the URL asks. Until the
// user chooses, devices whose primary pointer is coarse (phones, tablets)
// default to on: they are the ones on metered, slow links.
export function wsCompression(): boolean {
  const choice = stored(compressionKey);
  if (choice === "on") return true;
  if (choice === "off") return false;
  return (
    typeof matchMedia === "function" && matchMedia("(pointer: coarse)").matches
  );
}
export function setWsCompression(enabled: boolean) {
  store(compressionKey, enabled ? "on" : "off");
  for (const reopen of live) reopen();
}

/**
 * Seamlessly hands off all active managed connections onto `currentTransport()`
 * using make-before-break: the candidate stream opens before the previous
 * socket is closed, avoiding any disconnect state flicker or output gap.
 */
export function handoffLiveSockets() {
  for (const handoff of handoffs) handoff();
}

// Opens an authenticated one-shot stream without reconnects (for example a
// single speech-recognition session).
export function openSocket(path: string, compress = true): TransportSocket {
  return currentTransport().openStream(path, {
    token: authenticationToken(),
    compress: compress && wsCompression(),
  });
}

export type Connection =
  | "connecting"
  | "connected"
  | "reconnecting"
  | "exited"
  | "missing"
  // 4409: a hybrid session moved to its other side. Terminal: the page
  // switches view and reconnects explicitly once the terminal is back.
  | "moved";
export interface SocketConnection {
  // False is definitely rejected. True includes an uncertain send outcome,
  // reported via onError, which callers must never replay. Only controls coalesce.
  send(kind: number, payload: Uint8Array): boolean;
  close(): void;
  // Explicit recovery may reopen an exited or moved session, not a missing one.
  reconnect(): void;
  // Probes an OPEN socket via `resume` when none is in flight, replacing a
  // silent mobile connection after resumeTimeout.
  probe(): void;
}
// How long a foregrounded OPEN socket may stay silent after `resume` before
// it is treated as a dead mobile connection and replaced.
export const resumeTimeout = 3000;
export interface ConnectOptions {
  // Asks the server for a reply on a socket that still claims OPEN when the
  // page returns to the foreground, and returns true once it sent one. Any
  // message within resumeTimeout proves the connection healthy; silence
  // replaces it. Returning false, or omitting it, reattaches immediately and
  // takes the server snapshot as the source of truth.
  resume?: (send: SocketConnection["send"]) => boolean;
  // Terminal consumers validate against their current rendered recovery point.
  canHandoff?: (position: StreamPosition) => boolean;
}
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
  const id = ++streamCounter;
  let targetPath = typeof path === "string" ? path : path();
  const ownerStream = relayOwnerAuthenticated();
  let inputUncertain = false;
  let sentInput = false;
  let candidateTimer: number | undefined;
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
    if (!sentInput) return;
    sentInput = false;
    inputUncertain = true;
    const current = streamStatuses.get(id);
    if (current) streamStatuses.set(id, { ...current, inputUncertain });
    publishStreams();
    onError("先前输入的执行结果无法确认；未自动重发，请检查会话后继续。");
  }
  function clearCandidate() {
    inputFence.clear();
    clearTimeout(candidateTimer);
    candidateTimer = undefined;
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
      if (kind === 0 || kind === 3) sentInput = true;
      if (kind === 0 || kind === 3 || kind === 6) inputFence.sent();
    } catch {
      // A partially accepted send has an unknown result. Never queue/replay it.
      if (kind === 0 || kind === 3) sentInput = true;
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
      if (inputFence.receive(event.data)) return;
      try {
        onMessage(event.data);
      } catch (error) {
        onError(error instanceof Error ? error.message : "无效的服务器消息");
        next.close(1002, "Invalid server message");
      }
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
      markUncertain();
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
      const ceiling = Math.min(8000, 500 * 2 ** Math.min(attempt++, 4));
      const delay = ceiling * (0.5 + Math.random() * 0.5);
      timer = window.setTimeout(open, delay);
    };
    next.onerror = () => next.close();
  }
  function open() {
    if (disposed || finished) return;
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
      live.delete(reopen);
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
  const handoff = () => {
    if (disposed || finished) return;
    if (socket?.readyState !== SOCKET_OPEN) {
      open();
      return;
    }
    clearCandidate();
    const target = typeof path === "string" ? path : path();
    if (!interactiveStreamKind(target)) return;
    const next = currentTransport().openStream(target, {
      token: authenticationToken(),
      compress: wsCompression(),
    });
    candidateSocket = next;
    next.onopen = null;
    next.onerror = () => {
      if (candidateSocket === next) clearCandidate();
    };
    next.onclose = () => {
      if (candidateSocket === next) clearCandidate();
    };
    candidateTimer = window.setTimeout(clearCandidate, 10_000);
    const commit = () => {
      if (candidateSocket !== next || disposed || finished) return;
      if (next.readyState !== SOCKET_OPEN) {
        clearCandidate();
        return;
      }
      const buffered = staged.take();
      if (!buffered) return;
      clearTimeout(candidateTimer);
      candidateTimer = undefined;
      candidateSocket = undefined;
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
      if (interactiveStreamKind(target) === "terminal") sentInput = false;
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
        if (candidateSocket !== next || !socket) return;
        if (interactiveStreamKind(target) !== "terminal") return commit();
        inputFence.begin(
          socket,
          () => {
            sentInput = false;
            commit();
          },
          clearCandidate,
          resumeTimeout,
          maxBufferedAmount,
        );
      },
      () => {
        if (candidateSocket === next) clearCandidate();
      },
    );
  };
  const revoked = () => {
    if (ownerStream) api.close();
  };
  window.addEventListener("webmux:relay-revoked", revoked);
  live.add(reopen);
  handoffs.add(handoff);
  open();
  return api;
}
