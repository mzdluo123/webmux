import { inflateSync } from "fflate";
import {
  ChatFrameSchema,
  SessionEventSchema,
  StreamPositionSchema,
  SubmissionStatusSchema,
  type StreamPosition,
} from "../contracts/schemas";
import { randomHex } from "./ids";
import {
  interactiveStreamKind,
  SOCKET_OPEN,
  type TransportSocket,
} from "./transport";

function terminalFrame(data: string | ArrayBuffer) {
  if (typeof data === "string") return undefined;
  const frame = new Uint8Array(data);
  return frame[0] === 7 ? inflateSync(frame.subarray(1)) : frame;
}

// A candidate remains a bounded replay source, never an input writer.
export function stageHandoff(
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
        const frame = terminalFrame(event.data);
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

// The server processes this never-submitted ID on the same FIFO as PTY writes.
// Only its missing receipt proves that all preceding input passed that lane.
export class InputHandoffFence {
  private serial = 0;
  private pending:
    | {
        id: string;
        serial: number;
        socket: TransportSocket;
        covered: () => void;
        cancel: () => void;
        timer: ReturnType<typeof setTimeout>;
        limit: number;
      }
    | undefined;

  sent() {
    this.serial++;
  }

  clear() {
    clearTimeout(this.pending?.timer);
    this.pending = undefined;
  }

  begin(
    socket: TransportSocket,
    covered: () => void,
    cancel: () => void,
    timeout: number,
    limit: number,
  ) {
    this.clear();
    this.pending = {
      id: "",
      serial: this.serial,
      socket,
      covered,
      cancel,
      limit,
      timer: setTimeout(cancel, timeout),
    };
    this.query();
  }

  private query() {
    const pending = this.pending;
    if (!pending) return;
    pending.id = "hf-" + randomHex(16);
    pending.serial = this.serial;
    const payload = new TextEncoder().encode(
      JSON.stringify({ ids: [pending.id] }),
    );
    const frame = new Uint8Array(payload.length + 1);
    frame[0] = 4;
    frame.set(payload, 1);
    if (
      pending.socket.readyState !== SOCKET_OPEN ||
      pending.socket.bufferedAmount + frame.length > pending.limit
    ) {
      pending.cancel();
      return;
    }
    try {
      pending.socket.send(frame);
    } catch {
      pending.cancel();
    }
  }

  receive(data: string | ArrayBuffer) {
    const pending = this.pending;
    if (!pending) return false;
    try {
      const frame = terminalFrame(data);
      if (!frame || frame[0] !== 5) return false;
      const result = SubmissionStatusSchema.safeParse(
        JSON.parse(new TextDecoder().decode(frame.subarray(1))),
      );
      if (
        !result.success ||
        result.data.id !== pending.id ||
        result.data.state !== "missing"
      )
        return false;
    } catch {
      return false;
    }
    if (pending.serial !== this.serial) this.query();
    else {
      this.clear();
      pending.covered();
    }
    return true;
  }
}
