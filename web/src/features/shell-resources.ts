import {
  createSessionList,
  type SessionList,
} from "../hooks/sessions/session-list.svelte";
import {
  createServerSetup,
  type ServerSetup,
} from "../hooks/shared/server-setup.svelte";
import type { Repositories } from "../hooks/workspace/repositories.svelte";
import type { Archived } from "../hooks/sessions/archived.svelte";
import type { LaunchConfigs } from "../hooks/settings/launch-configs.svelte";
import type { TerminalDrafts } from "../domains/terminal/terminal-draft.svelte";
import { retainedTerminalDraftIds } from "../domains/terminal/terminal-draft-owners";
import { featureContributions } from "./registry.svelte";
import { requestFeatureHostRefresh } from "../platform/feature-host.svelte";

export interface ShellSessionResources {
  sessions: SessionList;
  setup: ServerSetup;
  repositories?: Repositories;
  archived?: Archived;
  launchConfigs?: LaunchConfigs;
  refreshWorktrees(): void;
  dispose(): void;
}

/** Compose enabled projections around the one core session feed. */
export function createShellSessionResources(
  drafts: TerminalDrafts,
  currentSession: () => string | undefined,
): ShellSessionResources {
  const entries = featureContributions.entries;
  const archived = entries.chat?.createArchived();
  const repositories = entries.workspace?.createRepositories();
  const launchConfigs = entries.launch?.createLaunchConfigs();
  const setup = createServerSetup();
  const controller = new AbortController();
  function reconcile(ids: string[]) {
    if (controller.signal.aborted) return;
    const primary = currentSession();
    if (primary) ids.push(primary);
    for (const id of retainedTerminalDraftIds(drafts)) ids.push(id);
    drafts.reconcile(ids);
  }
  const sessions = createSessionList({
    drafts,
    onreconnect: () => {
      void requestFeatureHostRefresh().then(() => {
        if (!controller.signal.aborted && repositories)
          void repositories.refresh();
      });
    },
    onEvent: (event, previous) => {
      entries.notifications?.observe(event);
      entries.forward?.observeSessionEvent(event);
      // Without the history projection, do not guess which saved drafts still
      // belong to inactive archived sessions from an earlier configuration.
      if (!archived) return;
      if (event.type === "snapshot") {
        const ids = event.sessions.map((session) => session.id);
        void archived.refresh().then((records) => {
          if (records) reconcile([...ids, ...records]);
        });
      } else if (event.type === "removed") {
        const removed = previous.find((session) => session.id === event.id);
        // A removed event for an ID already absent from the active list is a
        // canonical archive commit or remote record removal from NotifyRemoved.
        if (!removed || removed.views.includes("chat")) void archived.refresh();
      } else if (
        event.type !== "forwards" &&
        event.session.views.includes("chat") &&
        (event.type === "created" || event.session.state !== "active")
      ) {
        void archived.refresh();
      }
    },
  });
  void setup.load(controller.signal);
  if (launchConfigs) void launchConfigs.load(true);
  const onVisible = () => {
    if (!document.hidden && !controller.signal.aborted && repositories) {
      void repositories.refresh();
    }
  };
  if (repositories) {
    void repositories.refresh();
    if (typeof document !== "undefined" && typeof window !== "undefined") {
      document.addEventListener("visibilitychange", onVisible);
      window.addEventListener("pageshow", onVisible);
    }
  }
  return {
    sessions,
    setup,
    repositories,
    archived,
    launchConfigs,
    refreshWorktrees() {
      if (repositories) void repositories.refresh();
      if (archived)
        void archived.refresh().then((records) => {
          if (records)
            reconcile([
              ...sessions.sessions.map((session) => session.id),
              ...records,
            ]);
        });
    },
    dispose() {
      if (
        repositories &&
        typeof document !== "undefined" &&
        typeof window !== "undefined"
      ) {
        document.removeEventListener("visibilitychange", onVisible);
        window.removeEventListener("pageshow", onVisible);
      }
      sessions.close();
      entries.forward?.disconnectSessionFeed();
      controller.abort();
      archived?.dispose();
      repositories?.dispose();
      launchConfigs?.dispose();
    },
  };
}
