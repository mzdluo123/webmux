import type { Terminal } from "@xterm/xterm";
import type { Modifiers } from "./terminal-keys";

export interface XrAction {
  id: string;
  label: string;
  run(this: void): void | Promise<void>;
  disabled?: boolean;
  confirm?: string;
}

export interface XrTextLine {
  text: string;
  tone?: "normal" | "muted" | "added" | "removed" | "error";
}

export interface XrTextPanel {
  id: string;
  kind: "chat" | "code" | "diff";
  title: string;
  lines: readonly XrTextLine[];
  actions?: readonly XrAction[];
}

/** A presentation of an existing mounted session; never owns its socket. */
export interface XrSessionSource {
  id: string;
  readonly title: string;
  readonly status: string;
  readonly terminal: Terminal | undefined;
  readonly graphicsUnsupported: boolean;
  readonly modifiers: Modifiers;
  readonly shiftLocked: boolean;
  toggleModifier(key: keyof Modifiers): void;
  resetModifiers(): void;
  lockModifier(key: "shift"): void;
  panels(): readonly XrTextPanel[];
  actions(): readonly XrAction[];
  input(value: string): void;
  focus(): void;
  latest(): void;
  scroll(lines: number): void;
  resize(cols: number, rows: number): boolean;
  setImmersive(active: boolean): void;
}

export interface XrAvailableSession {
  id: string;
  title: string;
}

export interface XrWorkspaceOptions {
  initialId: string;
  sources(this: void): readonly XrSessionSource[];
  available(this: void): readonly XrAvailableSession[];
  open(this: void, id: string): void | Promise<void>;
  error(this: void, message: string): void;
  exit(this: void): void;
}

export interface XrWorkspaceRuntime {
  dispose(): void;
}

export interface XrHeaderControl {
  readonly supported: boolean;
  readonly active: boolean;
  readonly busy: boolean;
  enter(): void;
  exit(): void;
}
