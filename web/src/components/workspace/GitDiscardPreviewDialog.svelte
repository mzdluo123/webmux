<script lang="ts">
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type { GitDiscardPreview } from "../../contracts/schemas";
  import { canConfirmPreviewModel } from "../../domains/workspace/git-v2-adapter";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";

  let {
    preview,
    loading = false,
    submitting = false,
    staleRejected = false,
    acknowledgeSessions = false,
    dialogError = "",
    onAcknowledgeSessionsChange,
    onRefreshAfterStale,
    onConfirm,
    onClose,
  }: {
    preview: GitDiscardPreview | null;
    loading?: boolean;
    submitting?: boolean;
    staleRejected?: boolean;
    acknowledgeSessions?: boolean;
    dialogError?: string;
    onAcknowledgeSessionsChange: (checked: boolean) => void;
    onRefreshAfterStale?: () => void;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const scopeLabel = $derived.by(() => {
    switch (preview?.scope) {
      case "lines":
        return `所选代码行（${preview.lineCount} 行）`;
      case "hunks":
        return `所选变更块（${preview.hunkCount} 块）`;
      default:
        return `文件级更改（${preview?.items.length ?? 0} 个路径）`;
    }
  });

  const canConfirm = $derived(
    !staleRejected &&
      canConfirmPreviewModel(preview, loading, submitting, acknowledgeSessions),
  );
</script>

<ResponsiveDialog
  title="确认丢弃未暂存更改 (Discard Changes)"
  description="此操作将直接改写工作区文件或删除未跟踪文件（绝不触碰已暂存的 Git index），执行前会自动保存一份安全恢复快照。"
  drawerDescription="核对将覆盖或删除的工作区路径后确认丢弃。"
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-discard-preview-dialog">
    <InlineNotice tone="warning">
      丢弃更改与「取消暂存
      (Unstage)」严格分离：此处仅还原工作区未暂存改动或移除未跟踪文件，已暂存区内容保持不变。服务端在写入磁盘前将再次校验文件哈希（409
      防误覆盖保护），并在
      <span class="font-mono">refs/webmux/discard-recovery/</span>
      保留最多 20 份紧急恢复快照。
    </InlineNotice>

    {#if staleRejected}
      <InlineNotice tone="warning">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>
            自打开预览以来，外部终端或 Agent
            修改了目标文件，已拒绝覆盖并中止写入。
          </span>
          {#if onRefreshAfterStale}
            <Button
              size="sm"
              variant="outline"
              disabled={loading || submitting}
              onclick={onRefreshAfterStale}
              data-testid="git-discard-refresh-stale-btn"
            >
              <IconRefresh aria-hidden="true" />
              刷新最新快照再预览
            </Button>
          {/if}
        </div>
      </InlineNotice>
    {/if}

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        正在核验工作区文件哈希与影响范围…
      </p>
    {:else if preview}
      <div class="space-y-2 rounded-lg border bg-background p-3 text-xs">
        <div class="flex min-w-0 flex-wrap items-center gap-1.5">
          <Badge variant="destructive" class="max-w-full">
            <span class="truncate">破坏性写操作</span>
          </Badge>
          <Badge variant="outline" class="max-w-full">
            <span class="truncate">{scopeLabel}</span>
          </Badge>
        </div>
        <p class="font-medium wrap-anywhere text-foreground">
          {preview.summary}
        </p>

        {#if preview.trackedOverwritePaths.length > 0}
          <div class="space-y-1">
            <p class="font-semibold text-foreground">
              将还原覆盖的已跟踪文件 ({preview.trackedOverwritePaths.length})：
            </p>
            <ul
              class="max-h-28 space-y-0.5 overflow-y-auto rounded-sm border bg-background p-2 font-mono"
              data-testid="git-discard-tracked-list"
            >
              {#each preview.trackedOverwritePaths as p (p)}
                <li class="break-all text-foreground">{p}</li>
              {/each}
            </ul>
          </div>
        {/if}

        {#if preview.untrackedDeletePaths.length > 0}
          <div class="space-y-1">
            <p class="font-semibold text-destructive">
              将从磁盘删除的未跟踪文件 ({preview.untrackedDeletePaths.length})：
            </p>
            <ul
              class="max-h-28 space-y-0.5 overflow-y-auto rounded-sm border bg-background p-2 font-mono"
              data-testid="git-discard-untracked-list"
            >
              {#each preview.untrackedDeletePaths as p (p)}
                <li class="break-all text-destructive">{p}</li>
              {/each}
            </ul>
          </div>
        {/if}
      </div>

      {#if preview.blockers.length > 0 || preview.warnings.length > 0}
        <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />
      {/if}

      <p class="text-muted-foreground" data-testid="git-discard-recovery-note">
        执行前将自动保存有界恢复快照（最多保留最近 20
        次），执行后可在底部操作栏复制恢复命令。
      </p>

      {#if preview.sessions.length > 0}
        <GitOperationSessionAck
          sessions={preview.sessions}
          {acknowledgeSessions}
          disabled={submitting}
          acknowledgementLabel="我已知晓当前工作树有活跃会话，丢弃更改将直接改写或删除上述工作树文件"
          {onAcknowledgeSessionsChange}
        />
      {/if}
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      wrap
      confirmLabel="确认丢弃更改"
      submittingLabel="正在安全备份并丢弃…"
      confirmVariant="destructive"
      confirmTestId="git-discard-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
