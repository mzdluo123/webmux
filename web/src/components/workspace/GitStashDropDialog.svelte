<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import type {
    GitStashDropPreview,
    GitStashEntry,
  } from "../../contracts/schemas";
  import { canExecutePreviewedMutation } from "../../domains/workspace/git-v2-adapter";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitRecoveryCommandBox from "./GitRecoveryCommandBox.svelte";

  let {
    target,
    preview,
    loading = false,
    submitting = false,
    dialogError = "",
    onConfirm,
    onClose,
  }: {
    target: GitStashEntry | null;
    preview: GitStashDropPreview | null;
    loading?: boolean;
    submitting?: boolean;
    dialogError?: string;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const entry = $derived(preview?.entry ?? target);
  const recoveryCmd = $derived(
    preview?.recoveryCommand ?? (entry ? `git stash apply ${entry.oid}` : ""),
  );
  const canConfirm = $derived(
    canExecutePreviewedMutation({
      loading,
      submitting,
      confirmationId: preview?.confirmationId,
      blockerCount: preview?.blockers.length ?? 0,
    }),
  );
</script>

<ResponsiveDialog
  title="确认丢弃单条贮藏 (Drop Stash)"
  description="将从 refs/stash 引用日志中移除此条贮藏记录。为防误删，系统不提供一键清空全部贮藏。"
  drawerDescription="核对贮藏对象 OID 与恢复命令后确认丢弃。"
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-stash-drop-dialog">
    {#if entry}
      <div
        class="space-y-1.5 rounded-lg border border-git-border bg-git-panel p-2.5 text-xs"
      >
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge variant="destructive">
            <span class="font-mono">{entry.stashRef}</span>
          </Badge>
          {#if preview}
            <Badge variant="outline">{preview.fileCount} 个文件</Badge>
          {/if}
        </div>
        <p class="font-medium wrap-anywhere text-foreground">{entry.summary}</p>
        <p class="font-mono break-all text-muted-foreground">
          对象 OID：{entry.oid}
        </p>
      </div>
    {/if}

    {#if recoveryCmd}
      <GitRecoveryCommandBox
        label="Git GC 回收前紧急恢复命令："
        copyLabel="复制贮藏恢复命令"
        command={recoveryCmd}
        testId="git-stash-drop-recovery-cmd"
      />
    {/if}

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        正在核验贮藏引用与对象快照…
      </p>
    {:else if preview}
      <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      confirmLabel="确认丢弃此贮藏"
      submittingLabel="正在丢弃…"
      confirmVariant="destructive"
      confirmTestId="git-stash-drop-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
