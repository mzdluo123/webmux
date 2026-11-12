<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { computeGitGraphLayout } from "../../domains/workspace/git-graph-layout";
  import { syncElementScrollTop } from "../../domains/workspace/git-workbench-memory";
  import type {
    GitCompareViewState,
    GitHistoryRow,
    GitHistoryStatus,
    GitTaskBaseline,
  } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitLogCommitRow from "./GitLogCommitRow.svelte";
  import GitLogFilterBar from "./GitLogFilterBar.svelte";
  import type {
    GitLogFilterCallbacks,
    GitLogFilterFields,
  } from "./git-log-pane-types";

  let {
    desktop = true,
    historyState,
    historyReason = "",
    taskBaseline = null,
    nextCursor = "",
    historyLoadingMore = false,
    visibleCommits,
    selectedCommitOid,
    compareMode = null,
    showBranchesPanel = true,
    showAuthorColumn = true,
    listScrollTop = 0,
    onListScrollChange,
    onOpenRefDrawer,
    onToggleBranchesPanel,
    onResetLayout,
    onLoadMoreHistory,
    onSelectCommit,
    onMarkCompareBase,
    onStartCompare,
    onSetTaskBaseline,
    onCherryPickCommit,
    onRevertCommit,
    ...filterProps
  }: GitLogFilterFields &
    GitLogFilterCallbacks & {
      desktop?: boolean;
      historyState: GitHistoryStatus;
      historyReason?: string;
      taskBaseline?: GitTaskBaseline | null;
      nextCursor?: string;
      historyLoadingMore?: boolean;
      visibleCommits: readonly GitHistoryRow[];
      selectedCommitOid: string | null;
      compareMode?: GitCompareViewState | null;
      showBranchesPanel?: boolean;
      showAuthorColumn?: boolean;
      listScrollTop?: number;
      onListScrollChange?: (top: number) => void;
      onOpenRefDrawer?: () => void;
      onToggleBranchesPanel?: () => void;
      onResetLayout?: () => void;
      onLoadMoreHistory?: () => void;
      onSelectCommit: (oid: string) => void;
      onMarkCompareBase?: (oid: string | null) => void;
      onStartCompare?: (baseOid: string, targetOid: string) => void;
      onSetTaskBaseline?: (baseOid?: string) => void;
      onCherryPickCommit?: (oid: string) => void;
      onRevertCommit?: (oid: string) => void;
    } = $props();

  let listScrollEl = $state<HTMLDivElement>();
  $effect(() => syncElementScrollTop(listScrollEl, listScrollTop));

  const graphLayouts = $derived(computeGitGraphLayout(visibleCommits));

  function focusCommitRow(index: number) {
    const rows =
      listScrollEl?.querySelectorAll<HTMLButtonElement>("[data-commit-row]");
    rows?.[index]?.focus();
  }

  function handleListKeydown(e: KeyboardEvent) {
    if (visibleCommits.length === 0) return;
    if (!(e.target instanceof HTMLElement)) return;
    if (!e.target.hasAttribute("data-commit-row")) return;

    const currentIdx = Math.max(
      0,
      visibleCommits.findIndex((c) => c.oid === selectedCommitOid),
    );
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIdx = Math.min(visibleCommits.length - 1, currentIdx + 1);
      onSelectCommit(visibleCommits[nextIdx]!.oid);
      focusCommitRow(nextIdx);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIdx = Math.max(0, currentIdx - 1);
      onSelectCommit(visibleCommits[prevIdx]!.oid);
      focusCommitRow(prevIdx);
    } else if (e.key === "Home") {
      e.preventDefault();
      onSelectCommit(visibleCommits[0]!.oid);
      focusCommitRow(0);
    } else if (e.key === "End") {
      e.preventDefault();
      const lastIdx = visibleCommits.length - 1;
      onSelectCommit(visibleCommits[lastIdx]!.oid);
      focusCommitRow(lastIdx);
    }
  }

  function describeHistoryStatus(status: GitHistoryStatus): string {
    if (historyReason) return historyReason;
    switch (status) {
      case "no_worktree":
        return "当前未绑定可用工作树，无法解析「当前分支 / 当前任务 / 待推送 / 远端新增」范围；请切换至「所有历史」或在顶部选择工作树。";
      case "no_task_baseline":
        return "当前工作树尚未设置任务起点基线；可将当前 HEAD 设为任务起点，或在「所有历史」中任选历史提交菜单设为任务基线。";
      case "detached_no_branch":
        return "当前工作树处于游离 HEAD 状态，无关联本地分支，无法计算分支专属范围。";
      case "no_upstream":
        return "当前分支尚未关联远程上游分支（Upstream），无法计算待推送或远端新增提交。";
      case "upstream_gone":
        return "当前分支关联的远程上游分支已不存在（gone）。";
      case "unborn":
        return "当前分支尚未产生首个提交（unborn HEAD）。";
      default:
        return "当前范围暂无提交记录。";
    }
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
  aria-label="提交历史列表"
>
  <GitLogFilterBar
    {desktop}
    {...filterProps}
    {showBranchesPanel}
    {onOpenRefDrawer}
    {onToggleBranchesPanel}
    {onResetLayout}
    onCancelCompareMark={onMarkCompareBase
      ? () => onMarkCompareBase(null)
      : undefined}
  />

  {#if filterProps.scope === "task" && taskBaseline}
    <div
      class="flex min-w-0 flex-wrap items-center justify-between gap-1.5 border-b border-git-border bg-git-panel px-2.5 py-1.5 text-xs"
    >
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        <span class="text-muted-foreground">任务起点：</span>
        <Badge variant="info">
          <span class="font-mono">
            {taskBaseline.unborn || !taskBaseline.baseOid
              ? "空仓起点"
              : taskBaseline.baseOid.slice(0, 7)}
          </span>
        </Badge>
        <Badge variant="outline">
          {taskBaseline.source === "worktree_create"
            ? "新建工作树自动记录"
            : "手动指定起点"}
        </Badge>
        {#if !taskBaseline.cleanAtStart}
          <Badge variant="warning">含设定前已存在修改</Badge>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Interactive listbox delegates ArrowUp/ArrowDown/Home/End across child option buttons. -->
  <!-- svelte-ignore a11y_interactive_supports_focus -->
  <div
    bind:this={listScrollEl}
    class="min-h-0 flex-1 overflow-y-auto p-1.5"
    role="listbox"
    aria-label="提交记录"
    onkeydown={handleListKeydown}
    onscroll={(e) => {
      const el = e.currentTarget;
      onListScrollChange?.(el.scrollTop);
      if (
        nextCursor &&
        !historyLoadingMore &&
        onLoadMoreHistory &&
        el.scrollHeight - el.scrollTop - el.clientHeight < 120
      ) {
        onLoadMoreHistory();
      }
    }}
  >
    {#if historyState !== "ready"}
      <div class="p-2">
        <InlineNotice
          tone={historyState === "upstream_gone" ? "warning" : "info"}
        >
          <span class="mr-2">{describeHistoryStatus(historyState)}</span>
          {#if historyState === "no_task_baseline" && onSetTaskBaseline}
            <Button
              size="xs"
              variant="outline"
              onclick={() => onSetTaskBaseline()}
            >
              将当前 HEAD 设为任务起点
            </Button>
          {/if}
        </InlineNotice>
      </div>
    {:else if visibleCommits.length === 0}
      <p class="p-3 text-xs text-muted-foreground">
        没有匹配筛选条件的提交记录。
      </p>
    {:else}
      {#each visibleCommits as row, idx (row.oid)}
        <GitLogCommitRow
          {row}
          layout={graphLayouts[idx]}
          selected={selectedCommitOid === row.oid}
          {selectedCommitOid}
          compareBaseMarkOid={filterProps.compareBaseMarkOid}
          taskBaselineOid={taskBaseline && !taskBaseline.unborn
            ? (taskBaseline.baseOid ?? null)
            : null}
          {compareMode}
          {showAuthorColumn}
          onSelect={onSelectCommit}
          {onMarkCompareBase}
          {onStartCompare}
          {onSetTaskBaseline}
          {onCherryPickCommit}
          {onRevertCommit}
        />
      {/each}
    {/if}
  </div>

  {#if historyState === "ready" && nextCursor && onLoadMoreHistory}
    <div
      class="flex shrink-0 items-center justify-between gap-2 border-t border-git-border bg-git-panel px-2.5 py-1.5"
    >
      <span class="text-xs text-muted-foreground">
        已加载 {visibleCommits.length} 条提交
      </span>
      <Button
        size="xs"
        variant="outline"
        disabled={historyLoadingMore}
        onclick={onLoadMoreHistory}
      >
        {historyLoadingMore ? "正在加载更多历史…" : "加载更多历史"}
      </Button>
    </div>
  {/if}
</section>
