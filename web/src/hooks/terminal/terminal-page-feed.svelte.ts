import { SvelteMap } from "svelte/reactivity";
import type { SessionInfo } from "../../contracts/schemas";
import { applySessionEvent } from "../../domains/sessions/session-events";
import { connectSessions, type SessionEvent } from "../sessions/session-feed";

interface TerminalPageFeedOptions {
  ids: () => string[];
  info: () => Record<string, SessionInfo>;
  forwarding: () => boolean;
  chatEnabled: () => boolean;
  forward: () =>
    | {
        isPromptedPort: (port: number) => boolean;
        observeSessionEvent: (event: SessionEvent) => void;
      }
    | undefined;
  error: (message: string) => void;
}

export function useTerminalPageFeed(options: TerminalPageFeedOptions) {
  // Every session, for the forwarding dialog's detected ports.
  let sessions = $state.raw<SessionInfo[]>([]);
  const listening = $derived(
    !!options.forward() &&
      options
        .ids()
        .some((sessionId) =>
          options
            .info()
            [sessionId]?.listeningPorts.some((entry) =>
              options.forward()?.isPromptedPort(entry.port),
            ),
        ),
  );
  // The session-list connection carries the forward list and live session
  // updates for panes on the chat view (whose terminal stream is detached).
  // Otherwise this page opens it only while a pane listens on a port or the
  // forwarding dialog is open.
  const onChatSide = $derived(
    options
      .ids()
      .some(
        (sessionId) =>
          options.chatEnabled() &&
          options.info()[sessionId]?.views.includes("chat") &&
          options.info()[sessionId]?.view !== "terminal",
      ),
  );
  const byId = $derived(
    new SvelteMap(sessions.map((session) => [session.id, session])),
  );
  $effect(() => {
    if (!listening && !options.forwarding() && !onChatSide) return;
    const socket = connectSessions(
      (event) => {
        sessions = applySessionEvent(sessions, event);
      },
      () => {},
      options.error,
      options.forward()?.observeSessionEvent,
    );
    return () => socket.close();
  });

  return {
    get sessions() {
      return sessions;
    },
    get byId() {
      return byId;
    },
  };
}
