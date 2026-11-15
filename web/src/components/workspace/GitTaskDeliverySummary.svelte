<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
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
    ? "space-y-2 rounded-md border bg-background p-2.5 text-xs"
    : "shrink-0 space-y-2 border-t bg-background px-3 py-2.5 text-xs"}
  data-testid="git-task-delivery-summary"
>
  <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
    <span class="font-semibold text-foreground">
      {compact ? m.s7_delivery_title_compact() : m.s7_delivery_title_full()}
    </span>
    <div class="flex min-w-0 flex-wrap items-center gap-1">
      <Badge variant="outline"
        >{m.s7_delivery_net_changes({ count: netFileCount })}</Badge
      >
      <Badge variant={commitCount > 0 ? "info" : "outline"}>
        {m.s7_delivery_task_commits({ count: commitCount })}
      </Badge>
      {#if uncommittedCount > 0}
        <Badge variant="warning"
          >{m.s7_delivery_uncommitted({ count: uncommittedCount })}</Badge
        >
      {:else}
        <Badge variant="secondary">{m.s7_delivery_clean()}</Badge>
      {/if}
      {#if context.ahead > 0}
        <Badge variant="info"
          >{m.s7_delivery_ahead({ count: context.ahead })}</Badge
        >
      {/if}
      {#if activeSessionCount > 0}
        <Badge variant="outline"
          >{m.s7_delivery_active_sessions({ count: activeSessionCount })}</Badge
        >
      {/if}
      {#if exitedSessionCount > 0}
        <Badge variant="outline"
          >{m.s7_delivery_exited_sessions({ count: exitedSessionCount })}</Badge
        >
      {/if}
      {#if archivedSessionCount > 0}
        <Badge variant="outline"
          >{m.s7_delivery_archived_sessions({
            count: archivedSessionCount,
          })}</Badge
        >
      {/if}
    </div>
  </div>

  {#if uncommittedCount === 0 && (commitCount > 0 || context.ahead > 0)}
    <p class="leading-relaxed text-muted-foreground">
      {m.s7_delivery_ready_hint()}
    </p>
  {/if}

  <div class="flex min-w-0 flex-wrap items-center gap-1.5">
    {#if compact && onSwitchToTaskReview}
      <Button size="sm" variant="outline" onclick={onSwitchToTaskReview}>
        <IconGitCompare aria-hidden="true" />
        {m.s7_delivery_review_task_btn({ count: netFileCount })}
      </Button>
    {/if}

    {#if onOpenPush && !readonlyMode}
      <Button
        size="sm"
        variant={emphasizePush ? "default" : "outline"}
        disabled={!canPush}
        title={context.checkedOutBranch
          ? m.s7_delivery_open_push_title({ branch: context.checkedOutBranch })
          : m.s7_delivery_push_detached_title()}
        onclick={onOpenPush}
      >
        <IconArrowUp aria-hidden="true" />
        {m.s7_delivery_push_btn()}
      </Button>
    {/if}

    {#if onReturnSession}
      <Button size="sm" variant="outline" onclick={onReturnSession}>
        <IconArrowBackUp aria-hidden="true" />
        {m.s7_delivery_return_session()}
      </Button>
    {/if}

    {#if onArchiveSession && exitedSessionCount > 0}
      <Button size="sm" variant="outline" onclick={onArchiveSession}>
        <IconArchive aria-hidden="true" />
        {m.s7_delivery_archive_session()}
      </Button>
    {/if}

    {#if onRemoveWorktree && context.worktreeKind === "linked"}
      <Button size="sm" variant="outline" onclick={onRemoveWorktree}>
        <IconTrash aria-hidden="true" />
        {m.s7_delivery_remove_worktree()}
      </Button>
    {/if}
  </div>
</div>
