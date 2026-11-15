import type { StreamPosition } from "../contracts/schemas";
import type { InteractiveStreamKind, TransportPath } from "./transport";

export type Connection =
  | "connecting"
  | "connected"
  | "reconnecting"
  | "exited"
  | "missing"
  // 4409: a hybrid session moved to its other side. Terminal: the page
  // switches view and reconnects explicitly once the terminal is back.
  | "moved";

export interface ManagedStreamStatus {
  id: number;
  kind: InteractiveStreamKind | "other";
  path: TransportPath;
  state: Connection;
  inputUncertain: boolean;
}

export interface HandoffOptions {
  targetPath?: TransportPath;
  allowReconnect?: boolean;
}

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
