import { inflateSync } from "fflate";
import {
  SubmissionIdSchema,
  SubmissionStatusSchema,
} from "../contracts/schemas";
import { randomHex } from "./ids";
import { SOCKET_OPEN, type TransportSocket } from "./transport";

export function decodeTerminalFrame(
  data: string | ArrayBuffer,
): Uint8Array | undefined {
  if (typeof data === "string") return undefined;
  const frame = new Uint8Array(data);
  return frame[0] === 7 ? inflateSync(frame.subarray(1)) : frame;
}

function sendFenceQuery(
  socket: TransportSocket,
  prefix: string,
  limit: number,
): string | null {
  const id = prefix + randomHex(16);
  const payload = new TextEncoder().encode(JSON.stringify({ ids: [id] }));
  const frame = new Uint8Array(payload.length + 1);
  frame[0] = 4;
  frame.set(payload, 1);
  if (
    socket.readyState !== SOCKET_OPEN ||
    socket.bufferedAmount + frame.length > limit
  ) {
    return null;
  }
  try {
    socket.send(frame);
    return id;
  } catch {
    return null;
  }
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
    pending.serial = this.serial;
    pending.inputSerial = this.inputSerial;
    const id = sendFenceQuery(pending.socket, "hf-", pending.limit);
    if (!id) {
      pending.cancel();
      return;
    }
    pending.id = id;
  }

  receive(data: string | ArrayBuffer): "handoff" | "confirm" | "none" {
    let id: string;
    let state: string;
    try {
      const frame = decodeTerminalFrame(data);
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
