import type {
  ControlState,
  SessionInfo,
  SubmissionProgress,
} from "../../contracts/schemas";
import type { Connection } from "../../platform/ws-contracts";
import type { Modifiers } from "./terminal-keys";
import type { Terminal } from "@xterm/xterm";
import type { SearchAddon } from "@xterm/addon-search";
import { m } from "$lib/paraglide/messages.js";

export interface TerminalFindOptions {
  caseSensitive: boolean;
  wholeWord: boolean;
  regex: boolean;
}

export interface TerminalFindResults {
  resultIndex: number;
  resultCount: number;
}

export const terminalFindLimit = 1000;

export function terminalFindError(query: string, regex: boolean): string {
  if (!query || !regex) return "";
  try {
    new RegExp(query);
    return "";
  } catch {
    return m.s4_find_invalid_regex();
  }
}

export function terminalFindResultText(
  results: TerminalFindResults | null,
): string {
  if (!results) return "";
  if (results.resultCount >= terminalFindLimit)
    return m.s4_find_results_at_limit();
  if (!results.resultCount) return m.s4_find_no_matches();
  if (results.resultIndex < 0)
    return m.s4_find_match_count({ count: String(results.resultCount) });
  return `${results.resultIndex + 1}/${results.resultCount}`;
}

export function terminalFindScope(
  scrollback: number,
  bufferType: "normal" | "alternate",
  truncated: boolean,
): string {
  const scope = m.s4_find_scope_buffer({ lines: String(scrollback) });
  return [
    scope,
    bufferType === "alternate" ? m.s4_find_scope_alt_screen() : "",
    truncated ? m.s4_find_scope_truncated() : "",
  ]
    .filter(Boolean)
    .join(" · ");
}

export function terminalFindReplayText(
  restoring: boolean,
  replayed: boolean,
): string {
  return restoring
    ? m.s4_find_replaying()
    : replayed
      ? m.s4_find_replayed_press_enter()
      : "";
}

export function isTerminalFindShortcut(
  event: Pick<
    KeyboardEvent,
    "key" | "ctrlKey" | "shiftKey" | "altKey" | "metaKey" | "isComposing"
  >,
  platform: string,
): boolean {
  return (
    !event.isComposing &&
    !event.altKey &&
    event.key.toLowerCase() === "f" &&
    ((event.ctrlKey && event.shiftKey && !event.metaKey) ||
      (/Mac/.test(platform) &&
        event.metaKey &&
        !event.ctrlKey &&
        !event.shiftKey))
  );
}

export function executeTerminalFind(
  terminal: Terminal,
  search: SearchAddon,
  query: string,
  options: TerminalFindOptions,
  previous: boolean,
  incremental: boolean,
  findActive: boolean,
) {
  if (incremental || !findActive) {
    search.clearDecorations();
    // The addon starts at row zero without a selection. Seed a one-cell
    // anchor at the viewport, synchronously replaced by the actual match
    // (or cleared on no match). Never seed before validating the query.
    terminal.select(0, terminal.buffer.active.viewportY, 1);
  }
  const theme = terminal.options.theme;
  const searchOptions = {
    ...options,
    incremental,
    decorations: {
      matchBackground: theme?.yellow,
      matchBorder: theme?.yellow,
      matchOverviewRuler: theme?.yellow ?? "#a16207",
      activeMatchBackground: theme?.blue,
      activeMatchBorder: theme?.blue,
      activeMatchColorOverviewRuler: theme?.blue ?? "#2563eb",
    },
  };
  if (previous) search.findPrevious(query, searchOptions);
  else search.findNext(query, searchOptions);
}

export interface TerminalController {
  readonly emulator: Terminal | undefined;
  resizeGrid(cols: number, rows: number): boolean;
  readonly ready: boolean;
  readonly info: SessionInfo | undefined;
  readonly connection: Connection;
  readonly restoring: boolean;
  /** The run whose output this page has drawn, if any. */
  readonly drawn: string | undefined;
  readonly exitCode: number | null;
  readonly fontSize: number;
  readonly modifiers: Modifiers;
  readonly shiftLocked: boolean;
  readonly error: string;
  readonly selection: string;
  // Whether the newest output is in view (see `updateBottom`).
  readonly atBottom: boolean;
  readonly historyTruncated: boolean;
  readonly control: ControlState | undefined;
  readonly controlling: boolean;
  readonly clientId: string;
  readonly canFind: boolean;
  readonly findResults: TerminalFindResults | null;
  readonly findReplayed: boolean;
  readonly bufferType: "normal" | "alternate";
  readonly scrollback: number;
  findNext(
    query: string,
    options: TerminalFindOptions,
    incremental?: boolean,
  ): void;
  findPrevious(query: string, options: TerminalFindOptions): void;
  clearFind(): void;
  claim(): void;
  submit(
    submissionId: string,
    text: string,
    enter: boolean,
    report: (state: SubmissionProgress) => void,
  ): boolean;
  track(
    submissionId: string,
    report: (state: SubmissionProgress) => void,
  ): void;
  untrack(submissionId: string): void;
  copySelection(): void;
  clearSelection(): void;
  send(input: string): boolean;
  setFontSize(value: number): void;
  resetModifiers(): void;
  toggleModifier(key: keyof Modifiers): void;
  lockModifier(key: "shift"): void;
  paste(text: string): boolean;
  // Toggles the soft keyboard; `beforeOpen` runs just before focusing.
  keyboard(beforeOpen?: (target: HTMLElement) => void): void;
  clear(): void;
  dismissError(): void;
  resync(): void;
  // Reattaches after the session moved back to its terminal side.
  reconnect(): void;
  fit(): void;
  scrollToBottom(): void;
  focus(): void;
}
