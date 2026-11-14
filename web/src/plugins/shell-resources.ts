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
import { pluginContributions } from "./registry.svelte";
import { requestPluginHostRefresh } from "../platform/plugin-host.svelte";

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
): ShellSessionResources {
  const entries = pluginContributions.entries;
  const archived = entries.chat?.createArchived();
  const repositories = entries.workspace?.createRepositories();
  const launchConfigs = entries.launch?.createLaunchConfigs();
  const setup = createServerSetup();
  const controller = new AbortController();
  const sessions = createSessionList({
    drafts,
    onreconnect: () => {
      void requestPluginHostRefresh().then(() => {
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
          if (records) drafts.reconcile([...ids, ...records]);
        });
      } else if (event.type === "removed") {
        if (
          previous
            .find((session) => session.id === event.id)
            ?.views.includes("chat")
        )
          void archived.refresh();
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
  if (repositories) void repositories.refresh();
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
            drafts.reconcile([
              ...sessions.sessions.map((session) => session.id),
              ...records,
            ]);
        });
    },
    dispose() {
      sessions.close();
      entries.forward?.disconnectSessionFeed();
      controller.abort();
      archived?.dispose();
      repositories?.dispose();
      launchConfigs?.dispose();
    },
  };
}
