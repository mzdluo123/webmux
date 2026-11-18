import { inflateSync } from "fflate";
import {
  ChatFrameSchema,
  SessionEventSchema,
  StreamPositionSchema,
  SubmissionIdSchema,
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

// The server processes query IDs on the same FIFO as PTY writes, so a receipt
// proves that every preceding input frame on that socket completed its write.
export class InputHandoffFence {
  private serial = 0;
  private inputSerial = 0;
  private confirmedSerial = 0;
  private readonly queries = new Map<string, number>();
  private readonly submissions = new Map<string, number>();
  private pending:
    | {
        id: string;
        serial: number;
        inputSerial: number;
        socket: TransportSocket;
        covered: () => void;
        cancel: () => void;
        timer: ReturnType<typeof setTimeout>;
        limit: number;
      }
    | undefined;

  sent(kind: number, payload: Uint8Array) {
    if (kind === 0 || kind === 3 || kind === 6) this.serial++;
    if (kind === 0) {
      this.inputSerial++;
      return;
    }
    if (kind === 3) {
      const target = ++this.inputSerial;
      const newline = payload.indexOf(10);
      if (newline <= 0) return;
      try {
        const raw: unknown = JSON.parse(
          new TextDecoder().decode(payload.subarray(0, newline)),
        );
        if (typeof raw === "object" && raw !== null && "id" in raw) {
          const parsed = SubmissionIdSchema.safeParse(raw.id);
          if (parsed.success) {
            if (this.submissions.size >= 256) {
              const oldest = this.submissions.keys().next().value;
              if (oldest !== undefined) this.submissions.delete(oldest);
            }
            this.submissions.set(parsed.data, target);
          }
        }
      } catch {
        // Synthetic payloads in tests still advance inputSerial above.
      }
      return;
    }
    if (kind === 4 && this.inputSerial > this.confirmedSerial) {
      try {
        const raw: unknown = JSON.parse(new TextDecoder().decode(payload));
        if (
          typeof raw === "object" &&
          raw !== null &&
          "ids" in raw &&
          Array.isArray(raw.ids)
        ) {
          let lastId: string | undefined;
          for (const candidate of raw.ids) {
            const parsed = SubmissionIdSchema.safeParse(candidate);
            if (parsed.success) lastId = parsed.data;
          }
          if (lastId !== undefined) {
            if (this.queries.size >= 64) {
              const oldest = this.queries.keys().next().value;
              if (oldest !== undefined) this.queries.delete(oldest);
            }
            this.queries.set(lastId, this.inputSerial);
          }
        }
      } catch {
        // Malformed query payloads are ignored by the server as well.
      }
    }
  }

  failedSend(kind: number) {
    if (kind === 0 || kind === 3) this.inputSerial++;
  }

  resetUncertain() {
    this.inputSerial = 0;
    this.confirmedSerial = 0;
    this.queries.clear();
    this.submissions.clear();
  }

  consumeUncertain(): boolean {
    const uncertain = this.inputSerial > this.confirmedSerial;
    this.resetUncertain();
    return uncertain;
  }

  needsFollowUpProbe(): boolean {
    return this.inputSerial > this.confirmedSerial && this.queries.size === 0;
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
      inputSerial: this.inputSerial,
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
    pending.inputSerial = this.inputSerial;
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

  receive(data: string | ArrayBuffer): "handoff" | "confirm" | "none" {
    let id: string;
    let state: string;
    try {
      const frame = terminalFrame(data);
      if (!frame || frame[0] !== 5) return "none";
      const result = SubmissionStatusSchema.safeParse(
        JSON.parse(new TextDecoder().decode(frame.subarray(1))),
      );
      if (!result.success) return "none";
      id = result.data.id;
      state = result.data.state;
    } catch {
      return "none";
    }
    const pending = this.pending;
    if (pending && id === pending.id && state === "missing") {
      this.confirmedSerial = Math.max(
        this.confirmedSerial,
        pending.inputSerial,
      );
      if (pending.serial !== this.serial) this.query();
      else {
        this.clear();
        pending.covered();
      }
      return "handoff";
    }
    let confirmed = false;
    const queried = this.queries.get(id);
    if (queried !== undefined) {
      this.queries.delete(id);
      this.confirmedSerial = Math.max(this.confirmedSerial, queried);
      confirmed = true;
    }
    const submitted = this.submissions.get(id);
    if (submitted !== undefined && state !== "received") {
      this.submissions.delete(id);
      if (state === "written" || state === "failed") {
        this.confirmedSerial = Math.max(this.confirmedSerial, submitted);
        confirmed = true;
      }
    }
    return confirmed ? "confirm" : "none";
  }
}
