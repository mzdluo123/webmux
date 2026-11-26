import { inflateSync } from "fflate";
import {
  ControlStateSchema,
  ExitSchema,
  SessionInfoSchema,
  StreamPositionSchema,
  SubmissionStatusSchema,
  type ControlState,
  type SessionInfo,
  type StreamPosition,
} from "../../contracts/schemas";

type SubmissionStatus = ReturnType<typeof SubmissionStatusSchema.parse>;

/** The mounted presentation never owns the stream or its parsed model. */
export interface TerminalSocketView {
  fit(): void;
  scrollToBottom(): void;
  isVisible(): boolean;
  copy(text: string): void;
  resetFind(): void;
  preserveViewport(): boolean;
}

export interface TerminalSocketAttachment {
  themeChanged(): void;
  dispose(): void;
}

export type TerminalJsonFrame =
  | { kind: "info"; value: SessionInfo }
  | { kind: "exit"; exitCode: number }
  | { kind: "position"; value: StreamPosition }
  | { kind: "submission"; value: SubmissionStatus }
  | { kind: "control"; value: ControlState };

export function decodeTerminalFrame(data: ArrayBuffer): {
  code: number;
  payload: Uint8Array;
} {
  let frame = new Uint8Array(data);
  if (frame[0] === 7) frame = inflateSync(frame.subarray(1));
  return { code: frame[0]!, payload: frame.subarray(1) };
}

export function parseTerminalJsonFrame(
  code: number,
  payload: Uint8Array,
  decoder: TextDecoder,
): TerminalJsonFrame | undefined {
  const json = () => JSON.parse(decoder.decode(payload));
  if (code === 2)
    return { kind: "info", value: SessionInfoSchema.parse(json()) };
  if (code === 3)
    return { kind: "exit", exitCode: ExitSchema.parse(json()).exitCode };
  if (code === 4)
    return { kind: "position", value: StreamPositionSchema.parse(json()) };
  if (code === 5)
    return { kind: "submission", value: SubmissionStatusSchema.parse(json()) };
  if (code === 6)
    return { kind: "control", value: ControlStateSchema.parse(json()) };
  return undefined;
}
