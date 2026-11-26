import type { SessionInfo } from "../../contracts/schemas";
import {
  chatDocumentScroll,
  isStandalone,
  type ChatScrollState,
} from "../../platform/capabilities";
import { errorMessage } from "../../platform/error";
import { sessionPanelIds } from "./session-dock";
import {
  createSplitShellSession,
  directionalPane,
  MAX_PANES,
  parsePrefixKeydown,
  type PaneRect,
  type SplitDirection,
} from "./terminal-layout";
import { m } from "$lib/paraglide/messages.js";

export async function runTerminalSplit(
  source: SessionInfo,
  direction: SplitDirection,
  target: string,
  hooks: {
    isMounted(): boolean;
    onStart(): void;
    onCreated(
      session: SessionInfo,
      direction: SplitDirection,
      target: string,
    ): Promise<void>;
    onError(message: string): void;
    onFinally(): void;
  },
) {
  hooks.onStart();
  try {
    const session = await createSplitShellSession(source);
    if (hooks.isMounted()) await hooks.onCreated(session, direction, target);
  } catch (err) {
    if (hooks.isMounted())
      hooks.onError(m.s1_split_pane_failed({ error: errorMessage(err) }));
  } finally {
    if (hooks.isMounted()) hooks.onFinally();
  }
}

export function resolveTerminalDocumentScroll(options: {
  pageActive: boolean;
  ids: readonly string[];
  scrollStates: Record<string, ChatScrollState>;
  viewOf(id: string): string | undefined;
  isPanelVisible(panel: string): boolean;
}): boolean {
  if (!options.pageActive || options.ids.length !== 1) return false;
  const firstId = options.ids[0];
  const current = firstId ? options.scrollStates[firstId] : undefined;
  if (!firstId || !current) return false;
  const panels = sessionPanelIds(firstId);
  if (
    !options.isPanelVisible(panels.terminal) ||
    options.viewOf(firstId) !== "chat"
  )
    return false;
  return chatDocumentScroll({
    ...current,
    singlePane: true,
    standalone: isStandalone(),
  });
}

export interface TerminalKeydownHandlers {
  getPrefix(): boolean;
  setPrefix(value: boolean): void;
  getActive(): string;
  getIds(): readonly string[];
  isBusy(): boolean;
  sendPrefix(activeId: string): void;
  split(direction: SplitDirection, activeId: string): void;
  focus(sessionId: string): void;
  toggleZoom(activeId: string): void;
  close(activeId: string): void;
  paneRect(sessionId: string): PaneRect | undefined;
}

function messageRevealTarget(search: string): string | undefined {
  const query = new URLSearchParams(search);
  const target = query.get("messageSession");
  return query.get("message") && target ? target : undefined;
}

export async function ensureExistingPane(options: {
  sessionId: string;
  activate(sessionId: string): boolean;
  getIds(): readonly string[];
  add(sessionId: string, direction: SplitDirection): Promise<void>;
  getError(): string;
  clearError(): void;
}) {
  if (options.activate(options.sessionId)) return;
  if (options.getIds().length >= MAX_PANES)
    throw new Error(m.s1_max_panes_reopen({ max: String(MAX_PANES) }));
  options.clearError();
  await options.add(options.sessionId, "horizontal");
  if (!options.getIds().includes(options.sessionId))
    throw new Error(
      options.getError() || m.s1_open_session_failed_unmodified(),
    );
}

export async function ensureMessagePane(options: {
  search: string;
  getIds(): readonly string[];
  add(sessionId: string, direction: SplitDirection): Promise<void>;
  onLimitError(message: string): void;
  onRevealed(target: string): void;
}) {
  const target = messageRevealTarget(options.search);
  if (!target) return;
  if (!options.getIds().includes(target)) {
    if (options.getIds().length >= MAX_PANES) {
      options.onLimitError(
        m.s1_max_panes_reveal_message({ max: String(MAX_PANES) }),
      );
      return;
    }
    await options.add(target, "horizontal");
    if (!options.getIds().includes(target)) return;
  }
  options.onRevealed(target);
}

export function handleTerminalPageKeydown(
  event: KeyboardEvent,
  handlers: TerminalKeydownHandlers,
) {
  const cmd = parsePrefixKeydown(event, handlers.getPrefix());
  if (cmd.kind === "ignore") return;
  event.preventDefault();
  event.stopImmediatePropagation();
  if (cmd.kind === "enter-prefix") {
    handlers.setPrefix(true);
    return;
  }
  handlers.setPrefix(false);
  const active = handlers.getActive();
  const ids = handlers.getIds();
  if (cmd.kind === "send-prefix") handlers.sendPrefix(active);
  else if (cmd.kind === "split") handlers.split(cmd.direction, active);
  else if (cmd.kind === "cycle")
    handlers.focus(ids[(ids.indexOf(active) + 1) % ids.length]!);
  else if (cmd.kind === "zoom") handlers.toggleZoom(active);
  else if (cmd.kind === "close" && !handlers.isBusy()) handlers.close(active);
  else if (cmd.kind === "arrow") {
    const panes = ids.flatMap((sessionId) => {
      const rect = handlers.paneRect(sessionId);
      return rect ? [{ ...rect, sessionId }] : [];
    });
    const next = directionalPane(panes, active, cmd.key);
    if (next) handlers.focus(next);
  }
}
