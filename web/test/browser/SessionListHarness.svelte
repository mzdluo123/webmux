<script lang="ts">
  import { createArchiveSignal } from "../../src/domains/sessions/archive-signal";
  import { createTerminalDrafts } from "../../src/domains/terminal/terminal-draft.svelte";
  import type { Archived } from "../../src/hooks/sessions/archived.svelte";
  import type { SessionList as SessionListFeed } from "../../src/hooks/sessions/session-list.svelte";
  import { createServerSetup } from "../../src/hooks/shared/server-setup.svelte";
  import { setShell, type Shell } from "../../src/hooks/shared/shell.svelte";
  import type {
    ArchivedSession,
    SessionInfo,
  } from "../../src/contracts/schemas";
  import SessionList from "../../src/pages/SessionList.svelte";

  let {
    sessions = [],
    loading = false,
    error = "",
    records = [],
  }: {
    sessions?: readonly SessionInfo[];
    loading?: boolean;
    error?: string;
    records?: readonly ArchivedSession[];
  } = $props();

  const drafts = createTerminalDrafts();
  const archiveSignal = createArchiveSignal();
  const setup = createServerSetup();

  const feed: SessionListFeed = {
    get sessions() {
      return sessions;
    },
    get loading() {
      return loading;
    },
    get ready() {
      return !loading;
    },
    connection: "connected",
    get error() {
      return error;
    },
    subscribeRemoved: () => () => {},
    close: () => {},
  };

  const archived: Archived = {
    get records() {
      return records;
    },
    initialSettled: true,
    refresh: async () => records.map((item) => item.id),
    dispose: () => {},
  };

  const shell: Shell = {
    drafts,
    setActivePane: () => {},
    sessions: feed,
    archived,
    archiveSignal,
    setup,
    start: () => {},
    rename: () => {},
    close: () => {},
    removeWorktree: () => {},
    chooseHomeWorktrees: () => {},
    repositoryDetails: () => {},
    worktreeDetails: () => {},
    switchBranch: () => {},
    releaseWorktree: () => {},
    branchOperation: () => {},
    forward: () => {},
    addRepository: () => {},
    settingsRepository: undefined,
    openRepositorySettings: () => {},
  };
  setShell(shell);
</script>

<SessionList />
