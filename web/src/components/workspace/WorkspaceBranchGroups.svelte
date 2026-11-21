<script lang="ts">
  import type {
    Repository,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
  import type {
    ArchiveActions,
    ArchivePlacement,
  } from "../../domains/sessions/archived";
  import { setPinned } from "../../domains/workspace/home-worktrees";
  import { homeWorktrees } from "../../domains/workspace/home-worktrees.svelte";
  import type { BranchOperation } from "../../domains/workspace/branch-actions";
  import {
    deriveRepositoryExpanded,
    hasVisibleArchivedInRepository,
    isRepositoryGroupOpen,
  } from "../../hooks/sessions/session-list";
  import { openGitWorkbench } from "../../hooks/shared/shell.svelte";
  import { featureAvailable } from "../../platform/feature-host.svelte";
  import DirectSessions from "../sessions/DirectSessions.svelte";
  import RepositoryGroup from "./RepositoryGroup.svelte";

  let {
    repositories,
    sessions,
    list,
    direct,
    visible,
    visibleIds,
    visibleArchiveIds,
    placement,
    selectedRepository,
    selectedWorktree,
    filtering,
    compact,
    now,
    archive,
    worktreeDetailsOpen = $bindable(false),
    onstart,
    onremove,
    onswitch,
    onclose,
    onoperation,
  }: {
    repositories: readonly Repository[];
    sessions: readonly SessionInfo[];
    list: readonly SessionInfo[];
    direct: SessionInfo[];
    visible: readonly SessionInfo[];
    visibleIds: Set<string> | undefined;
    visibleArchiveIds: Set<string> | undefined;
    placement: ArchivePlacement;
    selectedRepository: string;
    selectedWorktree: Worktree | undefined;
    filtering: boolean;
    compact: boolean;
    now: number;
    archive: ArchiveActions;
    worktreeDetailsOpen?: boolean;
    onstart: (
      target:
        | { kind: "new-worktree"; repositoryId: string }
        | { kind: "worktree"; repositoryId: string; worktreeId: string },
    ) => void;
    onremove: (tree: Worktree) => void;
    onswitch: (tree: Worktree) => void;
    onclose: (session: SessionInfo) => void;
    onoperation: (repositoryId: string, operation: BranchOperation) => void;
  } = $props();

  let focusedExpanded = $state<Record<string, boolean>>({});
  let expansionOverrides = $state<Record<string, boolean>>({});
  let filterExpanded = $state<Record<string, boolean>>({});

  $effect(() => {
    void filtering;
    void visible;
    void selectedRepository;
    void selectedWorktree;
    filterExpanded = {};
  });

  const expanded = $derived(
    deriveRepositoryExpanded(
      repositories,
      expansionOverrides,
      sessions,
      placement,
    ),
  );

  function archivedInRepository(id: string) {
    return hasVisibleArchivedInRepository(placement, id, visibleArchiveIds);
  }

  function pin(repositoryId: string, ids: string[], pinned: boolean) {
    const repo = repositories.find((r) => r.id === repositoryId);
    if (repo)
      homeWorktrees.save(setPinned(homeWorktrees.value, repo, ids, pinned));
  }
</script>

{#each repositories as repo (repo.id)}
  <RepositoryGroup
    {repo}
    sessions={list.filter((s) => s.workspace?.repositoryId === repo.id)}
    {filtering}
    {compact}
    focused={selectedRepository === repo.id}
    worktree={selectedRepository === repo.id ? selectedWorktree?.id : undefined}
    hidden={selectedRepository !== "all" && selectedRepository !== repo.id}
    {visibleIds}
    open={isRepositoryGroupOpen({
      repoId: repo.id,
      filtering,
      visible,
      hasArchived: archivedInRepository(repo.id),
      filterExpanded,
      selectedRepository,
      focusedExpanded,
      expanded,
    })}
    onopenchange={(open) => {
      if (filtering) filterExpanded[repo.id] = open;
      else if (selectedRepository === repo.id) focusedExpanded[repo.id] = open;
      else expansionOverrides[repo.id] = open;
    }}
    {now}
    onstart={(worktreeId) =>
      worktreeId === undefined
        ? onstart({ kind: "new-worktree", repositoryId: repo.id })
        : onstart({ kind: "worktree", repositoryId: repo.id, worktreeId })}
    ontogglepin={(worktreeId, pinned) => pin(repo.id, [worktreeId], pinned)}
    {onremove}
    {onswitch}
    {onclose}
    archived={placement.repositories.get(repo.id)}
    {visibleArchiveIds}
    {archive}
    detailsOpen={worktreeDetailsOpen}
    ondetailschange={(open) => (worktreeDetailsOpen = open)}
    onoperation={(operation) => onoperation(repo.id, operation)}
    ongit={featureAvailable("workspace.git-workbench")
      ? (tree, view) =>
          openGitWorkbench({
            repositoryId: repo.id,
            worktreeId: tree.id,
            view,
          })
      : undefined}
  />
{/each}
{#if direct.length || placement.direct.length}
  <DirectSessions
    sessions={direct}
    {compact}
    focused={selectedRepository === "direct"}
    {visibleIds}
    {now}
    {onclose}
    archived={placement.direct}
    visibleArchiveIds={selectedRepository === "all" ||
    selectedRepository === "direct"
      ? visibleArchiveIds
      : new Set()}
    {archive}
  />
{/if}
