<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import GitFileStatusIcon from "../shared/GitFileStatusIcon.svelte";
  import IconEye from "~icons/tabler/eye";
  import IconEyeCheck from "~icons/tabler/eye-check";
  import IconFlag from "~icons/tabler/flag";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type {
    GitTaskReviewState,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import { emptyTaskReviewState } from "../../hooks/workspace/git-task-review.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
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

  let listEl = $state<HTMLElement | null>(null);

  const review = $derived(taskReview ?? emptyTaskReviewState());
  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );
  const baseline = $derived(review.baseline);
  const baselineLabel = $derived(
    !baseline
      ? m.s7_task_no_baseline_badge()
      : baseline.unborn || !baseline.baseOid
        ? m.s7_unborn_baseline()
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
  aria-label={m.s7_task_pane_aria()}
  data-testid="git-task-review-pane"
>
  <GitPanelToolbar>
    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
      <IconFlag
        class="size-3.5 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
      <span class="text-xs font-semibold text-foreground"
        >{m.s7_task_baseline_heading()}</span
      >
      {#if baseline}
        <Badge variant="info">
          <span class="font-mono">{baselineLabel}</span>
        </Badge>
        <Badge variant="outline">
          {baseline.source === "worktree_create"
            ? m.s7_task_source_auto()
            : m.s7_task_source_manual()}
        </Badge>
      {:else if review.status === "no_task_baseline"}
        <Badge variant="warning">{m.s7_task_not_set_badge()}</Badge>
      {/if}
    </div>

    {#if onRefresh}
      <Button
        size="icon-sm"
        variant="ghost"
        aria-label={m.s7_task_refresh_aria()}
        title={m.s7_task_refresh_aria()}
        onclick={onRefresh}
      >
        <IconRefresh aria-hidden="true" />
      </Button>
    {/if}
  </GitPanelToolbar>

  {#if readonlyMode}
    <div class="p-3">
      <InlineNotice tone="info">
        {m.s7_task_readonly_notice()}
      </InlineNotice>
    </div>
  {:else if review.status === "no_task_baseline"}
    <div class="space-y-2 border-b bg-background p-3 text-xs">
      <InlineNotice tone="info">
        {m.s7_task_no_baseline_notice()}
      </InlineNotice>
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if onSetBaseline}
          <Button
            size="sm"
            variant="default"
            disabled={review.mutating}
            onclick={() => onSetBaseline()}
          >
            {m.s7_task_set_head_btn()}
          </Button>
        {/if}
        {#if onOpenLogToPickBaseline}
          <Button size="sm" variant="outline" onclick={onOpenLogToPickBaseline}>
            {m.s7_task_pick_in_log_btn()}
          </Button>
        {/if}
      </div>
    </div>
  {:else if baseline}
    <div class="space-y-2 border-b bg-background px-3 py-2 text-xs">
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
        <span class="min-w-0 text-muted-foreground">
          {m.s7_task_stats_summary({
            commits: review.commitCount,
            uncommitted: review.uncommittedCount,
            files: review.files.length,
          })}
        </span>
        <div class="flex min-w-0 flex-wrap items-center gap-1.5">
          {#if !baseline.unborn && baseline.baseOid && onInspectCommit}
            <Button
              size="sm"
              variant="ghost"
              onclick={() => onInspectCommit(baseline.baseOid!)}
            >
              {m.s7_task_locate_baseline_btn()}
            </Button>
          {/if}
          {#if onSetBaseline}
            <Button
              size="sm"
              variant="outline"
              disabled={review.mutating}
              onclick={() => onSetBaseline()}
            >
              {m.s7_task_reset_head_btn()}
            </Button>
          {/if}
          {#if onClearBaseline}
            <Button
              size="sm"
              variant="ghost"
              disabled={review.mutating}
              onclick={onClearBaseline}
            >
              {m.s7_task_clear_baseline_btn()}
            </Button>
          {/if}
        </div>
      </div>

      {#if !baseline.cleanAtStart}
        <InlineNotice tone="warning">
          {m.s7_task_dirty_start_warning()}
        </InlineNotice>
      {/if}

      {#if onSearchChange}
        <Input
          size="sm"
          mono
          value={review.searchQuery}
          placeholder={m.s7_task_search_placeholder()}
          aria-label={m.s7_task_search_aria()}
          oninput={(e) => onSearchChange(e.currentTarget.value)}
        />
      {/if}
    </div>
  {/if}

  <!-- Interactive listbox delegates ArrowUp/ArrowDown/Home/End across child option buttons. -->
  <ScrollArea
    bind:viewportRef={listEl}
    class="min-h-8 flex-1"
    viewportProps={{
      role: "listbox",
      "aria-label": m.s7_task_listbox_aria(),
      onkeydown: handleListKeydown,
    }}
  >
    <div class="space-y-0.5 p-2">
      {#if review.status === "loading"}
        <p class="p-3 text-xs text-muted-foreground" role="status">
          {m.s7_task_loading()}
        </p>
      {:else if review.status === "error"}
        <div class="p-2">
          <InlineNotice tone="danger">{review.reason}</InlineNotice>
        </div>
      {:else if review.status === "ready"}
        {#if review.files.length === 0}
          <p class="p-3 text-xs text-muted-foreground">
            {review.searchQuery.trim()
              ? m.s7_task_empty_filtered()
              : m.s7_task_empty_clean()}
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
                  aria-label={m.s7_mark_reviewed_path({ path })}
                  title={isReviewed
                    ? m.s7_reviewed_path({ path })
                    : m.s7_mark_reviewed_path({ path })}
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
                  <div
                    class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5"
                  >
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
                {review.loadingMore
                  ? m.s7_loading_more()
                  : m.s7_task_load_more_btn()}
              </Button>
            </div>
          {/if}
        {/if}
      {/if}
    </div>
  </ScrollArea>

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
