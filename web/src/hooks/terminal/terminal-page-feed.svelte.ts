import { SvelteMap, SvelteSet } from "svelte/reactivity";
import type { SessionInfo } from "../../contracts/schemas";
import { applySessionEvent } from "../../domains/sessions/session-events";
import { connectSessions, type SessionEvent } from "../sessions/session-feed";
import type { SessionList } from "../sessions/session-list.svelte";

interface TerminalPageFeedOptions {
  ids: () => string[];
  source?: SessionList;
  forward: () =>
    | { observeSessionEvent: (event: SessionEvent) => void }
    | undefined;
  error: (message: string) => void;
}

/** Core membership and removal observation also runs without any plugins. */
export function useTerminalPageFeed(options: TerminalPageFeedOptions) {
  let standaloneSessions = $state.raw<SessionInfo[]>([]);
  let standaloneReady = $state(false);
  const removedListeners = new SvelteSet<(id: string) => void>();
  const standaloneFeed = $derived(!options.source && options.ids().length > 0);
  const sessions = $derived(options.source?.sessions ?? standaloneSessions);
  const byId = $derived(
    new SvelteMap(sessions.map((session) => [session.id, session])),
  );
  // The shell owns its feed. A standalone page keeps one for every owner,
  // including recovery before chat attaches, so final removal has no gap.
  $effect(() => {
    if (!standaloneFeed) return;
    const socket = connectSessions(
      (event) => {
        if (event.type === "snapshot") standaloneReady = true;
        if (event.type === "removed")
          for (const listener of removedListeners) listener(event.id);
        standaloneSessions = applySessionEvent(standaloneSessions, event);
      },
      () => {},
      options.error,
      (event) => options.forward()?.observeSessionEvent(event),
    );
    return () => socket.close();
  });

  return {
    get sessions() {
      return sessions;
    },
    get ready() {
      return options.source?.ready ?? standaloneReady;
    },
    get byId() {
      return byId;
    },
    subscribeRemoved(listener: (id: string) => void) {
      if (options.source) return options.source.subscribeRemoved(listener);
      removedListeners.add(listener);
      return () => {
        removedListeners.delete(listener);
      };
    },
  };
}
