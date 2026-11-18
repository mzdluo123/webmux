<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type {
    GitDivergenceKind,
    GitDivergencePreview,
  } from "../../contracts/schemas";
  import {
    gitDivergenceConfirmLabel,
    gitDivergenceTitleLabel,
  } from "../../domains/workspace/git-divergence-model";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import BranchCommitList from "./BranchCommitList.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";
  import GitReviewBody from "./GitReviewBody.svelte";
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
      ? m.s6_divergence_commits_rebase()
      : effectiveKind === "merge"
        ? m.s6_divergence_commits_merge()
        : effectiveKind === "cherry_pick"
          ? m.s6_divergence_commits_cherry_pick()
          : m.s6_divergence_commits_revert(),
  );
</script>

<ResponsiveDialog
  title={gitDivergenceTitleLabel(effectiveKind)}
  description={m.s6_divergence_drawer_desc()}
  drawerFullScreen
  pinDrawerHeader
  truncateDrawerTitle
  busy={submitting}
  class="sm:max-w-[min(680px,calc(100vw-2rem))]"
  onclose={onClose}
>
  <GitReviewBody
    fullScreen
    testId="git-divergence-preview-dialog"
    resetScrollOn={dialogError}
  >
    {#if dialogError}
      <InlineNotice tone="danger">{dialogError}</InlineNotice>
    {/if}

    {#if loading && !preview && !dialogError}
      <p class="py-4 text-xs text-muted-foreground" role="status">
        {m.s6_divergence_loading_body()}
      </p>
    {:else if preview}
      <div
        class="min-w-0 space-y-2 rounded-lg border bg-background p-3 text-xs"
      >
        <div class="flex min-w-0 flex-wrap items-center gap-1.5">
          {#if preview.fastForward}
            <Badge variant="success" class="max-w-full">
              <span class="truncate"
                >{m.s6_divergence_fast_forward_badge()}</span
              >
            </Badge>
          {/if}
          {#if preview.alreadyUpToDate}
            <Badge variant="secondary" class="max-w-full">
              <span class="truncate">{m.s6_divergence_up_to_date_badge()}</span>
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
            <span class="text-muted-foreground"
              >{m.s6_divergence_target_worktree()}</span
            >
            <span class="font-mono break-all text-foreground"
              >{preview.worktreePath}</span
            >
          </div>
          {#if preview.headOid}
            <div class="wrap-anywhere">
              <span class="text-muted-foreground"
                >{m.s6_divergence_current_head()}</span
              >
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
              <span class="text-muted-foreground"
                >{m.s6_divergence_target_ref()}</span
              >
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
              <span class="text-muted-foreground"
                >{m.s6_divergence_merge_base()}</span
              >
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
        acknowledgementLabel={m.s6_divergence_session_ack({
          count: String(sessions.length),
        })}
        {onAcknowledgeSessionsChange}
      />
    {/if}

    {#snippet footer()}
      <div
        class="grid grid-cols-[auto_minmax(0,15rem)] items-end justify-end gap-2 pt-1"
      >
        <Button variant="outline" disabled={submitting} onclick={onClose}>
          {m.s6_cancel()}
        </Button>
        <Button
          variant="default"
          wrap
          disabled={!canConfirm}
          onclick={onConfirm}
          data-testid="git-divergence-confirm-btn"
        >
          {#if submitting}
            {m.s6_executing_ellipsis()}
          {:else}
            {gitDivergenceConfirmLabel(effectiveKind, preview?.fastForward)}
          {/if}
        </Button>
      </div>
    {/snippet}
  </GitReviewBody>
</ResponsiveDialog>
