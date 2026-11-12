<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconPlayerTrackNext from "~icons/tabler/player-track-next";
  import IconRestore from "~icons/tabler/restore";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Resizable from "$lib/components/ui/resizable";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import type { GitReviewScope } from "../../contracts/git-workbench-schemas";
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
    commitMessage,
    reviewScope: controlledReviewScope,
    taskReview,
    taskDiffReader,
    stagingDiff,
    stagingBusy = false,
    uncertainBlocked = false,
    draftSaving = false,
    draftRevision = 0,
    draftConflict = null,
    activeDiffStale = false,
    activeDiffStaleReason = "",
    uncommittedProgress: externalUncommittedProgress,
    taskProgress: externalTaskProgress,
    liveWorktreeIdentityForPath,
    hasActiveWorktreeSession = false,
    activeSessionCount = 0,
    exitedSessionCount = 0,
    archivedSessionCount = 0,
    paneSizes = [40, 60],
    diffMode = "unified",
    listScrollTop = 0,
    onPaneSizesChange,
    onResetLayout,
    onDiffModeChange,
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
    onStageHunk,
    onUnstageHunk,
    onRefreshSnapshot,
    onCommitMessageChange,
    onOpenCommitPreview,
    onOpenPush,
    onCreateBranchFromHead,
    onResolveDraftKeepLocal,
    onResolveDraftAdoptRemote,
    onResolveDraftMergeBoth,
    onRequestCommitAssist,
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
  }: GitUncommittedChangesSharedProps &
    GitPaneScrollCallbacks & {
      activeChangeId: string | null;
      reviewScope?: GitReviewScope;
      taskDiffReader?: GitDiffPanelReader;
      stagingDiff?: GitStagingDiffController;
      activeDiffStale?: boolean;
      activeDiffStaleReason?: string;
      uncommittedProgress?: GitReviewProgressSummary;
      taskProgress?: GitReviewProgressSummary;
      liveWorktreeIdentityForPath?: (path: string) => string | undefined;
      paneSizes?: [number, number];
      onPaneSizesChange?: (sizes: [number, number]) => void;
      onReviewScopeChange?: (scope: GitReviewScope) => void;
      onToggleTaskReviewed?: (path: string) => void;
      onJumpNextUnreviewed?: (scope: GitReviewScope) => string | null;
      onAddReworkItem?: (
        input: Parameters<typeof createReworkDraftItem>[0],
        options?: { openSheet?: boolean },
      ) => void;
      onStageHunk: (changeId: string, hunkId: string) => void;
      onUnstageHunk: (changeId: string, hunkId: string) => void;
      onSelectTaskFile?: (path: string) => void;
      onTaskSearchChange?: (q: string) => void;
      onLoadMoreTaskFiles?: () => void;
      onRefreshTaskReview?: () => void;
      onSetTaskBaseline?: (baseOid?: string) => void;
      onClearTaskBaseline?: () => void;
      onOpenLogToPickBaseline?: () => void;
      onInspectCommit?: (oid: string) => void;
    } = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  let localReviewScope = $state<GitReviewScope>("uncommitted");
  let mobileStep = $state<"changes" | "diff">("changes");
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
      ? "未设起点"
      : taskReview.baseline.unborn || !taskReview.baseline.baseOid
        ? "空仓起点"
        : taskReview.baseline.baseOid.slice(0, 7),
  );
  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );

  const activeProgress = $derived(
    effectiveScope === "task"
      ? (externalTaskProgress ??
          computeLocalReviewProgress(
            "task",
            changes,
            taskReview?.files ?? [],
            reviewedPaths,
            isChangeReviewed,
          ))
      : (externalUncommittedProgress ??
          computeLocalReviewProgress(
            "uncommitted",
            changes,
            taskReview?.files ?? [],
            reviewedPaths,
            isChangeReviewed,
          )),
  );

  function setReviewScope(next: GitReviewScope) {
    localReviewScope = next;
    mobileStep = "changes";
    onReviewScopeChange?.(next);
  }

  function handleNextUnreviewed() {
    if (onJumpNextUnreviewed) {
      const jumped = onJumpNextUnreviewed(effectiveScope);
      if (jumped && !desktop.current) mobileStep = "diff";
      return;
    }
    if (effectiveScope === "task") {
      const files = taskReview?.files ?? [];
      const nextPath = findNextUnreviewedTaskFile(
        files,
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
      {context}
      {changes}
      {activeChange}
      {reviewedPaths}
      {commitMessage}
      {taskReview}
      {readonlyMode}
      {stagingBusy}
      {uncertainBlocked}
      {draftSaving}
      {draftRevision}
      {draftConflict}
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
      {onStageGroup}
      {onRefreshSnapshot}
      {onCommitMessageChange}
      {onOpenCommitPreview}
      {onOpenPush}
      {onCreateBranchFromHead}
      {onResolveDraftKeepLocal}
      {onResolveDraftAdoptRemote}
      {onResolveDraftMergeBoth}
      {onRequestCommitAssist}
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
      fallbackRightLabel="当前工作区"
      backLabel="返回任务变更列表"
      presentation={mobile ? "mobile" : "docked"}
      isTaskScope={true}
      liveWorktreeIdentity={activeTaskPath
        ? liveWorktreeIdentityForPath?.(activeTaskPath)
        : undefined}
      {onAddReworkItem}
      {diffMode}
      {onDiffModeChange}
      onBack={mobile ? () => (mobileStep = "changes") : undefined}
    />
  {:else}
    <section
      class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
      aria-label="工作树文件差异与代码块暂存"
    >
      {#if mobile}
        <div
          class="flex items-center justify-between gap-2 border-b border-git-border bg-git-panel p-2"
        >
          <Button
            size="xs"
            variant="ghost"
            onclick={() => (mobileStep = "changes")}
          >
            <IconArrowLeft aria-hidden="true" />
            返回变更列表
          </Button>
          {#if activeProgress.remaining > 0}
            <Button
              size="xs"
              variant="outline"
              onclick={handleNextUnreviewed}
              data-testid="git-mobile-next-unreviewed"
            >
              <IconPlayerTrackNext aria-hidden="true" />
              下一处未审阅 ({activeProgress.remaining})
            </Button>
          {/if}
        </div>
      {/if}

      <div class="min-h-0 flex-1">
        {#if !activeChange}
          <p class="p-4 text-xs text-muted-foreground">
            选择左侧任意变更项查看差异或逐块暂存。
          </p>
        {:else}
          <GitStagingDiffPanel
            change={activeChange}
            {context}
            {stagingDiff}
            {diffMode}
            {readonlyMode}
            busy={stagingBusy}
            stale={activeDiffStale}
            staleReason={activeDiffStaleReason}
            {hasActiveWorktreeSession}
            liveWorktreeIdentity={liveWorktreeIdentityForPath?.(
              activeChange.path,
            )}
            {onDiffModeChange}
            {onStageChange}
            {onUnstageChange}
            {onStageHunk}
            {onUnstageHunk}
            {onAddReworkItem}
            {onRefreshSnapshot}
            {onOpenWorktreeTerminal}
            {onNewWorktreeTerminal}
          />
        {/if}
      </div>
    </section>
  {/if}
{/snippet}

<div class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface">
  <GitPanelToolbar>
    <SegmentedControl.Root
      wrap
      value={effectiveScope}
      onValueChange={(val) => {
        if (val === "uncommitted" || val === "task") setReviewScope(val);
      }}
      aria-label="审阅范围"
    >
      <SegmentedControl.Item value="uncommitted">
        未提交改动 ({changes.length})
      </SegmentedControl.Item>
      <SegmentedControl.Item
        value="task"
        aria-label={taskReview?.status === "ready"
          ? `相对任务起点的变化 (${taskReview.files.length})`
          : "相对任务起点的变化"}
        title="相对任务起点的变化"
      >
        <span class="sm:hidden">任务变化</span>
        <span class="max-sm:hidden">相对任务起点的变化</span>
        {#if taskReview?.status === "ready"}
          <span>({taskReview.files.length})</span>
        {/if}
      </SegmentedControl.Item>
    </SegmentedControl.Root>

    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      {#if !readonlyMode && activeProgress.total > 0}
        <Badge
          variant={activeProgress.remaining === 0 ? "secondary" : "outline"}
          data-testid="git-review-progress-badge"
        >
          已审阅 {activeProgress.reviewed}/{activeProgress.total}
        </Badge>
        <Button
          size="xs"
          variant="outline"
          disabled={activeProgress.remaining === 0}
          onclick={handleNextUnreviewed}
          aria-label="下一处未审阅"
          title="定位并打开下一处未审阅文件"
          data-testid="git-next-unreviewed"
        >
          <IconPlayerTrackNext aria-hidden="true" />
          <span class="sm:hidden">下一处</span>
          <span class="max-sm:hidden">下一处未审阅</span>
        </Button>
      {/if}

      {#if desktop.current}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label="重置提交分栏比例"
          title="重置提交分栏比例"
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
