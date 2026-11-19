<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type {
    GitCommitOutcome,
    GitPrecedingCommit,
    GitWorkbenchViewState,
  } from "../../contracts/git-workbench-schemas";
  import { applyShellCommitConfirm } from "../../domains/workspace/git-shell-actions";
  import {
    applyShellDiscardConfirm,
    type PendingShellDiscard,
  } from "../../domains/workspace/git-shell-v3-actions";
  import GitAbortOperationDialog from "./GitAbortOperationDialog.svelte";
  import GitCommitPreviewDialog from "./GitCommitPreviewDialog.svelte";
  import GitDiscardPreviewDialog from "./GitDiscardPreviewDialog.svelte";
  import GitDivergencePreviewDialog from "./GitDivergencePreviewDialog.svelte";
  import GitFileInspectorDialog from "./GitFileInspectorDialog.svelte";
  import GitPushDialog from "./GitPushDialog.svelte";
  import GitStashApplyDialog from "./GitStashApplyDialog.svelte";
  import GitStashCreateDialog from "./GitStashCreateDialog.svelte";
  import GitStashDropDialog from "./GitStashDropDialog.svelte";

  let {
    view = $bindable(),
    pendingDiscard = $bindable(null),
    simulatedCommitOutcome = null,
    acknowledgeSessions = $bindable(false),
    onOpenFollowUpPush,
  }: {
    view: GitWorkbenchViewState;
    pendingDiscard?: PendingShellDiscard | null;
    simulatedCommitOutcome?: GitCommitOutcome | null;
    acknowledgeSessions?: boolean;
    onOpenFollowUpPush: (branch: string, preceding: GitPrecedingCommit) => void;
  } = $props();

  function confirmCommit() {
    const res = applyShellCommitConfirm(view, simulatedCommitOutcome);
    if (!res) return;
    view = res.nextView;
    if (res.followUpPush) {
      onOpenFollowUpPush(res.followUpPush.branch, res.followUpPush.preceding);
    }
  }
</script>

{#if view.commitPreview}
  <GitCommitPreviewDialog
    model={view.commitPreview}
    commitMessage={view.commitMessage}
    onConfirm={confirmCommit}
    onClose={() => (view = { ...view, commitPreview: null })}
  />
{/if}

{#if view.pushPreview}
  <GitPushDialog
    model={view.pushPreview}
    onConfirm={() => {
      const push = view.pushPreview;
      if (!push) return;
      view = {
        ...view,
        pushPreview: null,
        statusBanner: m.s7_shell_push_confirmed({
          src: push.sourceBranch,
          remote: push.remote,
          dst: push.targetBranch,
        }),
      };
    }}
    onClose={() => {
      const hadPreceding = view.pushPreview?.precedingCommit;
      view = {
        ...view,
        pushPreview: null,
        statusBanner: hadPreceding
          ? m.s7_shell_push_cancelled_after_commit({
              oid: hadPreceding.shortOid,
            })
          : view.statusBanner,
      };
    }}
    onMergeRemoteBranch={(remoteRef) =>
      (view = {
        ...view,
        pushPreview: null,
        statusBanner: m.s7_shell_merge_remote_opened({ remoteRef }),
      })}
    onRebaseOntoRemoteBranch={(remoteRef) =>
      (view = {
        ...view,
        pushPreview: null,
        statusBanner: m.s7_shell_rebase_remote_opened({ remoteRef }),
      })}
  />
{/if}

{#if view.divergencePreview}
  <GitDivergencePreviewDialog
    kind={view.divergencePreview.kind}
    preview={view.divergencePreview}
    {acknowledgeSessions}
    onAcknowledgeSessionsChange={(checked) => (acknowledgeSessions = checked)}
    onConfirm={() =>
      (view = {
        ...view,
        divergencePreview: null,
        statusBanner: m.s7_shell_divergence_confirmed(),
      })}
    onClose={() => (view = { ...view, divergencePreview: null })}
  />
{/if}

{#if view.abortPreview}
  <GitAbortOperationDialog
    abortPreview={view.abortPreview}
    acknowledged={acknowledgeSessions}
    onAcknowledgeChange={(checked) => (acknowledgeSessions = checked)}
    onConfirmAbort={() =>
      (view = {
        ...view,
        abortPreview: null,
        inProgress: null,
        conflictOpen: false,
        statusBanner: m.s7_shell_abort_confirmed(),
      })}
    onDismiss={() => (view = { ...view, abortPreview: null })}
  />
{/if}

{#if view.stashCreatePreview}
  <GitStashCreateDialog
    message=""
    includeUntracked={true}
    keepIndex={false}
    preview={view.stashCreatePreview}
    {acknowledgeSessions}
    onMessageChange={() => {}}
    onIncludeUntrackedChange={() => {}}
    onKeepIndexChange={() => {}}
    onAcknowledgeSessionsChange={(checked) => (acknowledgeSessions = checked)}
    onConfirm={() =>
      (view = {
        ...view,
        stashCreatePreview: null,
        statusBanner: m.s7_shell_stash_create_confirmed(),
      })}
    onClose={() => (view = { ...view, stashCreatePreview: null })}
  />
{/if}

{#if view.stashApplyPreview}
  <GitStashApplyDialog
    mode={view.stashApplyPreview.mode}
    reinstateIndex={view.stashApplyPreview.reinstateIndex}
    target={view.stashApplyPreview.entry}
    preview={view.stashApplyPreview}
    {acknowledgeSessions}
    onModeChange={(mode) =>
      view.stashApplyPreview &&
      (view = {
        ...view,
        stashApplyPreview: { ...view.stashApplyPreview, mode },
      })}
    onReinstateIndexChange={(reinstateIndex) =>
      view.stashApplyPreview &&
      (view = {
        ...view,
        stashApplyPreview: { ...view.stashApplyPreview, reinstateIndex },
      })}
    onAcknowledgeSessionsChange={(checked) => (acknowledgeSessions = checked)}
    onConfirm={() =>
      (view = {
        ...view,
        stashApplyPreview: null,
        statusBanner: m.s7_shell_stash_apply_confirmed(),
      })}
    onClose={() => (view = { ...view, stashApplyPreview: null })}
  />
{/if}

{#if view.stashDropPreview}
  <GitStashDropDialog
    target={view.stashDropPreview.entry}
    preview={view.stashDropPreview}
    onConfirm={() =>
      (view = {
        ...view,
        stashDropPreview: null,
        statusBanner: m.s7_shell_stash_drop_confirmed(),
      })}
    onClose={() => (view = { ...view, stashDropPreview: null })}
  />
{/if}

{#if view.fileInspectorOpen}
  <GitFileInspectorDialog
    hasWorktree={Boolean(view.context.worktreePath)}
    fixtureHistoryRows={view.fixtureHistoryRows}
    fixtureBlame={view.fixtureBlame}
    onJumpToCommit={(oid) =>
      (view = {
        ...view,
        fileInspectorOpen: false,
        mode: "log",
        selectedCommitOid: oid,
      })}
    onClose={() => (view = { ...view, fileInspectorOpen: false })}
  />
{/if}

{#if view.discardPreview}
  <GitDiscardPreviewDialog
    preview={view.discardPreview}
    {acknowledgeSessions}
    onAcknowledgeSessionsChange={(checked) => (acknowledgeSessions = checked)}
    onConfirm={() => {
      view = applyShellDiscardConfirm(view, pendingDiscard);
      pendingDiscard = null;
    }}
    onClose={() => {
      pendingDiscard = null;
      view = { ...view, discardPreview: null };
    }}
  />
{/if}
