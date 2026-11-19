import {
  ChatFrameSchema,
  SessionEventSchema,
  StreamPositionSchema,
  type StreamPosition,
} from "../contracts/schemas";
import type { ConnectOptions, HandoffOptions } from "./ws-contracts";
import { decodeTerminalFrame, InputHandoffFence } from "./ws-input-fence";
import {
  currentTransport,
  interactiveStreamKind,
  SOCKET_OPEN,
  type ClientTransport,
  type TransportSocket,
} from "./transport";

// A candidate remains a bounded replay source while staging; input stays on the
// active socket until the candidate is ready and the input fence drains.
function stageHandoff(
  next: TransportSocket,
  target: string,
  limit: number,
  canHandoff: ((position: StreamPosition) => boolean) | undefined,
  onReady: () => void,
  onCancel: () => void,
) {
  let buffered: (string | ArrayBuffer)[] = [];
  let bytes = 0;
  let reset = false;
  let awaitingReset = false;
  let ready = false;
  let position: StreamPosition | undefined;
  const requestReset = () => {
    buffered = [];
    bytes = 0;
    ready = false;
    reset = false;
    awaitingReset = true;
    next.send(new Uint8Array([2]));
  };
  next.onmessage = (event) => {
    bytes +=
      typeof event.data === "string"
        ? event.data.length * 2
        : event.data.byteLength;
    if (bytes > limit) return onCancel();
    buffered.push(event.data);
    if (ready) return;
    try {
      const kind = interactiveStreamKind(target);
      if (kind === "terminal") {
        const frame = decodeTerminalFrame(event.data);
        if (!frame) return;
        if (awaitingReset && frame[0] !== 4) {
          buffered.pop();
          return;
        }
        if (frame[0] === 4) {
          position = StreamPositionSchema.parse(
            JSON.parse(new TextDecoder().decode(frame.subarray(1))),
          );
          reset = position.mode === "reset";
          if (!reset && canHandoff && !canHandoff(position)) {
            requestReset();
            return;
          }
          awaitingReset = false;
          ready = !reset;
        } else if (frame[0] === 1 && reset) ready = true;
      } else if (typeof event.data === "string") {
        const value: unknown = JSON.parse(event.data);
        ready =
          kind === "chat"
            ? ChatFrameSchema.parse(value).type === "snapshot"
            : SessionEventSchema.parse(value).type === "snapshot";
      }
      if (ready) onReady();
    } catch {
      onCancel();
    }
  };
  return {
    take() {
      // Baseline output can advance while its input barrier is in flight.
      if (position && !reset && canHandoff && !canHandoff(position)) {
        try {
          requestReset();
        } catch {
          onCancel();
        }
        return undefined;
      }
      return buffered;
    },
  };
}

export interface ManagedHandoffDeps {
  inputFence: InputHandoffFence;
  isActive: () => boolean;
  resolvePath: () => string;
  getSocket: () => TransportSocket | undefined;
  openCandidate: (
    transport: ClientTransport,
    target: string,
  ) => TransportSocket;
  adoptSocket: (next: TransportSocket, target: string) => void;
  reopen: () => void;
  canHandoff: ConnectOptions["canHandoff"];
  limit: number;
  fenceTimeout: number;
}

function isPeerTransportPath(path: TransportSocket["transportPath"]): boolean {
  return path === "webrtc-direct" || path === "webrtc-turn";
}

export function createManagedHandoff(deps: ManagedHandoffDeps) {
  let candidateSocket: TransportSocket | undefined;
  let candidateTimer: number | undefined;
  let candidateTransport: ClientTransport | undefined;
  let candidateAllowReconnect = false;
  const { inputFence } = deps;

  function clear() {
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

  function handoff(handoffOptions: HandoffOptions) {
    if (!deps.isActive()) return;
    const target = deps.resolvePath();
    const current = deps.getSocket();
    if (!interactiveStreamKind(target)) {
      if (
        handoffOptions.allowReconnect &&
        isPeerTransportPath(current?.transportPath) &&
        (!handoffOptions.targetPath ||
          current?.transportPath !== handoffOptions.targetPath)
      ) {
        deps.reopen();
      }
      return;
    }
    if (
      handoffOptions.targetPath &&
      current?.transportPath === handoffOptions.targetPath
    ) {
      if (
        candidateSocket &&
        candidateSocket.transportPath !== handoffOptions.targetPath
      ) {
        clear();
      }
      return;
    }
    if (current?.readyState !== SOCKET_OPEN) {
      deps.reopen();
      return;
    }
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
    clear();
    const previous = current;
    const next = deps.openCandidate(transport, target);
    candidateSocket = next;
    candidateTransport = transport;
    candidateAllowReconnect = handoffOptions.allowReconnect === true;
    const currentCandidate = () =>
      candidateSocket === next &&
      deps.isActive() &&
      deps.getSocket() === previous &&
      currentTransport() === transport;
    const failed = () => {
      if (candidateSocket !== next) return;
      const reconnect = candidateAllowReconnect && currentCandidate();
      clear();
      if (reconnect) deps.reopen();
    };
    if (
      handoffOptions.targetPath &&
      next.transportPath !== handoffOptions.targetPath
    ) {
      clear();
      return;
    }
    next.onopen = null;
    next.onerror = failed;
    next.onclose = failed;
    candidateTimer = window.setTimeout(failed, 10_000);
    const commit = () => {
      if (!currentCandidate()) {
        if (candidateSocket === next) clear();
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
      deps.adoptSocket(next, target);
      for (const data of buffered) {
        if (deps.getSocket() !== next || next.readyState !== SOCKET_OPEN)
          return;
        next.onmessage?.({ data });
      }
      next.onopen?.();
    };
    const staged = stageHandoff(
      next,
      target,
      deps.limit,
      deps.canHandoff,
      () => {
        if (!currentCandidate()) {
          if (candidateSocket === next) clear();
          return;
        }
        if (interactiveStreamKind(target) !== "terminal") return commit();
        inputFence.begin(
          previous,
          () => {
            if (!currentCandidate()) {
              if (candidateSocket === next) clear();
              return;
            }
            inputFence.resetUncertain();
            commit();
          },
          failed,
          deps.fenceTimeout,
          deps.limit,
        );
      },
      failed,
    );
  }

  return {
    clear,
    handoff,
    isCandidate: (sock: TransportSocket) => candidateSocket === sock,
  };
}
