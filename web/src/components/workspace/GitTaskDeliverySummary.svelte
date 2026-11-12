<script lang="ts">
  import IconArchive from "~icons/tabler/archive";
  import IconArrowBackUp from "~icons/tabler/arrow-back-up";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconGitCompare from "~icons/tabler/git-compare";
  import IconTrash from "~icons/tabler/trash";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type { GitWorktreeContext } from "../../contracts/git-workbench-schemas";

  let {
    context,
    commitCount = 0,
    uncommittedCount = 0,
    netFileCount = 0,
    activeSessionCount = 0,
    exitedSessionCount = 0,
    archivedSessionCount = 0,
    compact = false,
    onSwitchToTaskReview,
    onReturnSession,
    onOpenPush,
    onArchiveSession,
    onRemoveWorktree,
  }: {
    context: GitWorktreeContext;
    commitCount?: number;
    uncommittedCount?: number;
    netFileCount?: number;
    activeSessionCount?: number;
    exitedSessionCount?: number;
    archivedSessionCount?: number;
    compact?: boolean;
    onSwitchToTaskReview?: () => void;
    onReturnSession?: () => void;
    onOpenPush?: () => void;
    onArchiveSession?: () => void;
    onRemoveWorktree?: () => void;
  } = $props();

  const readonlyMode = $derived(
    context.headState === "readonly" || !context.worktreePath,
  );
  const canPush = $derived(
    !readonlyMode && Boolean(context.checkedOutBranch) && Boolean(onOpenPush),
  );
  const emphasizePush = $derived(
    !compact &&
      uncommittedCount === 0 &&
      (commitCount > 0 || context.ahead > 0),
  );
</script>

<div
  class={compact
    ? "space-y-2 rounded-md border border-git-border bg-git-panel p-2.5 text-xs"
    : "shrink-0 space-y-2 border-t border-git-border bg-git-panel px-3 py-2.5 text-xs"}
  data-testid="git-task-delivery-summary"
>
  <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
    <span class="font-semibold text-foreground">
      {compact ? "当前工作区无未提交改动 · 任务交付概览" : "任务交付与收尾摘要"}
    </span>
    <div class="flex min-w-0 flex-wrap items-center gap-1">
      <Badge variant="outline">净变更 {netFileCount} 项</Badge>
      <Badge variant={commitCount > 0 ? "info" : "outline"}>
        任务提交 {commitCount} 笔
      </Badge>
      {#if uncommittedCount > 0}
        <Badge variant="warning">未提交 {uncommittedCount} 项</Badge>
      {:else}
        <Badge variant="secondary">工作区干净</Badge>
      {/if}
      {#if context.ahead > 0}
        <Badge variant="info">待推送 {context.ahead} 笔</Badge>
      {/if}
      {#if activeSessionCount > 0}
        <Badge variant="outline">运行会话 {activeSessionCount}</Badge>
      {/if}
      {#if exitedSessionCount > 0}
        <Badge variant="outline">已结束会话 {exitedSessionCount}</Badge>
      {/if}
      {#if archivedSessionCount > 0}
        <Badge variant="outline">已归档 {archivedSessionCount}</Badge>
      {/if}
    </div>
  </div>

  {#if uncommittedCount === 0 && (commitCount > 0 || context.ahead > 0)}
    <p class="leading-relaxed text-muted-foreground">
      Agent
      或命令行提交已准备就绪；推送不会自动移除工作树，可按需返回会话、推送分支或手动安全清理。
    </p>
  {/if}

  <div class="flex min-w-0 flex-wrap items-center gap-1.5">
    {#if compact && onSwitchToTaskReview}
      <Button size="xs" variant="git-primary" onclick={onSwitchToTaskReview}>
        <IconGitCompare aria-hidden="true" />
        审阅相对任务起点的变化 ({netFileCount})
      </Button>
    {/if}

    {#if onOpenPush && !readonlyMode}
      <Button
        size="xs"
        variant={emphasizePush ? "git-primary" : "outline"}
        disabled={!canPush}
        title={context.checkedOutBranch
          ? `打开 ${context.checkedOutBranch} 推送预览`
          : "游离或未诞生 HEAD 请先创建分支后再推送"}
        onclick={onOpenPush}
      >
        <IconArrowUp aria-hidden="true" />
        推送提交…
      </Button>
    {/if}

    {#if onReturnSession}
      <Button size="xs" variant="outline" onclick={onReturnSession}>
        <IconArrowBackUp aria-hidden="true" />
        返回会话
      </Button>
    {/if}

    {#if onArchiveSession && exitedSessionCount > 0}
      <Button size="xs" variant="outline" onclick={onArchiveSession}>
        <IconArchive aria-hidden="true" />
        归档已完成会话
      </Button>
    {/if}

    {#if onRemoveWorktree && context.worktreeKind === "linked"}
      <Button size="xs" variant="outline" onclick={onRemoveWorktree}>
        <IconTrash aria-hidden="true" />
        移除工作树…
      </Button>
    {/if}
  </div>
</div>
