<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import type { GitBranchRenamePreview } from "../../contracts/schemas";
  import { canExecutePreviewedMutation } from "../../domains/workspace/git-v2-adapter";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";

  let {
    oldName,
    newName,
    preview,
    loading = false,
    submitting = false,
    acknowledgeSessions = false,
    dialogError = "",
    onNewNameChange,
    onAcknowledgeSessionsChange,
    onConfirm,
    onClose,
  }: {
    oldName: string;
    newName: string;
    preview: GitBranchRenamePreview | null;
    loading?: boolean;
    submitting?: boolean;
    acknowledgeSessions?: boolean;
    dialogError?: string;
    onNewNameChange: (val: string) => void;
    onAcknowledgeSessionsChange: (checked: boolean) => void;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const canConfirm = $derived(
    Boolean(newName.trim()) &&
      newName.trim() !== oldName &&
      canExecutePreviewedMutation({
        loading,
        submitting,
        confirmationId: preview?.confirmationId,
        blockerCount: preview?.blockers.length ?? 0,
        sessionCount: preview?.sessions.length ?? 0,
        acknowledgeSessions,
      }),
  );
</script>

<ResponsiveDialog
  title={`重命名本地分支 · ${oldName}`}
  description="两步预览并执行 git branch -m，自动迁移该分支关联的工作树 HEAD 与本地上游跟踪配置。"
  drawerDescription="核对新分支名称与受影响工作树后确认重命名。"
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-branch-rename-dialog">
    <div class="space-y-1.5">
      <Label for="git-branch-rename-input">新本地分支名称</Label>
      <Input
        id="git-branch-rename-input"
        mono
        value={newName}
        placeholder="feat/new-branch-name"
        disabled={submitting}
        oninput={(e) => onNewNameChange(e.currentTarget.value)}
        data-testid="git-branch-rename-input"
      />
    </div>

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        正在校验分支引用格式与工作树影响范围…
      </p>
    {:else if preview}
      <div
        class="space-y-1.5 rounded-lg border border-git-border bg-git-panel p-3 text-xs"
      >
        <p class="wrap-anywhere">
          <span class="text-muted-foreground">重命名变更：</span>
          <span class="font-mono text-foreground">{preview.oldName}</span>
          →
          <span class="font-mono font-semibold text-foreground">
            {preview.newName}
          </span>
        </p>
        <p class="wrap-anywhere">
          <span class="text-muted-foreground">指向提交：</span>
          <span class="font-mono text-foreground">
            {preview.headOid.slice(0, 7)}
          </span>
          · {preview.subject}
        </p>
        {#if preview.upstream}
          <p class="wrap-anywhere">
            <span class="text-muted-foreground">上游跟踪配置：</span>
            将保留对
            <span class="font-mono text-foreground">{preview.upstream}</span>
            的跟踪（远端分支本身不会在远端重命名）
          </p>
        {/if}
        {#if preview.affectedWorktrees.length > 0}
          <div class="pt-1">
            <span class="text-muted-foreground">同步更新检出工作树：</span>
            <div class="mt-1 flex flex-wrap gap-1">
              {#each preview.affectedWorktrees as wt (wt.id)}
                <Badge variant="info">
                  <span class="font-mono">{wt.path}</span>
                </Badge>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />

      <GitOperationSessionAck
        sessions={preview.sessions}
        {acknowledgeSessions}
        disabled={submitting}
        acknowledgementLabel="我已知晓当前工作树有活跃会话，分支重命名将同步更新其检出分支名"
        {onAcknowledgeSessionsChange}
      />
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      confirmLabel="确认重命名分支"
      submittingLabel="正在重命名…"
      confirmTestId="git-branch-rename-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
