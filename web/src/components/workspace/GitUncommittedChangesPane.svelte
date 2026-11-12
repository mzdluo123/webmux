<script lang="ts" module>
  import type {
    GitTaskReviewState,
    GitWorkingChange,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";

  export interface GitUncommittedChangesSharedProps {
    context: GitWorktreeContext;
    changes: readonly GitWorkingChange[];
    reviewedPaths: readonly string[];
    commitMessage: string;
    taskReview?: GitTaskReviewState;
    stagingBusy?: boolean;
    uncertainBlocked?: boolean;
    hasActiveWorktreeSession?: boolean;
    activeSessionCount?: number;
    exitedSessionCount?: number;
    archivedSessionCount?: number;
    onSelectChange: (id: string) => void;
    onToggleReviewed: (change: GitWorkingChange) => void;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
    onStageGroup?: (
      op: "stage" | "unstage",
      group: "staged" | "unstaged" | "untracked",
    ) => void;
    onRefreshSnapshot?: () => void;
    onCommitMessageChange: (msg: string) => void;
    onOpenCommitPreview: (followUpPush: boolean) => void;
    onOpenPush?: () => void;
    onCreateBranchFromHead?: () => void;
    onOpenWorktreeTerminal?: () => void;
    onNewWorktreeTerminal?: () => void;
    onReturnSession?: () => void;
    onArchiveSession?: () => void;
    onRemoveWorktree?: () => void;
  }
</script>

<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { GitChangeGroup } from "../../contracts/git-workbench-schemas";
  import { syncElementScrollTop } from "../../domains/workspace/git-workbench-memory";
  import {
    groupWorkingChanges,
    isChangeReviewed,
  } from "../../domains/workspace/git-workbench-state";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitCommitComposer from "./GitCommitComposer.svelte";
  import GitConflictTerminalNotice from "./GitConflictTerminalNotice.svelte";
  import GitPanelToolbar from "./GitPanelToolbar.svelte";
  import GitTaskDeliverySummary from "./GitTaskDeliverySummary.svelte";
  import GitWorkingChangeRow from "./GitWorkingChangeRow.svelte";

  let {
    context,
    changes,
    activeChange,
    reviewedPaths,
    commitMessage,
    taskReview,
    readonlyMode = false,
    stagingBusy = false,
    uncertainBlocked = false,
    hasActiveWorktreeSession = false,
    activeSessionCount = 0,
    exitedSessionCount = 0,
    archivedSessionCount = 0,
    listScrollTop = 0,
    onListScrollChange,
    onSwitchToTaskReview,
    onSelectChange,
    onToggleReviewed,
    onStageChange,
    onUnstageChange,
    onStageGroup,
    onRefreshSnapshot,
    onCommitMessageChange,
    onOpenCommitPreview,
    onOpenPush,
    onCreateBranchFromHead,
    onOpenWorktreeTerminal,
    onNewWorktreeTerminal,
    onReturnSession,
    onArchiveSession,
    onRemoveWorktree,
  }: GitUncommittedChangesSharedProps & {
    activeChange: GitWorkingChange | null;
    readonlyMode?: boolean;
    listScrollTop?: number;
    onListScrollChange?: (top: number) => void;
    onSwitchToTaskReview?: () => void;
  } = $props();

  let listScrollEl = $state<HTMLDivElement>();
  let collapsed = $state<Record<string, boolean>>({});

  $effect(() => syncElementScrollTop(listScrollEl, listScrollTop));

  const grouped = $derived(groupWorkingChanges(changes));
  const sections: { key: GitChangeGroup; title: string }[] = [
    { key: "conflicted", title: "冲突 (Conflicts)" },
    { key: "staged", title: "已暂存 (Staged · Git index)" },
    { key: "unstaged", title: "未暂存修改 (Unstaged)" },
    { key: "untracked", title: "未跟踪文件 (Untracked)" },
  ];

  function toggleGroup(key: string) {
    collapsed = { ...collapsed, [key]: !collapsed[key] };
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
  aria-label="工作树变更与提交"
>
  <GitPanelToolbar>
    <span class="min-w-0 truncate text-xs font-semibold text-foreground">
      变更树（阅读高亮 / 已审阅 / 真实暂存三态独立）
    </span>
    {#if onRefreshSnapshot}
      <Button
        size="icon-sm"
        variant="ghost"
        aria-label="刷新暂存区快照"
        title="刷新暂存区快照"
        onclick={onRefreshSnapshot}
      >
        <IconRefresh aria-hidden="true" />
      </Button>
    {/if}
  </GitPanelToolbar>

  {#if readonlyMode}
    <div class="p-3">
      <InlineNotice tone="info">
        当前处于无工作树只读仓库上下文，无法暂存或创建提交。请切换到具体工作树。
      </InlineNotice>
    </div>
  {/if}

  {#if grouped.conflicted.length > 0 || context.operationInProgress}
    <GitConflictTerminalNotice
      message={`${
        context.operationInProgress
          ? `正在进行 Git 操作：${context.operationInProgress}（冲突 ${grouped.conflicted.length} 项）`
          : `存在 ${grouped.conflicted.length} 个未解决冲突`
      }。请前往工作树终端解决冲突。`}
      {hasActiveWorktreeSession}
      {onOpenWorktreeTerminal}
      {onNewWorktreeTerminal}
    />
  {/if}

  <div
    bind:this={listScrollEl}
    class="min-h-0 flex-1 space-y-2 overflow-y-auto p-2"
    onscroll={(e) => onListScrollChange?.(e.currentTarget.scrollTop)}
  >
    {#if changes.length === 0 && !readonlyMode && taskReview?.status === "ready" && (taskReview.commitCount > 0 || context.ahead > 0)}
      <GitTaskDeliverySummary
        {context}
        compact
        commitCount={taskReview.commitCount}
        uncommittedCount={taskReview.uncommittedCount}
        netFileCount={taskReview.files.length}
        {activeSessionCount}
        {exitedSessionCount}
        {archivedSessionCount}
        {onSwitchToTaskReview}
        {onReturnSession}
        {onOpenPush}
        {onArchiveSession}
        {onRemoveWorktree}
      />
    {/if}

    {#each sections as sec (sec.key)}
      {@const list = grouped[sec.key]}
      {@const isOpen = !collapsed[sec.key]}
      <div class="space-y-0.5" data-change-section={sec.key}>
        <div class="flex min-w-0 flex-wrap items-center gap-1">
          <div class="min-w-[8rem] flex-1">
            <ListButton
              variant="git-section"
              aria-expanded={isOpen}
              onclick={() => toggleGroup(sec.key)}
            >
              {#if isOpen}
                <IconChevronDown class="size-3.5 shrink-0" aria-hidden="true" />
              {:else}
                <IconChevronRight
                  class="size-3.5 shrink-0"
                  aria-hidden="true"
                />
              {/if}
              <span class="min-w-0 flex-1 truncate">{sec.title}</span>
              <Badge variant="outline">{list.length}</Badge>
            </ListButton>
          </div>

          {#if !readonlyMode && list.length > 0 && onStageGroup}
            {#if sec.key === "staged"}
              <Button
                size="xs"
                variant="ghost"
                disabled={stagingBusy}
                onclick={() => onStageGroup("unstage", "staged")}
              >
                全部取消暂存
              </Button>
            {:else if sec.key === "unstaged"}
              <Button
                size="xs"
                variant="ghost"
                disabled={stagingBusy}
                onclick={() => onStageGroup("stage", "unstaged")}
              >
                全部暂存
              </Button>
            {:else if sec.key === "untracked"}
              <Button
                size="xs"
                variant="ghost"
                disabled={stagingBusy}
                onclick={() => onStageGroup("stage", "untracked")}
              >
                全部暂存
              </Button>
            {/if}
          {/if}
        </div>

        {#if isOpen}
          {#if list.length === 0}
            <p class="px-6 py-1 text-xs text-muted-foreground">暂无条目</p>
          {:else}
            <div class="space-y-0.5">
              {#each list as entry (entry.id)}
                <GitWorkingChangeRow
                  item={entry}
                  isReviewed={isChangeReviewed(reviewedPaths, entry)}
                  isReading={activeChange?.id === entry.id}
                  {readonlyMode}
                  {stagingBusy}
                  onSelect={() => onSelectChange(entry.id)}
                  {onToggleReviewed}
                  {onStageChange}
                  {onUnstageChange}
                />
              {/each}
            </div>
          {/if}
        {/if}
      </div>
    {/each}
  </div>

  <GitCommitComposer
    {context}
    stagedCount={grouped.staged.length}
    conflictedCount={grouped.conflicted.length}
    {commitMessage}
    {readonlyMode}
    busy={stagingBusy}
    {uncertainBlocked}
    {onCommitMessageChange}
    {onOpenCommitPreview}
    {onOpenPush}
    {onCreateBranchFromHead}
  />
</section>
