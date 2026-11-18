<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import type { BranchOperation } from "../../domains/workspace/branch-actions";
  import type {
    ArchivedSession,
    Repository,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
  import {
    EMPTY_ARCHIVE,
    repositoryRecords,
    type ArchiveActions,
    type RepositoryArchive,
  } from "../../domains/sessions/archived";
  import { homeWorktrees } from "../../domains/workspace/home-worktrees.svelte";
  import {
    homeLayout,
    worktreeTitle,
  } from "../../domains/workspace/home-worktrees";
  import { switchableWorktree } from "../../domains/workspace/worktree-sync";
  import WorkspaceGroup from "./WorkspaceGroup.svelte";

  let {
    repo,
    sessions,
    visibleIds,
    filtering,
    hidden = false,
    focused = false,
    worktree,
    compact = false,
    open,
    onopenchange,
    now,
    onstart,
    ontogglepin,
    onremove,
    onswitch,
    onclose,
    archived = EMPTY_ARCHIVE,
    visibleArchiveIds,
    archive,
    detailsOpen,
    ondetailschange,
    onoperation,
    ongit,
  }: {
    repo: Repository;
    // All sessions associated with this repository.
    sessions: SessionInfo[];
    visibleIds?: ReadonlySet<string>;
    filtering: boolean;
    hidden?: boolean;
    focused?: boolean;
    /** Shows only this worktree's group (picked in the workbench tree). */
    worktree?: string;
    compact?: boolean;
    open: boolean;
    onopenchange: (open: boolean) => void;
    now: number;
    // Without a worktree id the sheet creates a new worktree.
    onstart: (worktreeId?: string) => void;
    ontogglepin: (worktreeId: string, pinned: boolean) => void;
    onremove: (worktree: Worktree) => void;
    /** Opens the switch of a worktree's branch. */
    onswitch: (worktree: Worktree) => void;
    onclose: (session: SessionInfo) => void;
    // Every archived record placed in this repository.
    archived?: RepositoryArchive;
    // Archived records the filters keep; undefined while not filtering.
    visibleArchiveIds?: ReadonlySet<string>;
    archive?: ArchiveActions;
    /** Details dialog of the focused worktree, opened from the page header. */
    detailsOpen?: boolean;
    ondetailschange?: (open: boolean) => void;
    /** Starts a sync action of a worktree's branch. */
    onoperation?: (operation: BranchOperation) => void;
    ongit?: (worktree: Worktree, view?: "commit") => void;
  } = $props();

  let panel: HTMLDivElement | null = null;
  const motion = Collapsible.useManualDisclosureMotion(
    () => panel,
    () => open,
  );
  const showContent = $derived(open || motion.moving);
  let contentReady = $state(false);
  $effect(() => {
    if (open) contentReady = true;
  });

  const visible = $derived(
    visibleIds
      ? sessions.filter((session) => visibleIds.has(session.id))
      : sessions,
  );
  const missing = $derived(
    sessions.filter(
      (s) => !repo.worktrees.some((w) => w.id === s.workspace?.worktreeId),
    ),
  );
  // Archived records of a group that the filters keep.
  function shownArchive(records: readonly ArchivedSession[] = []) {
    return visibleArchiveIds
      ? records.filter((record) => visibleArchiveIds.has(record.id))
      : [...records];
  }
  const archivedVisible = $derived(
    shownArchive(repositoryRecords(archived)).length > 0,
  );
  const renderLayout = $derived(
    homeLayout(homeWorktrees.value, repo, sessions, archived.worktrees),
  );
  const layout = $derived(
    visibleIds
      ? homeLayout(homeWorktrees.value, repo, visible, archived.worktrees)
      : renderLayout,
  );
</script>

<Collapsible.Root
  class="min-w-0"
  data-repository-id={repo.id}
  hidden={hidden || (filtering && !visible.length && !archivedVisible)}
  role="region"
  aria-label={m.s8_repo_group_aria({ name: repo.name })}
  {open}
  onOpenChange={(value) => motion.toggle(value, onopenchange)}
>
  <!-- The focused repository is the page itself, so its name is not repeated. -->
  {#if !focused || repo.error}
    <div
      class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-3"
    >
      {#if !focused}
        <h2 class="min-w-0 sm:flex-1">
          <Collapsible.Trigger
            class="group flex min-w-0 max-w-full items-center gap-2 text-left font-semibold outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <IconChevronDown
              class="size-4 shrink-0 -rotate-90 motion-safe:transition-transform group-data-[state=open]:rotate-0"
              aria-hidden="true"
            />
            <span class="min-w-0 break-all">{repo.name}</span>
          </Collapsible.Trigger>
        </h2>
      {/if}
      {#if repo.error}<p
          role="alert"
          class="w-full text-sm break-all text-destructive"
        >
          {repo.error}
        </p>{/if}
    </div>
  {/if}
  <div bind:this={panel} inert={!open} class:overflow-clip={motion.moving}>
    {#if contentReady}<Collapsible.Content
        forceMount
        hidden={!showContent}
        class={["space-y-0", (!focused || repo.error) && "pt-4"]}
      >
        {#each renderLayout.shown as { tree, pinned }, index (tree.id)}
          {@const treeArchive = shownArchive(archived.worktrees.get(tree.id))}
          <WorkspaceGroup
            title={worktreeTitle(tree)}
            path={tree.path}
            worktree={tree}
            lastFetchedAt={repo.lastFetchedAt}
            sessions={sessions.filter(
              (s) => s.workspace?.worktreeId === tree.id,
            )}
            {visibleIds}
            {compact}
            separated={index > 0 && !worktree}
            focused={!!worktree && worktree === tree.id}
            detailsOpen={worktree === tree.id ? detailsOpen : undefined}
            ondetailschange={worktree === tree.id ? ondetailschange : undefined}
            hidden={hidden ||
              !showContent ||
              (!!worktree && worktree !== tree.id) ||
              (filtering &&
                !treeArchive.length &&
                !visible.some(
                  (session) => session.workspace?.worktreeId === tree.id,
                ))}
            {now}
            {pinned}
            ontogglepin={() => ontogglepin(tree.id, !pinned)}
            {onclose}
            onstart={() => onstart(tree.id)}
            onremove={tree.kind === "linked" ? () => onremove(tree) : undefined}
            onswitch={switchableWorktree(tree)
              ? () => onswitch(tree)
              : undefined}
            archived={treeArchive}
            {archive}
            {onoperation}
            ongit={ongit ? () => ongit(tree) : undefined}
            oncommit={ongit ? () => ongit(tree, "commit") : undefined}
          />
        {/each}
        {@const missingArchive = shownArchive(archived.missing)}
        {#if missing.length || archived.missing.length}<WorkspaceGroup
            title={m.s8_unavailable_workspaces()}
            sessions={missing}
            {visibleIds}
            {compact}
            separated={renderLayout.shown.length > 0}
            hidden={hidden ||
              !showContent ||
              !!worktree ||
              Boolean(
                visibleIds &&
                !missingArchive.length &&
                !missing.some((session) => visibleIds.has(session.id)),
              )}
            {now}
            {onclose}
            archived={missingArchive}
            {archive}
          />{/if}
        {#if !layout.shown.length && layout.unpinned.length}<p
            class="text-sm text-muted-foreground"
          >
            {m.s8_repo_group_no_pinned_hint()}
          </p>{/if}
        {#if !repo.worktrees.length && !missing.length && !archived.missing.length}<p
            class="text-sm text-muted-foreground"
          >
            {repo.error
              ? m.s8_repo_group_load_wt_error()
              : m.s8_repo_group_no_worktrees()}
          </p>{/if}
      </Collapsible.Content>{/if}
  </div>
</Collapsible.Root>
