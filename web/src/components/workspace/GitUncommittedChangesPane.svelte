<script lang="ts" module>
  import type {
    GitTaskReviewState,
    GitWorkingChange,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import type { GitCommitDraftConflictState } from "../../domains/workspace/git-commit-draft-model";
  import type { GitAmendDraftPrompt } from "../../hooks/workspace/git-commit-options.svelte";
  import type { GitFileInspectorTab } from "../../hooks/workspace/git-file-inspector-controller.svelte";
  import type { GitCommitComposerCallbacks } from "./GitCommitComposer.svelte";

  export interface GitUncommittedChangesSharedProps extends GitCommitComposerCallbacks {
    context: GitWorktreeContext;
    changes: readonly GitWorkingChange[];
    reviewedPaths: readonly string[];
    commitMessage: string;
    taskReview?: GitTaskReviewState;
    stagingBusy?: boolean;
    uncertainBlocked?: boolean;
    draftSaving?: boolean;
    draftRevision?: number;
    draftConflict?: GitCommitDraftConflictState | null;
    commitAmend?: boolean;
    commitAmendPushed?: boolean;
    commitAuthorOverride?: string;
    commitSignoff?: boolean;
    commitSignoffTrailer?: string;
    amendDraftPrompt?: GitAmendDraftPrompt | null;
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
    onDiscardChanges?: (
      changeIds: readonly string[],
      paths: readonly string[],
    ) => void;
    onRefreshSnapshot?: () => void;
    onOpenStashCreate?: (includeUntracked?: boolean) => void;
    onInspectFile?: (path: string, tab: GitFileInspectorTab) => void;
    onOpenConflictResolver?: (path?: string) => void;
    onOpenWorktreeTerminal?: () => void;
    onNewWorktreeTerminal?: () => void;
    onReturnSession?: () => void;
    onArchiveSession?: () => void;
    onRemoveWorktree?: () => void;
  }
</script>

<script lang="ts">
  import IconArchive from "~icons/tabler/archive";
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
  import GitDiscardBatchBar from "./GitDiscardBatchBar.svelte";
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
    draftSaving = false,
    draftRevision = 0,
    draftConflict = null,
    commitAmend = false,
    commitAmendPushed = false,
    commitAuthorOverride = "",
    commitSignoff = false,
    commitSignoffTrailer = "",
    amendDraftPrompt = null,
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
    onDiscardChanges,
    onRefreshSnapshot,
    onOpenStashCreate,
    onInspectFile,
    onCommitMessageChange,
    onOpenCommitPreview,
    onOpenPush,
    onCreateBranchFromHead,
    onResolveDraftKeepLocal,
    onResolveDraftAdoptRemote,
    onResolveDraftMergeBoth,
    onRequestCommitAssist,
    onToggleCommitAmend,
    onResolveAmendDraft,
    onCommitAuthorOverrideChange,
    onToggleCommitSignoff,
    onOpenConflictResolver,
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
  let batchSelectedIds = $state<string[]>([]);

  $effect(() => syncElementScrollTop(listScrollEl, listScrollTop));

  const grouped = $derived(groupWorkingChanges(changes));
  const selectedBatchItems = $derived(
    changes.filter(
      (c) =>
        (c.group === "unstaged" || c.group === "untracked") &&
        batchSelectedIds.includes(c.id),
    ),
  );

  function toggleBatchSelect(item: GitWorkingChange) {
    batchSelectedIds = batchSelectedIds.includes(item.id)
      ? batchSelectedIds.filter((id) => id !== item.id)
      : [...batchSelectedIds, item.id];
  }
  const sections: { key: GitChangeGroup; title: string }[] = [
    { key: "conflicted", title: "冲突 (Conflicts)" },
    { key: "staged", title: "已暂存 (Staged · Git index)" },
    { key: "unstaged", title: "未暂存修改 (Unstaged)" },
    { key: "untracked", title: "未跟踪文件 (Untracked)" },
  ];

  const visibleEntries = $derived.by(() => {
    const out: GitWorkingChange[] = [];
    for (const sec of sections) {
      if (!collapsed[sec.key]) {
        out.push(...grouped[sec.key]);
      }
    }
    return out;
  });

  function toggleGroup(key: string) {
    collapsed = { ...collapsed, [key]: !collapsed[key] };
  }

  function focusChangeRow(index: number) {
    const rows = listScrollEl?.querySelectorAll<HTMLButtonElement>(
      "[data-working-change-row]",
    );
    rows?.[index]?.focus();
  }

  function handleListKeydown(e: KeyboardEvent) {
    if (visibleEntries.length === 0) return;
    if (!(e.target instanceof HTMLElement)) return;
    if (!e.target.hasAttribute("data-working-change-row")) return;

    const currentIdx = Math.max(
      0,
      visibleEntries.findIndex((c) => c.id === activeChange?.id),
    );
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIdx = Math.min(visibleEntries.length - 1, currentIdx + 1);
      onSelectChange(visibleEntries[nextIdx]!.id);
      focusChangeRow(nextIdx);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIdx = Math.max(0, currentIdx - 1);
      onSelectChange(visibleEntries[prevIdx]!.id);
      focusChangeRow(prevIdx);
    } else if (e.key === "Home") {
      e.preventDefault();
      onSelectChange(visibleEntries[0]!.id);
      focusChangeRow(0);
    } else if (e.key === "End") {
      e.preventDefault();
      const lastIdx = visibleEntries.length - 1;
      onSelectChange(visibleEntries[lastIdx]!.id);
      focusChangeRow(lastIdx);
    }
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label="工作树变更与提交"
>
  <GitPanelToolbar>
    <span class="min-w-0 flex-1 truncate text-xs font-semibold text-foreground">
      变更树（阅读高亮 / 已审阅 / 真实暂存三态独立）
    </span>
    <div class="flex shrink-0 items-center gap-1">
      {#if !readonlyMode && onOpenStashCreate}
        <Button
          size="xs"
          variant="outline"
          disabled={stagingBusy ||
            changes.length === 0 ||
            grouped.conflicted.length > 0}
          title={changes.length === 0
            ? "当前无未提交改动可贮藏"
            : grouped.conflicted.length > 0
              ? "存在未解决冲突时无法创建贮藏"
              : "将当前工作树改动打包保存为贮藏 (Stash)"}
          onclick={() => onOpenStashCreate(grouped.untracked.length > 0)}
          data-testid="git-commit-stash-changes-btn"
        >
          <IconArchive aria-hidden="true" />
          贮藏更改…
        </Button>
      {/if}
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
    </div>
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
      }。请打开三方冲突解决器处理，或在工作树终端解决。`}
      {hasActiveWorktreeSession}
      onOpenConflictResolver={grouped.conflicted.length > 0 &&
      onOpenConflictResolver
        ? () => onOpenConflictResolver(grouped.conflicted[0]?.path)
        : undefined}
      {onOpenWorktreeTerminal}
      {onNewWorktreeTerminal}
    />
  {/if}

  {#if !readonlyMode && onDiscardChanges}
    <GitDiscardBatchBar
      selectedItems={selectedBatchItems}
      busy={stagingBusy}
      onStageSelected={(items) => {
        for (const item of items) onStageChange(item.id);
      }}
      onDiscardSelected={(items) =>
        onDiscardChanges(
          items.map((i) => i.id),
          items.map((i) => i.path),
        )}
      onClearSelection={() => (batchSelectedIds = [])}
    />
  {/if}

  <!-- Interactive listbox delegates ArrowUp/ArrowDown/Home/End across child option buttons. -->
  <!-- svelte-ignore a11y_interactive_supports_focus -->
  <div
    bind:this={listScrollEl}
    class="min-h-0 flex-1 space-y-2 overflow-y-auto p-2"
    role="listbox"
    aria-label="工作树变更条目"
    onscroll={(e) => onListScrollChange?.(e.currentTarget.scrollTop)}
    onkeydown={handleListKeydown}
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
          <div class="min-w-0 basis-32 flex-1">
            <ListButton
              variant="section-header"
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
            {:else if sec.key === "unstaged" || sec.key === "untracked"}
              {#if onDiscardChanges}
                <Button
                  size="xs"
                  variant="ghost"
                  disabled={stagingBusy}
                  onclick={() =>
                    onDiscardChanges(
                      list.map((i) => i.id),
                      list.map((i) => i.path),
                    )}
                  data-testid={`git-discard-group-${sec.key}`}
                >
                  全部丢弃…
                </Button>
              {/if}
              <Button
                size="xs"
                variant="ghost"
                disabled={stagingBusy}
                onclick={() =>
                  onStageGroup(
                    "stage",
                    sec.key === "unstaged" ? "unstaged" : "untracked",
                  )}
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
                  batchSelectable={Boolean(
                    onDiscardChanges &&
                    (entry.group === "unstaged" || entry.group === "untracked"),
                  )}
                  isSelectedForBatch={batchSelectedIds.includes(entry.id)}
                  {readonlyMode}
                  {stagingBusy}
                  onSelect={() => onSelectChange(entry.id)}
                  onToggleBatchSelect={toggleBatchSelect}
                  {onToggleReviewed}
                  {onStageChange}
                  {onUnstageChange}
                  onDiscardChange={onDiscardChanges &&
                  (entry.group === "unstaged" || entry.group === "untracked")
                    ? (item) => onDiscardChanges([item.id], [item.path])
                    : undefined}
                  {onOpenConflictResolver}
                  {onInspectFile}
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
    {draftSaving}
    {draftRevision}
    {draftConflict}
    {commitAmend}
    {commitAmendPushed}
    {commitAuthorOverride}
    {commitSignoff}
    {commitSignoffTrailer}
    {amendDraftPrompt}
    canAssistCommitDraft={activeSessionCount > 0}
    {onCommitMessageChange}
    {onOpenCommitPreview}
    {onOpenPush}
    {onCreateBranchFromHead}
    {onResolveDraftKeepLocal}
    {onResolveDraftAdoptRemote}
    {onResolveDraftMergeBoth}
    {onRequestCommitAssist}
    {onToggleCommitAmend}
    {onResolveAmendDraft}
    {onCommitAuthorOverrideChange}
    {onToggleCommitSignoff}
  />
</section>
