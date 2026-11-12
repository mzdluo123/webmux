<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconCheck from "~icons/tabler/check";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconRefresh from "~icons/tabler/refresh";
  import IconRestore from "~icons/tabler/restore";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Resizable from "$lib/components/ui/resizable";
  import type {
    GitChangeGroup,
    GitWorkingChange,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import {
    syncElementScrollTop,
    type GitPaneScrollCallbacks,
  } from "../../domains/workspace/git-workbench-memory";
  import {
    groupWorkingChanges,
    isChangeReviewed,
  } from "../../domains/workspace/git-workbench-state";
  import type { GitStagingDiffController } from "../../hooks/workspace/git-staging-diff.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitCommitComposer from "./GitCommitComposer.svelte";
  import GitConflictTerminalNotice from "./GitConflictTerminalNotice.svelte";
  import GitStagingDiffPanel from "./GitStagingDiffPanel.svelte";

  let {
    context,
    changes,
    activeChangeId,
    reviewedPaths,
    commitMessage,
    stagingDiff,
    stagingBusy = false,
    activeDiffStale = false,
    activeDiffStaleReason = "",
    hasActiveWorktreeSession = false,
    paneSizes = [40, 60],
    diffMode = "unified",
    listScrollTop = 0,
    onPaneSizesChange,
    onResetLayout,
    onDiffModeChange,
    onListScrollChange,
    onSelectChange,
    onToggleReviewed,
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
    onOpenWorktreeTerminal,
    onNewWorktreeTerminal,
  }: {
    context: GitWorktreeContext;
    changes: readonly GitWorkingChange[];
    activeChangeId: string | null;
    reviewedPaths: readonly string[];
    commitMessage: string;
    stagingDiff?: GitStagingDiffController;
    stagingBusy?: boolean;
    activeDiffStale?: boolean;
    activeDiffStaleReason?: string;
    hasActiveWorktreeSession?: boolean;
    paneSizes?: [number, number];
    onPaneSizesChange?: (sizes: [number, number]) => void;
    onSelectChange: (id: string) => void;
    onToggleReviewed: (change: GitWorkingChange) => void;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
    onStageGroup?: (
      op: "stage" | "unstage",
      group: "staged" | "unstaged" | "untracked",
    ) => void;
    onStageHunk: (changeId: string, hunkId: string) => void;
    onUnstageHunk: (changeId: string, hunkId: string) => void;
    onRefreshSnapshot?: () => void;
    onCommitMessageChange: (msg: string) => void;
    onOpenCommitPreview: (followUpPush: boolean) => void;
    onOpenPush?: () => void;
    onCreateBranchFromHead?: () => void;
    onOpenWorktreeTerminal?: () => void;
    onNewWorktreeTerminal?: () => void;
  } & GitPaneScrollCallbacks = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  let mobileStep = $state<"changes" | "diff">("changes");
  let paneResetKey = $state(0);
  let listScrollEl = $state<HTMLDivElement>();
  let collapsed = $state<Record<string, boolean>>({});

  $effect(() => syncElementScrollTop(listScrollEl, listScrollTop));

  const grouped = $derived(groupWorkingChanges(changes));
  const activeChange = $derived(
    changes.find((c) => c.id === activeChangeId) ?? changes[0] ?? null,
  );
  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );

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

{#snippet changeRow(item: GitWorkingChange)}
  {@const isReviewed = isChangeReviewed(reviewedPaths, item)}
  {@const isReading = activeChange?.id === item.id}
  <div
    class="flex min-w-0 items-center gap-1 rounded-sm"
    data-change-id={item.id}
    data-reviewed={isReviewed}
    data-group={item.group}
  >
    <Button
      size="icon-xs"
      variant="ghost"
      aria-pressed={isReviewed}
      aria-label={`标记已审阅 ${item.path}`}
      title={isReviewed ? `已审阅 ${item.path}` : `标记已审阅 ${item.path}`}
      onclick={() => onToggleReviewed(item)}
    >
      <IconCheck aria-hidden="true" />
    </Button>

    <div class="min-w-0 flex-1">
      <ListButton
        variant="git-row"
        aria-selected={isReading}
        onclick={() => {
          onSelectChange(item.id);
          if (!desktop.current) mobileStep = "diff";
        }}
      >
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
          <Badge
            variant={item.group === "conflicted"
              ? "destructive"
              : item.group === "staged"
                ? "info"
                : "outline"}
          >
            {item.status}
          </Badge>
          <span
            class="min-w-[6ch] max-w-full flex-1 truncate font-mono"
            title={item.oldPath && item.oldPath !== item.path
              ? `${item.oldPath} → ${item.path}`
              : item.path}
          >
            {item.oldPath && item.oldPath !== item.path
              ? `${item.oldPath} → ${item.path}`
              : item.path}
          </span>
          {#if isReviewed}
            <Badge variant="secondary" class="max-w-full">
              <span class="truncate">已审阅</span>
            </Badge>
          {/if}
          {#if item.preStagedFromCli}
            <Badge variant="warning" class="max-w-full">
              <span class="truncate">CLI 预暂存</span>
            </Badge>
          {/if}
        </div>
      </ListButton>
    </div>

    {#if !readonlyMode}
      {#if item.group === "staged"}
        <Button
          size="icon-xs"
          variant="ghost"
          disabled={stagingBusy}
          aria-label={`取消暂存文件 ${item.path}`}
          title={`取消暂存文件 ${item.path}`}
          onclick={() => onUnstageChange(item.id)}
        >
          <IconMinus aria-hidden="true" />
        </Button>
      {:else if item.group === "unstaged" || item.group === "untracked"}
        <Button
          size="icon-xs"
          variant="ghost"
          disabled={stagingBusy}
          aria-label={`暂存文件 ${item.path}`}
          title={`暂存文件 ${item.path}`}
          onclick={() => onStageChange(item.id)}
        >
          <IconPlus aria-hidden="true" />
        </Button>
      {/if}
    {/if}
  </div>
{/snippet}

{#snippet changesAndComposerPane()}
  <section
    class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
    aria-label="工作树变更与提交"
  >
    <div
      class="flex min-w-0 items-center justify-between gap-2 border-b border-git-border bg-git-panel px-3 py-1.5"
    >
      <span class="min-w-0 truncate text-xs font-semibold text-foreground">
        变更树（阅读高亮 / 已审阅 / 真实暂存三态独立）
      </span>
      <div class="flex shrink-0 items-center gap-1">
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
    </div>

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
                  <IconChevronDown
                    class="size-3.5 shrink-0"
                    aria-hidden="true"
                  />
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
                  {@render changeRow(entry)}
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
      {onCommitMessageChange}
      {onOpenCommitPreview}
      {onOpenPush}
      {onCreateBranchFromHead}
    />
  </section>
{/snippet}

{#snippet diffPane()}
  <section
    class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
    aria-label="工作树文件差异与代码块暂存"
  >
    {#if !desktop.current}
      <div class="border-b border-git-border bg-git-panel p-2">
        <Button
          size="xs"
          variant="ghost"
          onclick={() => (mobileStep = "changes")}
        >
          <IconArrowLeft aria-hidden="true" />
          返回变更列表
        </Button>
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
          {onDiffModeChange}
          {onStageChange}
          {onUnstageChange}
          {onStageHunk}
          {onUnstageHunk}
          {onRefreshSnapshot}
          {onOpenWorktreeTerminal}
          {onNewWorktreeTerminal}
        />
      {/if}
    </div>
  </section>
{/snippet}

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
      <Resizable.Pane defaultSize={paneSizes[0]} minSize={26} class="git-pane">
        {@render changesAndComposerPane()}
      </Resizable.Pane>
      <Resizable.Handle withHandle class="git-divider" />
      <Resizable.Pane defaultSize={paneSizes[1]} minSize={34} class="git-pane">
        {@render diffPane()}
      </Resizable.Pane>
    </Resizable.PaneGroup>
  {/key}
{:else if mobileStep === "changes"}
  {@render changesAndComposerPane()}
{:else}
  {@render diffPane()}
{/if}
