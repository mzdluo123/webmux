<script lang="ts">
  import { untrack } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconPlayerTrackNext from "~icons/tabler/player-track-next";
  import IconRestore from "~icons/tabler/restore";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Resizable from "$lib/components/ui/resizable";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitReviewScope } from "../../contracts/git-workbench-schemas";
  import type { GitHunkAndLineMutationProps } from "../../domains/workspace/git-line-selection";
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
  import GitLogDiffPanel from "./GitLogDiffPanel.svelte";
  import GitPanelToolbar from "./GitPanelToolbar.svelte";
  import GitStagingDiffPanel from "./GitStagingDiffPanel.svelte";
  import GitTaskReviewPane from "./GitTaskReviewPane.svelte";
  import GitUncommittedChangesPane, {
    type GitUncommittedChangesSharedProps,
  } from "./GitUncommittedChangesPane.svelte";

  let {
    context,
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
    hasActiveWorktreeSession = false,
    activeSessionCount = 0,
    exitedSessionCount = 0,
    archivedSessionCount = 0,
    paneSizes = [40, 60],
    listScrollTop = 0,
    onPaneSizesChange,
    onResetLayout,
    onListScrollChange,
    onReviewScopeChange,
    onSelectChange,
    onToggleReviewed,
    onToggleTaskReviewed,
    onJumpNextUnreviewed,
    onStageChange,
    onUnstageChange,
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
    ...composerAndDraftProps
  }: GitUncommittedChangesSharedProps &
    GitPaneScrollCallbacks & {
      activeChangeId: string | null;
      reviewScope?: GitReviewScope;
      taskDiffReader?: GitDiffPanelReader;
      stagingDiff?: GitStagingDiffController;
      activeDiffStale?: boolean;
      activeDiffStaleReason?: string;
      defaultLinePickerOpen?: boolean;
      uncommittedProgress?: GitReviewProgressSummary;
      taskProgress?: GitReviewProgressSummary;
      paneSizes?: [number, number];
      onPaneSizesChange?: (sizes: [number, number]) => void;
      onReviewScopeChange?: (scope: GitReviewScope) => void;
      onToggleTaskReviewed?: (path: string) => void;
      onJumpNextUnreviewed?: (scope: GitReviewScope) => string | null;
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
  let mobileStep = $state<"changes" | "diff">(
    untrack(() => (defaultLinePickerOpen ? "diff" : "changes")),
  );
  let paneResetKey = $state(0);

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
    mobileStep = "changes";
    onReviewScopeChange?.(next);
  }

  function handleNextUnreviewed() {
    if (onJumpNextUnreviewed) {
      if (onJumpNextUnreviewed(effectiveScope) && !desktop.current) {
        mobileStep = "diff";
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
        if (!desktop.current) mobileStep = "diff";
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
      if (!desktop.current) mobileStep = "diff";
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
        if (!desktop.current) mobileStep = "diff";
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
      {...composerAndDraftProps}
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
        if (!desktop.current) mobileStep = "diff";
      }}
      {onToggleReviewed}
      {onStageChange}
      {onUnstageChange}
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
    <GitLogDiffPanel
      logDiff={taskDiffReader}
      fallbackFile={activeTaskFile}
      fallbackLeftLabel={taskBaselineLabel}
      fallbackRightLabel={m.s6_current_workspace()}
      backLabel={m.s6_back_to_task_changes()}
      presentation={mobile ? "mobile" : "docked"}
      onBack={mobile ? () => (mobileStep = "changes") : undefined}
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
            onclick={() => (mobileStep = "changes")}
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
  <GitPanelToolbar>
    <SegmentedControl
      wrap
      value={effectiveScope}
      onValueChange={(val) => {
        if (val === "uncommitted" || val === "task") setReviewScope(val);
      }}
      aria-label={m.s6_review_scope_aria()}
    >
      <SegmentedControlItem value="uncommitted">
        {m.s6_uncommitted_changes_tab({ count: changes.length })}
      </SegmentedControlItem>
      <SegmentedControlItem
        value="task"
        aria-label={taskReview?.status === "ready"
          ? m.s6_task_changes_with_count({ count: taskReview.files.length })
          : m.s6_task_changes_label()}
        title={m.s6_task_changes_label()}
      >
        <span class="sm:hidden">{m.s6_task_changes_short()}</span>
        <span class="max-sm:hidden">{m.s6_task_changes_label()}</span>
        {#if taskReview?.status === "ready"}
          <span>({taskReview.files.length})</span>
        {/if}
      </SegmentedControlItem>
    </SegmentedControl>

    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      {#if !readonlyMode && activeProgress.total > 0}
        <Badge
          variant={activeProgress.remaining === 0 ? "secondary" : "outline"}
          data-testid="git-review-progress-badge"
        >
          {m.s6_reviewed_progress_badge({
            reviewed: activeProgress.reviewed,
            total: activeProgress.total,
          })}
        </Badge>
        <Button
          size="sm"
          variant="outline"
          disabled={activeProgress.remaining === 0}
          onclick={handleNextUnreviewed}
          aria-label={m.s6_next_unreviewed_label()}
          title={m.s6_next_unreviewed_tooltip()}
          data-testid="git-next-unreviewed"
        >
          <IconPlayerTrackNext aria-hidden="true" />
          <span class="sm:hidden">{m.s6_next_unreviewed_short()}</span>
          <span class="max-sm:hidden">{m.s6_next_unreviewed_label()}</span>
        </Button>
      {/if}

      {#if desktop.current}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label={m.s6_reset_commit_pane_layout()}
          title={m.s6_reset_commit_pane_layout()}
          onclick={() => {
            onResetLayout?.();
            paneResetKey++;
          }}
        >
          <IconRestore aria-hidden="true" />
        </Button>
      {/if}
    </div>
  </GitPanelToolbar>

  <div class="min-h-0 min-w-0 flex-1 overflow-hidden">
    {#if desktop.current}
      {#key paneResetKey}
        <Resizable.PaneGroup
          direction="horizontal"
          class="h-full min-h-0 min-w-0"
          onLayoutChange={(sizes) => {
            if (sizes.length === 2) {
              onPaneSizesChange?.([sizes[0]!, sizes[1]!]);
            }
          }}
        >
          <Resizable.Pane
            defaultSize={paneSizes[0]}
            minSize={26}
            class="git-pane"
          >
            {@render leftPaneContent()}
          </Resizable.Pane>
          <Resizable.Handle
            withHandle
            class="git-divider"
            ondblclick={() => {
              onResetLayout?.();
              paneResetKey++;
            }}
          />
          <Resizable.Pane
            defaultSize={paneSizes[1]}
            minSize={34}
            class="git-pane"
          >
            {@render rightPaneContent(false)}
          </Resizable.Pane>
        </Resizable.PaneGroup>
      {/key}
    {:else if mobileStep === "changes"}
      {@render leftPaneContent()}
    {:else}
      {@render rightPaneContent(true)}
    {/if}
  </div>
</div>
