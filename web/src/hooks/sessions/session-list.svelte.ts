import { SvelteSet } from "svelte/reactivity";
import type { SessionInfo } from "../../contracts/schemas";
import { sessionPins } from "../../domains/sessions/session-pins.svelte";
import { applySessionEvent } from "../../domains/sessions/session-events";
import type { TerminalDrafts } from "../../domains/terminal/terminal-draft.svelte";
import { terminalDraftRetained } from "../../domains/terminal/terminal-draft-owners";
import type { Connection } from "../../platform/ws";
import { connectSessions, type SessionEvent } from "./session-feed";

let cachedSessions: SessionInfo[] | undefined;
export function sessionListSnapshot() {
  return cachedSessions;
}
export function clearSessionListSnapshot() {
  cachedSessions = undefined;
}

/** The core feed has no repository, archive, attention or forwarding owner. */
export interface SessionList {
  readonly sessions: readonly SessionInfo[];
  readonly loading: boolean;
  /** A live snapshot, not merely cached rows, has established membership. */
  readonly ready: boolean;
  readonly connection: Connection;
  readonly error: string;
  /** Every removed event, including archive commits after the live row left. */
  subscribeRemoved(listener: (id: string) => void): () => void;
  close(): void;
}

export function createSessionList({
  drafts,
  onreconnect,
  onEvent,
}: {
  drafts: TerminalDrafts;
  onreconnect?: () => void;
  onEvent?: (event: SessionEvent, previous: readonly SessionInfo[]) => void;
}): SessionList {
  let sessions = $state.raw<SessionInfo[]>(cachedSessions ?? []);
  let loading = $state(!cachedSessions);
  let ready = $state(false);
  let connection = $state<Connection>("connecting");
  let error = $state("");
  const removedListeners = new SvelteSet<(id: string) => void>();
  const socket = connectSessions(
    (event) => {
      const previous = sessions;
      sessions = applySessionEvent(sessions, event);
      cachedSessions = sessions;
      onEvent?.(event, previous);
      if (event.type === "snapshot") {
        sessionPins.refreshMissing(event.sessions);
        loading = false;
        ready = true;
      } else if (event.type === "removed") {
        for (const listener of removedListeners) listener(event.id);
        sessionPins.refreshEnded(event.id);
        const removed = previous.find((session) => session.id === event.id);
        if (
          removed &&
          !removed.views.includes("chat") &&
          !terminalDraftRetained(drafts, event.id)
        )
          drafts.remove(event.id);
      } else if (
        event.type !== "forwards" &&
        (event.session.status === "exited" || event.session.state !== "active")
      ) {
        sessionPins.refreshEnded(event.session.id);
      }
    },
    (state) => {
      const reconnected =
        state === "connected" && connection === "reconnecting";
      connection = state;
      if (reconnected) onreconnect?.();
    },
    (message) => {
      error = message;
    },
  );
  return {
    get sessions() {
      return sessions;
    },
    get loading() {
      return loading;
    },
    get ready() {
      return ready;
    },
    get connection() {
      return connection;
    },
    get error() {
      return error;
    },
    subscribeRemoved(listener) {
      removedListeners.add(listener);
      return () => {
        removedListeners.delete(listener);
      };
    },
    close() {
      socket.close();
      removedListeners.clear();
    },
  };
}
