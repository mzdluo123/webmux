<script lang="ts">
  import { untrack } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconPlayerTrackNext from "~icons/tabler/player-track-next";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { dockGroup, dockSplit } from "$lib/dock-layout";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitReviewScope } from "../../contracts/git-workbench-schemas";
  import type { GitHunkAndLineMutationProps } from "../../domains/workspace/git-line-selection";
  import type { createReworkDraftItem } from "../../domains/workspace/git-rework-model";
  import type { GitPaneScrollCallbacks } from "../../domains/workspace/git-workbench-memory";
  import { isChangeReviewed } from "../../domains/workspace/git-workbench-state";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import {
    computeLocalReviewProgress,
    findNextUnreviewedTaskFile,
    findNextUnreviewedWorkingChange,
    type GitReviewProgressSummary,
  } from "../../hooks/workspace/git-review-sync.svelte";
  import type { GitStagingDiffController } from "../../hooks/workspace/git-staging-diff.svelte";
  import DockPanel from "../shared/DockPanel.svelte";
  import DockWorkspace from "../shared/DockWorkspace.svelte";
  import GitCommitComposer, {
    type GitCommitComposerProps,
  } from "./GitCommitComposer.svelte";
  import GitCommitReviewToolbar from "./GitCommitReviewToolbar.svelte";
  import GitCompactPanelContent from "./GitCompactPanelContent.svelte";
  import GitLogDiffPanel from "./GitLogDiffPanel.svelte";
  import GitStagingDiffPanel from "./GitStagingDiffPanel.svelte";
  import GitTaskReviewPane from "./GitTaskReviewPane.svelte";
  import GitUncommittedChangesPane, {
    type GitUncommittedChangesProps,
  } from "./GitUncommittedChangesPane.svelte";

  let {
    context,
    storageKey = "git.commit",
    changes,
    activeChangeId,
    reviewedPaths,
    reviewScope: controlledReviewScope,
    taskReview,
    taskDiffReader,
    stagingDiff,
    stagingBusy = false,
    activeDiffStale = false,
    activeDiffStaleReason = "",
    defaultLinePickerOpen = false,
    uncommittedProgress: externalUncommittedProgress,
    taskProgress: externalTaskProgress,
    liveWorktreeIdentityForPath,
    hasActiveWorktreeSession = false,
    activeSessionCount = 0,
    exitedSessionCount = 0,
    archivedSessionCount = 0,
    listScrollTop = 0,
    onListScrollChange,
    onReviewScopeChange,
    onSelectChange,
    onToggleReviewed,
    onToggleTaskReviewed,
    onJumpNextUnreviewed,
    onAddReworkItem,
    onStageChange,
    onUnstageChange,
    onStageGroup,
    onOpenStashCreate,
    onInspectFile,
    onStageHunk,
    onUnstageHunk,
    onStageLines,
    onUnstageLines,
    onDiscardChanges,
    onDiscardHunk,
    onDiscardLines,
    onRefreshSnapshot,
    onOpenPush,
    onOpenConflictResolver,
    onOpenWorktreeTerminal,
    onNewWorktreeTerminal,
    onSelectTaskFile,
    onTaskSearchChange,
    onLoadMoreTaskFiles,
    onRefreshTaskReview,
    onSetTaskBaseline,
    onClearTaskBaseline,
    onOpenLogToPickBaseline,
    onInspectCommit,
    onReturnSession,
    onArchiveSession,
    onRemoveWorktree,
    ...composerProps
  }: GitUncommittedChangesProps &
    Omit<
      GitCommitComposerProps,
      | "context"
      | "stagedCount"
      | "conflictedCount"
      | "readonlyMode"
      | "busy"
      | "canAssistCommitDraft"
    > &
    GitPaneScrollCallbacks & {
      storageKey?: string;
      activeChangeId: string | null;
      reviewScope?: GitReviewScope;
      taskDiffReader?: GitDiffPanelReader;
      stagingDiff?: GitStagingDiffController;
      activeDiffStale?: boolean;
      activeDiffStaleReason?: string;
      defaultLinePickerOpen?: boolean;
      uncommittedProgress?: GitReviewProgressSummary;
      taskProgress?: GitReviewProgressSummary;
      liveWorktreeIdentityForPath?: (path: string) => string | undefined;
      onReviewScopeChange?: (scope: GitReviewScope) => void;
      onToggleTaskReviewed?: (path: string) => void;
      onJumpNextUnreviewed?: (scope: GitReviewScope) => string | null;
      onAddReworkItem?: (
        input: Parameters<typeof createReworkDraftItem>[0],
        options?: { openSheet?: boolean },
      ) => void;
      onSelectTaskFile?: (path: string) => void;
      onTaskSearchChange?: (q: string) => void;
      onLoadMoreTaskFiles?: () => void;
      onRefreshTaskReview?: () => void;
      onSetTaskBaseline?: (baseOid?: string) => void;
      onClearTaskBaseline?: () => void;
      onOpenLogToPickBaseline?: () => void;
      onInspectCommit?: (oid: string) => void;
    } & GitHunkAndLineMutationProps = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  let localReviewScope = $state<GitReviewScope>("uncommitted");
  const initial = dockSplit(
    "horizontal",
    dockSplit("vertical", dockGroup("changes"), dockGroup("composer"), 0.62),
    dockGroup("diff"),
    0.4,
  );
  let mobilePanel = $state(
    untrack(() => (defaultLinePickerOpen ? "diff" : "changes")),
  );
  let dock = $state<ReturnType<typeof DockWorkspace>>();
  let compactComposerTarget = $state<HTMLDivElement>();

  const commitCounts = $derived.by(() => {
    let staged = 0;
    let conflicted = 0;
    for (const change of changes) {
      if (change.group === "staged") staged++;
      else if (change.group === "conflicted") conflicted++;
    }
    return { staged, conflicted };
  });

  const effectiveScope = $derived(controlledReviewScope ?? localReviewScope);
  const activeChange = $derived(
    changes.find((c) => c.id === activeChangeId) ?? changes[0] ?? null,
  );
  const activeTaskFile = $derived(
    taskReview?.files.find(
      (f) => (f.newPath || f.oldPath) === taskReview.selectedPath,
    ) ??
      taskReview?.files[0] ??
      null,
  );
  const taskBaselineLabel = $derived(
    !taskReview?.baseline
      ? m.s6_task_baseline_unset()
      : taskReview.baseline.unborn || !taskReview.baseline.baseOid
        ? m.s6_task_baseline_unborn()
        : taskReview.baseline.baseOid.slice(0, 7),
  );
  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );

  const activeProgress = $derived(
    (effectiveScope === "task"
      ? externalTaskProgress
      : externalUncommittedProgress) ??
      computeLocalReviewProgress(
        effectiveScope,
        changes,
        taskReview?.files ?? [],
        reviewedPaths,
        isChangeReviewed,
      ),
  );

  function setReviewScope(next: GitReviewScope) {
    localReviewScope = next;
    if (!desktop.current) dock?.show("changes");
    onReviewScopeChange?.(next);
  }

  function handleNextUnreviewed() {
    if (onJumpNextUnreviewed) {
      if (onJumpNextUnreviewed(effectiveScope) && !desktop.current) {
        dock?.show("diff");
      }
      return;
    }
    if (effectiveScope === "task") {
      const nextPath = findNextUnreviewedTaskFile(
        taskReview?.files ?? [],
        taskReview?.selectedPath ?? null,
        (p) => reviewedPaths.includes(`task:${p}`) || reviewedPaths.includes(p),
      );
      if (nextPath) {
        onSelectTaskFile?.(nextPath);
        if (!desktop.current) dock?.show("diff");
      }
      return;
    }
    const nextChange = findNextUnreviewedWorkingChange(
      changes,
      activeChange?.id ?? null,
      (c) => isChangeReviewed(reviewedPaths, c),
    );
    if (nextChange) {
      onSelectChange(nextChange.id);
      if (!desktop.current) dock?.show("diff");
    }
  }
</script>

{#snippet leftPaneContent()}
  {#if effectiveScope === "task"}
    <GitTaskReviewPane
      {context}
      {taskReview}
      {reviewedPaths}
      {activeSessionCount}
      {exitedSessionCount}
      {archivedSessionCount}
      onSelectFile={(path) => {
        onSelectTaskFile?.(path);
        if (!desktop.current) dock?.show("diff");
      }}
      onSearchChange={onTaskSearchChange}
      onLoadMore={onLoadMoreTaskFiles}
      onRefresh={onRefreshTaskReview}
      onSetBaseline={onSetTaskBaseline}
      onClearBaseline={onClearTaskBaseline}
      {onOpenLogToPickBaseline}
      {onInspectCommit}
      onToggleReviewed={(path) =>
        onToggleTaskReviewed
          ? onToggleTaskReviewed(path)
          : onToggleReviewed({
              id: `task:${path}`,
              path,
              group: "unstaged",
              status: "M",
              binary: false,
              hunkStageAvailable: false,
              preStagedFromCli: false,
              identityKey: "",
              hunks: [],
            })}
      {onReturnSession}
      {onOpenPush}
      {onArchiveSession}
      {onRemoveWorktree}
    />
  {:else}
    <GitUncommittedChangesPane
      bind:composerTarget={compactComposerTarget}
      {context}
      {changes}
      {activeChange}
      {reviewedPaths}
      {taskReview}
      {readonlyMode}
      {stagingBusy}
      {hasActiveWorktreeSession}
      {activeSessionCount}
      {exitedSessionCount}
      {archivedSessionCount}
      {listScrollTop}
      {onListScrollChange}
      onSwitchToTaskReview={() => setReviewScope("task")}
      onSelectChange={(id) => {
        onSelectChange(id);
        if (!desktop.current) dock?.show("diff");
      }}
      {onToggleReviewed}
      {onStageChange}
      {onUnstageChange}
      {onStageGroup}
      {onOpenStashCreate}
      {onInspectFile}
      {onDiscardChanges}
      {onRefreshSnapshot}
      {onOpenPush}
      {onOpenConflictResolver}
      {onOpenWorktreeTerminal}
      {onNewWorktreeTerminal}
      {onReturnSession}
      {onArchiveSession}
      {onRemoveWorktree}
    />
  {/if}
{/snippet}

{#snippet rightPaneContent(mobile: boolean)}
  {#if effectiveScope === "task"}
    {@const activeTaskPath = activeTaskFile
      ? activeTaskFile.newPath || activeTaskFile.oldPath
      : ""}
    <GitLogDiffPanel
      logDiff={taskDiffReader}
      fallbackFile={activeTaskFile}
      fallbackLeftLabel={taskBaselineLabel}
      fallbackRightLabel={m.s6_current_workspace()}
      backLabel={m.s6_back_to_task_changes()}
      presentation={mobile ? "mobile" : "docked"}
      isTaskScope={true}
      liveWorktreeIdentity={activeTaskPath
        ? liveWorktreeIdentityForPath?.(activeTaskPath)
        : undefined}
      {onAddReworkItem}
      onBack={mobile ? () => dock?.show("changes") : undefined}
    />
  {:else}
    <section
      class="flex h-full min-h-0 min-w-0 flex-col bg-background"
      aria-label={m.s6_staging_diff_section_aria()}
    >
      {#if mobile}
        <div
          class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b bg-background p-2"
        >
          <Button
            size="sm"
            variant="ghost"
            wrap
            onclick={() => dock?.show("changes")}
          >
            <IconArrowLeft aria-hidden="true" />
            {m.s6_back_to_changes()}
          </Button>
          {#if activeProgress.remaining > 0}
            <Button
              size="sm"
              variant="outline"
              wrap
              onclick={handleNextUnreviewed}
              data-testid="git-mobile-next-unreviewed"
            >
              <IconPlayerTrackNext aria-hidden="true" />
              {m.s6_next_unreviewed_with_count({
                count: activeProgress.remaining,
              })}
            </Button>
          {/if}
        </div>
      {/if}

      <div class="flex min-h-0 flex-1 flex-col">
        {#if !activeChange}
          <p class="p-4 text-xs text-muted-foreground">
            {m.s6_select_change_prompt()}
          </p>
        {:else}
          <GitStagingDiffPanel
            change={activeChange}
            {context}
            {stagingDiff}
            {readonlyMode}
            busy={stagingBusy}
            stale={activeDiffStale}
            staleReason={activeDiffStaleReason}
            {defaultLinePickerOpen}
            {hasActiveWorktreeSession}
            liveWorktreeIdentity={liveWorktreeIdentityForPath?.(
              activeChange.path,
            )}
            {onStageChange}
            {onUnstageChange}
            {onStageHunk}
            {onUnstageHunk}
            {onStageLines}
            {onUnstageLines}
            onDiscardFile={onDiscardChanges
              ? (id, path) => onDiscardChanges([id], [path])
              : undefined}
            {onDiscardHunk}
            {onDiscardLines}
            {onAddReworkItem}
            {onRefreshSnapshot}
            {onOpenConflictResolver}
            {onOpenWorktreeTerminal}
            {onNewWorktreeTerminal}
          />
        {/if}
      </div>
    </section>
  {/if}
{/snippet}

<div class="flex h-full min-h-0 min-w-0 flex-col bg-background">
  <GitCommitReviewToolbar
    scope={effectiveScope}
    workingChangeCount={changes.length}
    taskFileCount={taskReview?.status === "ready"
      ? taskReview.files.length
      : undefined}
    progress={readonlyMode ? undefined : activeProgress}
    onScopeChange={setReviewScope}
    onNextUnreviewed={handleNextUnreviewed}
    onResetLayout={desktop.current ? () => dock?.reset() : undefined}
  />

  <DockWorkspace
    {initial}
    {storageKey}
    compact={!desktop.current}
    bind:mobilePanel
    bind:this={dock}
  >
    <DockPanel id="changes" title={m.s5_changes_tab()}>
      {@render leftPaneContent()}
    </DockPanel>

    <DockPanel id="composer" title={m.dock_git_commit_message()}>
      {#snippet children(visible)}
        <GitCompactPanelContent
          target={!desktop.current && !visible
            ? compactComposerTarget
            : undefined}
        >
          <ScrollArea
            class={!desktop.current && !visible ? "shrink-0" : "flex-1"}
            viewportClasses="flex-1"
          >
            <GitCommitComposer
              {...composerProps}
              {context}
              {readonlyMode}
              stagedCount={commitCounts.staged}
              conflictedCount={commitCounts.conflicted}
              busy={stagingBusy}
              canAssistCommitDraft={activeSessionCount > 0}
              {onOpenPush}
            />
          </ScrollArea>
        </GitCompactPanelContent>
      {/snippet}
    </DockPanel>

    <DockPanel id="diff" title={m.s6_log_diff_preview_placeholder()}>
      {@render rightPaneContent(!desktop.current)}
    </DockPanel>
  </DockWorkspace>
</div>
