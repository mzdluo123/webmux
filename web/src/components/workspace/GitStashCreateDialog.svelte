<script lang="ts">
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import type { GitStashCreatePreview } from "../../contracts/schemas";
  import {
    canConfirmPreviewModel,
    type StashPreviewDialogBaseProps,
  } from "../../domains/workspace/git-v2-adapter";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";

  let {
    message,
    includeUntracked,
    keepIndex,
    preview,
    loading = false,
    submitting = false,
    acknowledgeSessions = false,
    dialogError = "",
    onMessageChange,
    onIncludeUntrackedChange,
    onKeepIndexChange,
    onAcknowledgeSessionsChange,
    onConfirm,
    onClose,
  }: StashPreviewDialogBaseProps & {
    message: string;
    includeUntracked: boolean;
    keepIndex: boolean;
    preview: GitStashCreatePreview | null;
    onMessageChange: (value: string) => void;
    onIncludeUntrackedChange: (checked: boolean) => void;
    onKeepIndexChange: (checked: boolean) => void;
  } = $props();

  const canConfirm = $derived(
    canConfirmPreviewModel(preview, loading, submitting, acknowledgeSessions),
  );
</script>

<ResponsiveDialog
  title="新建贮藏 (Create Stash)"
  description="将当前工作树的改动打包保存到仓库 refs/stash，清理工作树以便切换任务。"
  drawerDescription="保存当前改动到 refs/stash。"
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-stash-create-dialog">
    <div class="space-y-1.5">
      <Label for="git-stash-message">贮藏备注说明（选填）</Label>
      <Input
        id="git-stash-message"
        value={message}
        placeholder="例如：WIP 登录鉴权重构中…"
        disabled={submitting}
        oninput={(e) => onMessageChange(e.currentTarget.value)}
      />
    </div>

    <div class="space-y-2 rounded-lg border bg-background p-3">
      <Label class="items-start">
        <Checkbox
          checked={includeUntracked}
          disabled={submitting}
          onCheckedChange={(val) => onIncludeUntrackedChange(Boolean(val))}
          class="mt-0.5"
          data-testid="git-stash-include-untracked-checkbox"
        />
        <span class="text-xs leading-snug">
          包含未跟踪文件 (<span class="font-mono">--include-untracked</span
          >，写入第 3 父节点 <span class="font-mono">S^3</span>)
        </span>
      </Label>

      <Label class="items-start">
        <Checkbox
          checked={keepIndex}
          disabled={submitting}
          onCheckedChange={(val) => onKeepIndexChange(Boolean(val))}
          class="mt-0.5"
          data-testid="git-stash-keep-index-checkbox"
        />
        <span class="text-xs leading-snug">
          保留当前暂存区内容 (<span class="font-mono">--keep-index</span
          >，仅清理未暂存工作树修改)
        </span>
      </Label>
    </div>

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        正在计算将要贮藏的文件范围…
      </p>
    {:else if preview}
      <div
        class="grid grid-cols-1 gap-1.5 rounded-lg border bg-background p-3 text-xs sm:grid-cols-2"
      >
        <p class="wrap-anywhere">
          <span class="text-muted-foreground">当前分支：</span>
          <span class="font-mono text-foreground">
            {preview.detached
              ? "游离 HEAD"
              : preview.currentBranch || "未命名分支"}
          </span>
        </p>
        <p class="wrap-anywhere">
          <span class="text-muted-foreground">变更统计：</span>
          <span class="text-foreground">
            暂存 {preview.stagedCount} · 未暂存 {preview.unstagedCount} · 未跟踪 {preview.untrackedCount}
          </span>
        </p>
        <p class="wrap-anywhere sm:col-span-2">
          <span class="text-muted-foreground">预计纳入文件：</span>
          <span class="font-medium text-foreground">
            {preview.affectedPaths.length} 个文件
          </span>
        </p>
      </div>

      <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />

      <GitOperationSessionAck
        sessions={preview.sessions}
        {acknowledgeSessions}
        disabled={submitting}
        acknowledgementLabel="我已知晓当前工作树有活跃会话，保存贮藏会重置工作树文件状态"
        {onAcknowledgeSessionsChange}
      />
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      confirmLabel="确认保存贮藏"
      submittingLabel="正在保存贮藏…"
      confirmTestId="git-stash-create-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
