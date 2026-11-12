<script lang="ts">
  import type { GitPrecedingCommit } from "../../contracts/git-workbench-schemas";
  import type { SessionInfo } from "../../contracts/schemas";
  import {
    deliverTextToWorktreeSession,
    startNewWorktreeSessionWithText,
  } from "../../hooks/workspace/git-session-handoff";
  import type { createGitWorkbenchController } from "../../hooks/workspace/git-workbench-controller.svelte";
  import type { Shell } from "../../hooks/shared/shell.svelte";
  import GitAbortOperationDialog from "./GitAbortOperationDialog.svelte";
  import GitCommitPreviewDialog from "./GitCommitPreviewDialog.svelte";
  import GitDivergencePreviewDialog from "./GitDivergencePreviewDialog.svelte";
  import GitPushDialog from "./GitPushDialog.svelte";
  import GitReworkDialog from "./GitReworkDialog.svelte";

  let {
    controller,
    shell,
    repositoryId,
    worktreeId,
    activeWorktreeSessions,
    onCreateBranchFromHead,
    onPush,
    onFetchRemote,
    onFastForwardBranch,
  }: {
    controller: ReturnType<typeof createGitWorkbenchController>;
    shell: Shell;
    repositoryId: string;
    worktreeId?: string;
    activeWorktreeSessions: readonly SessionInfo[];
    onCreateBranchFromHead: () => void;
    onPush: (branch?: string, preceding?: GitPrecedingCommit | null) => void;
    onFetchRemote: () => void;
    onFastForwardBranch: (branch: string) => void;
  } = $props();

  const stagedChanges = $derived(
    controller.view.workingChanges.filter((c) => c.group === "staged"),
  );

  async function handleDeliverToSession(targetSession: SessionInfo) {
    const fresh = await controller.rework.verifyWorktreeFreshness();
    if (!fresh) return;
    const text = controller.rework.buildPayloadText(stagedChanges);
    if (!text.trim()) return;
    deliverTextToWorktreeSession({ shell, session: targetSession, text });
    if (controller.rework.mode === "rework") {
      controller.rework.clearAll();
    }
    controller.rework.closeSheet();
  }

  function handleStartNewSessionWithDraft() {
    const text = controller.rework.buildPayloadText(stagedChanges);
    if (!text.trim()) return;
    startNewWorktreeSessionWithText({
      shell,
      repositoryId,
      worktreeId,
      text,
    });
    if (controller.rework.mode === "rework") {
      controller.rework.clearAll();
    }
    controller.rework.closeSheet();
  }
</script>

{#if controller.view.commitPreview}
  <GitCommitPreviewDialog
    model={controller.view.commitPreview}
    commitMessage={controller.view.commitMessage}
    {repositoryId}
    {worktreeId}
    submitting={controller.staging.commitSubmitting}
    uncertainBlocked={controller.operations.hasUncertainUnresolved ||
      controller.view.lastCommitOutcome?.status === "uncertain"}
    onConfirm={() =>
      void controller.confirmCommit((branch, preceding) =>
        onPush(branch, preceding),
      )}
    onRefreshPreview={() => void controller.refreshCommitPreview()}
    {onCreateBranchFromHead}
    onClose={() => controller.closeCommitPreview()}
  />
{/if}

{#if controller.push.open}
  <GitPushDialog
    push={controller.push}
    {onFetchRemote}
    {onFastForwardBranch}
    onMergeRemoteBranch={(remoteRef) =>
      void controller.divergence.openMergePreview("remote", remoteRef)}
    onRebaseOntoRemoteBranch={(remoteRef) =>
      void controller.divergence.openRebasePreview("remote", remoteRef)}
  />
{/if}

{#if controller.divergence.previewOpen}
  <GitDivergencePreviewDialog
    kind={controller.divergence.previewKind}
    preview={controller.divergence.preview}
    loading={controller.divergence.loadingPreview}
    submitting={controller.divergence.submitting}
    dialogError={controller.divergence.dialogError}
    acknowledgeSessions={controller.divergence.acknowledgeSessions}
    onAcknowledgeSessionsChange={(checked) =>
      controller.divergence.setAcknowledgeSessions(checked)}
    onConfirm={() => void controller.divergence.confirmDivergence()}
    onClose={() => controller.divergence.closePreview()}
  />
{/if}

{#if controller.divergence.abortOpen}
  <GitAbortOperationDialog
    abortPreview={controller.divergence.abortPreview}
    fetching={controller.divergence.loadingPreview}
    busy={controller.divergence.submitting}
    errorMessage={controller.divergence.dialogError}
    acknowledged={controller.divergence.acknowledgeSessions}
    onAcknowledgeChange={(checked) =>
      controller.divergence.setAcknowledgeSessions(checked)}
    onConfirmAbort={() => void controller.divergence.confirmAbort()}
    onDismiss={() => controller.divergence.closeAbortPreview()}
  />
{/if}

{#if controller.rework.sheetOpen}
  <GitReworkDialog
    mode={controller.rework.mode}
    items={controller.rework.items}
    {stagedChanges}
    activeSessions={activeWorktreeSessions}
    bind:selectedSessionId={controller.rework.selectedSessionId}
    checkingFreshness={controller.rework.checkingFreshness}
    stalePaths={controller.rework.stalePaths}
    staleConfirmed={controller.rework.staleConfirmed}
    copiedFeedback={controller.rework.copiedFeedback}
    hasCommitMessageDraft={Boolean(controller.view.commitMessage.trim())}
    onUpdateNote={(id, note) => controller.rework.updateItemNote(id, note)}
    onRemoveItem={(id) => controller.rework.removeItem(id)}
    onClearAll={() => controller.rework.clearAll()}
    onCopyMarkdown={() => void controller.rework.copyMarkdown(stagedChanges)}
    onConfirmStale={() => controller.rework.confirmSendDespiteStale()}
    onDeliverToSession={(s) => void handleDeliverToSession(s)}
    onStartNewSessionWithDraft={handleStartNewSessionWithDraft}
    onClose={() => controller.rework.closeSheet()}
  />
{/if}
