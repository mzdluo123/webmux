<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type {
    GitDivergenceKind,
    GitDivergencePreview,
  } from "../../contracts/schemas";
  import {
    gitDivergenceConfirmLabel,
    gitDivergenceTitleLabel,
  } from "../../domains/workspace/git-divergence-model";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import BranchCommitList from "./BranchCommitList.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";
  import PreviewIssues from "./PreviewIssues.svelte";

  export interface GitDivergenceDialogProps {
    kind: GitDivergenceKind;
    preview: GitDivergencePreview | null;
    loading?: boolean;
    submitting?: boolean;
    dialogError?: string;
    acknowledgeSessions?: boolean;
    onAcknowledgeSessionsChange?: (checked: boolean) => void;
    onConfirm: () => void;
    onClose: () => void;
  }

  let {
    kind,
    preview,
    loading = false,
    submitting = false,
    dialogError = "",
    acknowledgeSessions = false,
    onAcknowledgeSessionsChange,
    onConfirm,
    onClose,
  }: GitDivergenceDialogProps = $props();

  const effectiveKind = $derived(preview?.kind ?? kind);
  const sessions = $derived(preview?.sessions ?? []);
  const canConfirm = $derived(
    Boolean(
      preview &&
      preview.confirmationId &&
      preview.blockers.length === 0 &&
      !preview.alreadyUpToDate &&
      (sessions.length === 0 || acknowledgeSessions) &&
      !loading &&
      !submitting,
    ),
  );
  const short = (oid: string | undefined) => (oid ? oid.slice(0, 7) : "");
  const commitListTitle = $derived(
    effectiveKind === "rebase"
      ? "将按序重放的当前分支提交"
      : effectiveKind === "merge"
        ? "将合入当前分支的提交"
        : effectiveKind === "cherry_pick"
          ? "将遴选应用的提交"
          : "将生成反向还原的提交",
  );
</script>

<ResponsiveDialog
  title={gitDivergenceTitleLabel(effectiveKind)}
  description={preview
    ? preview.directionSummary
    : "正在读取工作树状态与提交影响范围…"}
  drawerDescription="核对方向、工作树路径与影响提交后确认执行"
  drawerFullScreen
  truncateDrawerTitle
  busy={submitting}
  class="sm:max-w-[min(680px,calc(100vw-2rem))]"
  onclose={onClose}
>
  <div class="min-w-0 space-y-3" data-testid="git-divergence-preview-dialog">
    {#if dialogError}
      <InlineNotice tone="danger">{dialogError}</InlineNotice>
    {/if}

    {#if loading && !preview}
      <p class="py-4 text-xs text-muted-foreground" role="status">
        正在计算操作方向、共同祖先与潜在阻塞项…
      </p>
    {:else if preview}
      <div
        class="min-w-0 space-y-2 rounded-lg border border-git-border bg-git-panel p-3 text-xs"
      >
        <div class="flex min-w-0 flex-wrap items-center gap-1.5">
          {#if preview.fastForward}
            <Badge variant="success" class="max-w-full">
              <span class="truncate">可无冲突快进 (Fast-Forward)</span>
            </Badge>
          {/if}
          {#if preview.alreadyUpToDate}
            <Badge variant="secondary" class="max-w-full">
              <span class="truncate">已包含目标提交，无需执行</span>
            </Badge>
          {/if}
          {#if preview.currentBranch}
            <Badge variant="info" class="max-w-full">
              <span class="truncate" title={preview.currentBranch}>
                {preview.currentBranch}
              </span>
            </Badge>
          {/if}
          <Badge variant="outline" class="max-w-full">
            <span class="truncate" title={preview.targetRef}>
              {preview.targetRef}
            </span>
          </Badge>
        </div>

        <p class="text-sm font-medium wrap-anywhere text-foreground">
          {preview.directionSummary}
        </p>

        <div class="space-y-1 pt-1 text-xs">
          <div>
            <span class="text-muted-foreground">目标工作树：</span>
            <span class="font-mono break-all text-foreground"
              >{preview.worktreePath}</span
            >
          </div>
          {#if preview.headOid}
            <div class="wrap-anywhere">
              <span class="text-muted-foreground">当前 HEAD：</span>
              <span class="font-mono text-foreground"
                >{short(preview.headOid)}</span
              >
              {#if preview.headSubject}
                <span class="text-muted-foreground">
                  · {preview.headSubject}</span
                >
              {/if}
            </div>
          {/if}
          {#if preview.targetOid}
            <div class="wrap-anywhere">
              <span class="text-muted-foreground">目标对象：</span>
              <span class="font-mono text-foreground">{preview.targetRef}</span>
              <span class="font-mono text-muted-foreground">
                ({short(preview.targetOid)})
              </span>
              {#if preview.targetSubject}
                <span class="text-muted-foreground">
                  · {preview.targetSubject}
                </span>
              {/if}
            </div>
          {/if}
          {#if preview.mergeBaseOid}
            <div>
              <span class="text-muted-foreground">共同祖先：</span>
              <span class="font-mono text-muted-foreground"
                >{short(preview.mergeBaseOid)}</span
              >
            </div>
          {/if}
        </div>
      </div>

      <PreviewIssues blockers={preview.blockers} warnings={preview.warnings} />

      {#if preview.commits.total > 0}
        <BranchCommitList title={commitListTitle} commits={preview.commits} />
      {/if}

      {#if preview.recoveryHint}
        <InlineNotice tone="info">{preview.recoveryHint}</InlineNotice>
      {/if}

      <GitOperationSessionAck
        {sessions}
        {acknowledgeSessions}
        disabled={submitting}
        acknowledgementLabel={`我已知晓：以上 ${sessions.length} 个会话将立即观察到工作树文件与索引变化。`}
        {onAcknowledgeSessionsChange}
      />
    {/if}

    <div class="flex flex-wrap justify-end gap-2 pt-1">
      <Button variant="outline" disabled={submitting} onclick={onClose}>
        取消
      </Button>
      <Button
        variant="git-primary"
        wrap
        disabled={!canConfirm}
        onclick={onConfirm}
        data-testid="git-divergence-confirm-btn"
      >
        {#if submitting}
          正在执行…
        {:else}
          {gitDivergenceConfirmLabel(effectiveKind, preview?.fastForward)}
        {/if}
      </Button>
    </div>
  </div>
</ResponsiveDialog>
