<script lang="ts">
  import SessionListRemovalDialog from "../components/sessions/SessionListRemovalDialog.svelte";
  import WorkbenchSessionQueue from "../components/workspace/WorkbenchSessionQueue.svelte";
  import { useLayoutRowsMotion } from "../hooks/shared/layout-rows-motion.svelte";
  import WorkbenchPreviewControl from "../components/workspace/WorkbenchPreviewControl.svelte";
  import { workbenchLayout } from "../domains/workspace/workbench-layout.svelte";
  import { queueGroup } from "../domains/workspace/session-queue";
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import WorkspaceHeaderActions from "../components/workspace/WorkspaceHeaderActions.svelte";
  import WorkspaceToolbar from "../components/workspace/WorkspaceToolbar.svelte";
  import WorkspaceBranchGroups from "../components/workspace/WorkspaceBranchGroups.svelte";
  import { getFeatureContributions } from "../features/context";
  import type { ArchivedSession, Worktree } from "../contracts/schemas";
  import { errorMessage } from "../platform/error";
  import HomeUpdateNotice from "../components/shared/HomeUpdateNotice.svelte";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import SessionFilters from "../components/sessions/SessionFilters.svelte";
  import SessionListNotices from "../components/sessions/SessionListNotices.svelte";
  import WorkspaceMobileHeader from "../components/workspace/WorkspaceMobileHeader.svelte";
  import WorktreeBadges from "../components/workspace/WorktreeBadges.svelte";
  import { removeRecord, resumeSession } from "../features/history";
  import {
    placeArchived,
    type ArchiveActions,
  } from "../domains/sessions/archived";
  import SessionListEmpty from "../components/sessions/SessionListEmpty.svelte";
  import { homeWorktrees } from "../domains/workspace/home-worktrees.svelte";
  import { worktreePins } from "../domains/workspace/worktree-pins.svelte";
  import {
    filterDirectSessions,
    filterVisibleArchiveIds,
    filterVisibleSessions,
    isSelectedWorktreePinned,
    isSessionListEmpty,
    mountSessionListPolling,
    reconcileSelectedRepository,
    removeRecordAfterWorktree,
    resolveScopeLabel,
    resolveScopedArchive,
    resolveScopedSessions,
    resolveSelectedRepository,
  } from "../hooks/sessions/session-list";
  import { type StatusFilter } from "../domains/sessions/session-status";
  import {
    resetBrowserChrome,
    setBrowserChrome,
  } from "../platform/browser-chrome";
  import { getShell } from "../hooks/shared/shell.svelte";
  import { useViewport } from "../hooks/shared/viewport.svelte";
  import { homeSessionView } from "../domains/sessions/home-session-view.svelte";
  import {
    featureEnabled,
    featureAvailable,
  } from "../platform/feature-host.svelte";

  // The shell owns the live session, repository and archive state.
  const shell = getShell();
  const { drafts } = shell;
  const sessions = $derived(shell.sessions.sessions);
  const recoverable = $derived(shell.archived?.records ?? []);
  const repositories = $derived(shell.repositories?.list ?? []);
  const repositoriesSettled = $derived(shell.repositories?.settled ?? false);
  const refreshing = $derived(shell.repositories?.refreshing ?? false);
  const repositoryError = $derived(
    shell.repositories?.error ?? m.s1_workspace_unavailable(),
  );
  const setup = $derived(shell.setup.value);
  const capabilityError = $derived(setup.capabilityError);
  const loading = $derived(shell.sessions.loading);
  const initializing = $derived(!shell.setup.settled);
  const error = $derived(shell.sessions.error);
  let removing = $state<Worktree | null>(null);
  let removingRecord = $state<ArchivedSession | null>(null);
  let recordError = $state("");
  let now = $state(Date.now());
  let query = $state("");
  let statusFilter = $state<StatusFilter>("all");
  const repositoryChoice = $derived(shell.repositories?.choice ?? null);
  const compact = $derived(homeSessionView.compact);
  const queue = $derived(workbenchLayout.value === "queue");
  let layoutRows = $state<HTMLDivElement | null>(null);
  useLayoutRowsMotion(
    () => layoutRows,
    () => workbenchLayout.value,
    "[data-session-id]",
  );
  const viewport = useViewport();
  const attentionOnly = $derived(queue && workbenchLayout.attentionOnly);
  const QuickStartComposer = $derived(
    featureEnabled("chat")
      ? getFeatureContributions().creation.QuickStartComposer
      : undefined,
  );
  const showQuickStart = $derived(
    queue &&
      workbenchLayout.queueView === "composer" &&
      !attentionOnly &&
      Boolean(QuickStartComposer) &&
      featureAvailable("workspace.manage") &&
      Boolean(shell.repositories),
  );
  let searchInput = $state<HTMLInputElement | null>(null);
  const list = $derived(
    sessions.filter((session) => session.state === "active"),
  );
  const direct = $derived(filterDirectSessions(list, repositories));
  const placement = $derived(placeArchived(recoverable, repositories));
  const selectedRepository = $derived(
    resolveSelectedRepository(repositoryChoice, repositories),
  );
  // A worktree picked in the tree narrows the repository view to it.
  const selectedWorktree = $derived(
    repositories
      .find((repo) => repo.id === selectedRepository)
      ?.worktrees.find((tree) => tree.id === shell.repositories?.worktree),
  );
  const scopeLabel = $derived(
    resolveScopeLabel(selectedRepository, selectedWorktree, repositories),
  );
  const scopedSessions = $derived(
    resolveScopedSessions(selectedRepository, selectedWorktree, list, direct),
  );
  const scopedArchive = $derived(
    resolveScopedArchive(
      selectedRepository,
      selectedWorktree,
      recoverable,
      placement,
    ),
  );
  const scopedArchiveCount = $derived(scopedArchive.length);
  $effect(() => {
    reconcileSelectedRepository({
      loading,
      initializing,
      repositoriesSettled,
      repositoryError,
      repositoryChoice,
      selectedRepository,
      selectedWorktreeId: shell.repositories?.worktree ?? "",
      repositories,
      list,
      select: (id) =>
        shell.repositories?.select(queue && !repositoryChoice ? "all" : id),
    });
  });
  const search = $derived(query.trim().toLocaleLowerCase());
  const filtering = $derived(Boolean(search) || statusFilter !== "all");
  const visible = $derived(
    filterVisibleSessions(scopedSessions, repositories, statusFilter, search),
  );
  const visibleIds = $derived(
    filtering || selectedRepository !== "all"
      ? new Set(visible.map((session) => session.id))
      : undefined,
  );
  const visibleArchiveIds = $derived(
    filterVisibleArchiveIds(
      recoverable,
      repositories,
      filtering,
      statusFilter,
      search,
    ),
  );
  const archivedMatches = $derived(
    attentionOnly || visibleArchiveIds === undefined
      ? 0
      : scopedArchive.filter((record) => visibleArchiveIds.has(record.id))
          .length,
  );
  let rebuildFailure = $state("");
  const archive: ArchiveActions = $derived({
    now,
    filtering,
    resume: resumeSession,
    remove: async (id) => {
      recordError = "";
      await removeRecord(id);
      drafts.remove(id);
      await shell.archived?.refresh();
    },
    removeRecordAndWorktree: (record, tree) => {
      recordError = "";
      removingRecord = record;
      removing = tree;
    },
    release: shell.releaseWorktree,
    onresumed: () => ((rebuildFailure = ""), void shell.archived?.refresh()),
    onrebuilt: (failure) => ((rebuildFailure = failure), refreshWorktrees()),
  });
  const running = $derived(
    sessions.some((session) => session.status === "running"),
  );
  $effect(() => setBrowserChrome(m.s1_chrome_workspace(), running));
  $effect(() => {
    if (workbenchLayout.attentionOnly) clearFilters(false);
  });
  function clearFilters(focus = true) {
    query = "";
    statusFilter = "all";
    if (focus) searchInput?.focus();
  }
  let worktreeDetailsOpen = $state(false);
  $effect(() => {
    void selectedWorktree?.id;
    worktreeDetailsOpen = false;
  });
  const selectedPinned = $derived(
    isSelectedWorktreePinned(
      homeWorktrees.value,
      repositories,
      selectedRepository,
      selectedWorktree,
    ),
  );
  async function removeAfterWorktree(record: ArchivedSession | null) {
    await removeRecordAfterWorktree(
      record,
      removeRecord,
      (id) => drafts.remove(id),
      async () => shell.archived?.refresh(),
      (msg) => (recordError = msg),
      () => (removingRecord = null),
      errorMessage,
    );
  }
  function refreshWorktrees() {
    void shell.repositories?.refresh();
    if (shell.archived)
      void shell.archived.refresh().then((records) => {
        if (records)
          drafts.reconcile([
            ...sessions.map((session) => session.id),
            ...records,
          ]);
      });
  }
  const closeRemoveWorktree = () => {
    removing = null;
    removingRecord = null;
  };
  onMount(() =>
    mountSessionListPolling(
      () => void shell.repositories?.refresh(),
      (nextNow) => (now = nextNow),
      resetBrowserChrome,
    ),
  );
</script>

{#snippet actions()}
  <WorkspaceHeaderActions {refreshing} {selectedWorktree} />
{/snippet}

{#if showQuickStart && QuickStartComposer && !loading && !initializing}
  <div
    class="workspace-page safe-area-page flex h-(--viewport-height) flex-col overflow-hidden"
    style:--viewport-height={`${viewport.height}px`}
  >
    <WorkspaceMobileHeader
      reconnecting={shell.sessions.connection === "reconnecting"}
    />
    <HomeUpdateNotice update={setup.pendingUpdate} inset />
    <QuickStartComposer />
  </div>
{:else}
  <div class="workspace-page safe-area-page min-h-dvh">
    <WorkspaceMobileHeader
      reconnecting={shell.sessions.connection === "reconnecting"}
    />
    <HomeUpdateNotice update={setup.pendingUpdate} inset />
    {#if loading || initializing}
      <div
        class="mx-auto max-w-6xl space-y-4 px-4 py-5 sm:px-6 lg:px-8"
        aria-label={m.s1_loading_sessions_aria()}
      >
        {#each [0, 1, 2] as item (item)}<Skeleton class="h-40 w-full" />{/each}
      </div>
    {:else}
      <div class="mx-auto max-w-6xl px-4 pt-4 pb-8 sm:px-6 sm:pt-8 lg:px-8">
        <main class="min-w-0">
          <WorkspaceToolbar
            title={attentionOnly
              ? m.s8_queue_attention()
              : selectedRepository === "all"
                ? m.s1_all_repositories()
                : scopeLabel}
            mainWorktree={selectedWorktree?.kind === "main"}
            pinnedWorktree={Boolean(
              selectedWorktree &&
              worktreePins.has(
                selectedWorktree.repositoryId,
                selectedWorktree.id,
              ),
            )}
            {loading}
            {filtering}
            visibleCount={attentionOnly
              ? visible.filter((session) => queueGroup(session) === "attention")
                  .length
              : visible.length}
            sessionCount={scopedSessions.length}
            repositoryCount={selectedRepository === "all"
              ? repositories.length
              : selectedRepository === "direct"
                ? 0
                : 1}
            {refreshing}
            {actions}
            onstart={() =>
              selectedWorktree
                ? shell.start({
                    kind: "worktree",
                    repositoryId: selectedRepository,
                    worktreeId: selectedWorktree.id,
                  })
                : shell.start()}
            startDisabled={Boolean(
              selectedWorktree &&
              (!selectedWorktree.available || selectedWorktree.kind === "bare"),
            )}
          >
            {#snippet meta()}
              {#if selectedWorktree}<WorktreeBadges
                  worktree={selectedWorktree}
                  pinned={selectedPinned}
                />{/if}
            {/snippet}
            <SessionFilters
              sessions={scopedSessions}
              archivedCount={scopedArchiveCount}
              bind:query
              bind:searchInput
              bind:statusFilter
              filtered={filtering &&
                (visible.length > 0 || archivedMatches > 0)}
              shortcutDisabled={Boolean(removing)}
              onclear={clearFilters}
            />
            {#if !queue}<WorkbenchPreviewControl />{/if}
          </WorkspaceToolbar>
          <SessionListNotices
            setupError={shell.setup.error}
            setupBusy={shell.setup.busy}
            onretrysetup={() => void shell.setup.load()}
            {error}
            {rebuildFailure}
            recordError={recordError ||
              (featureEnabled("chat") && !shell.archived
                ? m.s1_chat_records_unavailable_notice()
                : "")}
            {capabilityError}
            {repositoryError}
            hasRepositories={repositories.length > 0}
          />
          <div class="space-y-8" bind:this={layoutRows}>
            {#if queue}
              <WorkbenchSessionQueue
                sessions={visible}
                available={scopedSessions}
                {repositories}
                records={attentionOnly
                  ? []
                  : scopedArchive.filter(
                      (record) =>
                        !visibleArchiveIds || visibleArchiveIds.has(record.id),
                    )}
                {archive}
                {attentionOnly}
                {filtering}
                {now}
                onclear={clearFilters}
                onrename={shell.rename}
                onclose={shell.close}
              />
            {:else}
              <WorkspaceBranchGroups
                {repositories}
                {sessions}
                {list}
                {direct}
                {visible}
                {visibleIds}
                {visibleArchiveIds}
                {placement}
                {selectedRepository}
                {selectedWorktree}
                {filtering}
                {compact}
                {now}
                {archive}
                bind:worktreeDetailsOpen
                onstart={shell.start}
                onremove={(tree) => (removing = tree)}
                onswitch={(tree) => shell.switchBranch(tree)}
                onclose={shell.close}
                onoperation={shell.branchOperation}
              />
            {/if}
            {#if !attentionOnly && (queue ? !visible.length && !(filtering ? archivedMatches : scopedArchiveCount) : isSessionListEmpty( { filtering, visibleCount: visible.length, archivedMatches, repositoriesCount: repositories.length, listCount: list.length, recoverableCount: recoverable.length, selectedRepository, directCount: direct.length, directArchiveCount: placement.direct.length } ))}<SessionListEmpty
                filtered={filtering && !visible.length && !archivedMatches}
                onclear={clearFilters}
                onexpand={selectedRepository === "all"
                  ? undefined
                  : () => {
                      shell.repositories?.select("all");
                      searchInput?.focus();
                    }}
              />{/if}
          </div>
        </main>
      </div>
    {/if}
  </div>
{/if}
{#if removing}
  {@const target = removing}
  {@const targetRecord = removingRecord}
  <SessionListRemovalDialog
    worktree={target}
    record={targetRecord}
    onremoved={() => {
      worktreePins.remove(target.repositoryId, [target.id]);
      void removeAfterWorktree(targetRecord);
    }}
    onchange={refreshWorktrees}
    onclose={closeRemoveWorktree}
  />
{/if}
