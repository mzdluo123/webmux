<script lang="ts">
  import GitFileStatusIcon from "../shared/GitFileStatusIcon.svelte";
  import IconEye from "~icons/tabler/eye";
  import IconEyeCheck from "~icons/tabler/eye-check";
  import IconFlag from "~icons/tabler/flag";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import type {
    GitTaskReviewState,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import { emptyTaskReviewState } from "../../hooks/workspace/git-task-review.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitPanelToolbar from "./GitPanelToolbar.svelte";
  import GitTaskDeliverySummary from "./GitTaskDeliverySummary.svelte";

  let {
    context,
    taskReview,
    reviewedPaths = [],
    activeSessionCount = 0,
    exitedSessionCount = 0,
    archivedSessionCount = 0,
    onSelectFile,
    onSearchChange,
    onLoadMore,
    onRefresh,
    onSetBaseline,
    onClearBaseline,
    onOpenLogToPickBaseline,
    onInspectCommit,
    onToggleReviewed,
    onReturnSession,
    onOpenPush,
    onArchiveSession,
    onRemoveWorktree,
  }: {
    context: GitWorktreeContext;
    taskReview?: GitTaskReviewState;
    reviewedPaths?: readonly string[];
    activeSessionCount?: number;
    exitedSessionCount?: number;
    archivedSessionCount?: number;
    onSelectFile?: (path: string) => void;
    onSearchChange?: (q: string) => void;
    onLoadMore?: () => void;
    onRefresh?: () => void;
    onSetBaseline?: (baseOid?: string) => void;
    onClearBaseline?: () => void;
    onOpenLogToPickBaseline?: () => void;
    onInspectCommit?: (oid: string) => void;
    onToggleReviewed?: (path: string) => void;
    onReturnSession?: () => void;
    onOpenPush?: () => void;
    onArchiveSession?: () => void;
    onRemoveWorktree?: () => void;
  } = $props();

  let listEl = $state<HTMLDivElement>();

  const review = $derived(taskReview ?? emptyTaskReviewState());
  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );
  const baseline = $derived(review.baseline);
  const baselineLabel = $derived(
    !baseline
      ? "未设起点"
      : baseline.unborn || !baseline.baseOid
        ? "空仓起点"
        : baseline.baseOid.slice(0, 7),
  );
  const activeFile = $derived(
    review.files.find(
      (f) => (f.newPath || f.oldPath) === review.selectedPath,
    ) ??
      review.files[0] ??
      null,
  );

  function isPathReviewed(path: string): boolean {
    return (
      reviewedPaths.includes(`task:${path}`) || reviewedPaths.includes(path)
    );
  }

  function focusTaskRow(index: number) {
    const rows = listEl?.querySelectorAll<HTMLButtonElement>(
      "[data-task-file-row]",
    );
    rows?.[index]?.focus();
  }

  function handleListKeydown(e: KeyboardEvent) {
    if (review.files.length === 0) return;
    if (!(e.target instanceof HTMLElement)) return;
    if (!e.target.hasAttribute("data-task-file-row")) return;

    const activePath = activeFile
      ? activeFile.newPath || activeFile.oldPath
      : null;
    const currentIdx = Math.max(
      0,
      review.files.findIndex((f) => (f.newPath || f.oldPath) === activePath),
    );
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIdx = Math.min(review.files.length - 1, currentIdx + 1);
      const nextPath =
        review.files[nextIdx]!.newPath || review.files[nextIdx]!.oldPath;
      onSelectFile?.(nextPath);
      focusTaskRow(nextIdx);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIdx = Math.max(0, currentIdx - 1);
      const prevPath =
        review.files[prevIdx]!.newPath || review.files[prevIdx]!.oldPath;
      onSelectFile?.(prevPath);
      focusTaskRow(prevIdx);
    } else if (e.key === "Home") {
      e.preventDefault();
      const firstPath = review.files[0]!.newPath || review.files[0]!.oldPath;
      onSelectFile?.(firstPath);
      focusTaskRow(0);
    } else if (e.key === "End") {
      e.preventDefault();
      const lastIdx = review.files.length - 1;
      const lastPath =
        review.files[lastIdx]!.newPath || review.files[lastIdx]!.oldPath;
      onSelectFile?.(lastPath);
      focusTaskRow(lastIdx);
    }
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label="相对任务起点的净变更"
  data-testid="git-task-review-pane"
>
  <GitPanelToolbar>
    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
      <IconFlag
        class="size-3.5 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
      <span class="text-xs font-semibold text-foreground">任务起点基线</span>
      {#if baseline}
        <Badge variant="info">
          <span class="font-mono">{baselineLabel}</span>
        </Badge>
        <Badge variant="outline">
          {baseline.source === "worktree_create"
            ? "新建工作树自动记录"
            : "手动指定起点"}
        </Badge>
      {:else if review.status === "no_task_baseline"}
        <Badge variant="warning">未设置</Badge>
      {/if}
    </div>

    {#if onRefresh}
      <Button
        size="icon-sm"
        variant="ghost"
        aria-label="刷新相对任务起点的变化"
        title="刷新相对任务起点的变化"
        onclick={onRefresh}
      >
        <IconRefresh aria-hidden="true" />
      </Button>
    {/if}
  </GitPanelToolbar>

  {#if readonlyMode}
    <div class="p-3">
      <InlineNotice tone="info">
        当前处于无工作树只读仓库上下文，无法绑定任务审阅基线。请切换到具体工作树。
      </InlineNotice>
    </div>
  {:else if review.status === "no_task_baseline"}
    <div class="space-y-2 border-b bg-background p-3 text-xs">
      <InlineNotice tone="info">
        当前工作树尚未记录任务起点基线。新建任务工作树会自动捕获起点；存量工作树可将当前
        HEAD 或日志中的历史提交手动指定为任务起点。
      </InlineNotice>
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if onSetBaseline}
          <Button
            size="sm"
            variant="default"
            disabled={review.mutating}
            onclick={() => onSetBaseline()}
          >
            将当前 HEAD 设为任务起点
          </Button>
        {/if}
        {#if onOpenLogToPickBaseline}
          <Button size="sm" variant="outline" onclick={onOpenLogToPickBaseline}>
            前往日志选择历史提交起点
          </Button>
        {/if}
      </div>
    </div>
  {:else if baseline}
    <div class="space-y-2 border-b bg-background px-3 py-2 text-xs">
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
        <span class="min-w-0 text-muted-foreground">
          任务提交 {review.commitCount} 笔 · 当前未提交 {review.uncommittedCount}
          项 · 单份净变更 {review.files.length} 个文件
        </span>
        <div class="flex min-w-0 flex-wrap items-center gap-1.5">
          {#if !baseline.unborn && baseline.baseOid && onInspectCommit}
            <Button
              size="sm"
              variant="ghost"
              onclick={() => onInspectCommit(baseline.baseOid!)}
            >
              定位起点提交
            </Button>
          {/if}
          {#if onSetBaseline}
            <Button
              size="sm"
              variant="outline"
              disabled={review.mutating}
              onclick={() => onSetBaseline()}
            >
              重置为当前 HEAD
            </Button>
          {/if}
          {#if onClearBaseline}
            <Button
              size="sm"
              variant="ghost"
              disabled={review.mutating}
              onclick={onClearBaseline}
            >
              清除基线
            </Button>
          {/if}
        </div>
      </div>

      {#if !baseline.cleanAtStart}
        <InlineNotice tone="warning">
          相对所选提交的全部变化（含设定前已存在修改）
        </InlineNotice>
      {/if}

      {#if onSearchChange}
        <Input
          size="sm"
          mono
          value={review.searchQuery}
          placeholder="按路径筛选任务净变更…"
          aria-label="按路径筛选任务净变更"
          oninput={(e) => onSearchChange(e.currentTarget.value)}
        />
      {/if}
    </div>
  {/if}

  <!-- Interactive listbox delegates ArrowUp/ArrowDown/Home/End across child option buttons. -->
  <!-- svelte-ignore a11y_interactive_supports_focus -->
  <div
    bind:this={listEl}
    class="min-h-0 flex-1 space-y-0.5 overflow-y-auto p-2"
    role="listbox"
    aria-label="相对任务起点的净变更文件"
    onkeydown={handleListKeydown}
  >
    {#if review.status === "loading"}
      <p class="p-3 text-xs text-muted-foreground" role="status">
        正在读取相对任务起点的单份净变更…
      </p>
    {:else if review.status === "error"}
      <div class="p-2">
        <InlineNotice tone="danger">{review.reason}</InlineNotice>
      </div>
    {:else if review.status === "ready"}
      {#if review.files.length === 0}
        <p class="p-3 text-xs text-muted-foreground">
          {review.searchQuery.trim()
            ? "没有匹配路径筛选条件的任务变更文件。"
            : "相对任务起点尚无净文件变化（若文件修改后又还原，净差异也会归零）。"}
        </p>
      {:else}
        {#each review.files as file (file.newPath || file.oldPath)}
          {@const path = file.newPath || file.oldPath}
          {@const isReviewed = isPathReviewed(path)}
          {@const isSelected =
            (activeFile?.newPath || activeFile?.oldPath) === path}
          <div
            class="flex min-w-0 items-center gap-1 rounded-sm"
            data-task-file={path}
            data-reviewed={isReviewed}
          >
            {#if onToggleReviewed}
              <ListButton
                variant="review-toggle"
                aria-pressed={isReviewed}
                aria-label={`标记已审阅 ${path}`}
                title={isReviewed ? `已审阅 ${path}` : `标记已审阅 ${path}`}
                onclick={() => onToggleReviewed(path)}
              >
                {#if isReviewed}<IconEyeCheck
                    aria-hidden="true"
                  />{:else}<IconEye aria-hidden="true" />{/if}
              </ListButton>
            {/if}
            <div class="min-w-0 flex-1">
              <ListButton
                variant="compact-row"
                role="option"
                data-task-file-row="true"
                aria-selected={isSelected}
                onclick={() => onSelectFile?.(path)}
              >
                <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
                  <GitFileStatusIcon status={file.status} />
                  <span
                    class="min-w-[6ch] max-w-full flex-1 truncate font-mono"
                    title={file.oldPath && file.oldPath !== path
                      ? `${file.oldPath} → ${path}`
                      : path}
                  >
                    {file.oldPath && file.oldPath !== path
                      ? `${file.oldPath} → ${path}`
                      : path}
                  </span>
                </div>
              </ListButton>
            </div>
          </div>
        {/each}

        {#if review.nextCursor && onLoadMore}
          <div class="pt-2">
            <Button
              size="sm"
              variant="outline"
              disabled={review.loadingMore}
              onclick={onLoadMore}
            >
              {review.loadingMore ? "正在加载更多…" : "加载更多净变更文件"}
            </Button>
          </div>
        {/if}
      {/if}
    {/if}
  </div>

  {#if !readonlyMode && review.status === "ready"}
    <GitTaskDeliverySummary
      {context}
      commitCount={review.commitCount}
      uncommittedCount={review.uncommittedCount}
      netFileCount={review.files.length}
      {activeSessionCount}
      {exitedSessionCount}
      {archivedSessionCount}
      {onReturnSession}
      {onOpenPush}
      {onArchiveSession}
      {onRemoveWorktree}
    />
  {/if}
</section>
