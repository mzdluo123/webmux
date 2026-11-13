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
  import GitBranchRenameDialog from "./GitBranchRenameDialog.svelte";
  import GitBranchUpstreamDialog from "./GitBranchUpstreamDialog.svelte";
  import GitCommitPreviewDialog from "./GitCommitPreviewDialog.svelte";
  import GitDivergencePreviewDialog from "./GitDivergencePreviewDialog.svelte";
  import GitFileInspectorDialog from "./GitFileInspectorDialog.svelte";
  import GitPushDialog from "./GitPushDialog.svelte";
  import GitRemotesDialogs from "./GitRemotesDialogs.svelte";
  import GitReworkDialog from "./GitReworkDialog.svelte";
  import GitStashApplyDialog from "./GitStashApplyDialog.svelte";
  import GitStashCreateDialog from "./GitStashCreateDialog.svelte";
  import GitStashDropDialog from "./GitStashDropDialog.svelte";
  import GitTagCreateDialog from "./GitTagCreateDialog.svelte";
  import GitTagDeleteDialog from "./GitTagDeleteDialog.svelte";

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
  const remoteBranches = $derived(
    controller.view.branches.filter((b) => b.kind === "remote"),
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
    acknowledgePushedAmend={controller.commitOptions.acknowledgePushedAmend}
    onAcknowledgePushedAmendChange={(checked) =>
      controller.commitOptions.setAcknowledgePushedAmend(checked)}
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

{#if controller.stash.dialogs.createOpen}
  <GitStashCreateDialog
    message={controller.stash.dialogs.createMessage}
    includeUntracked={controller.stash.dialogs.createIncludeUntracked}
    keepIndex={controller.stash.dialogs.createKeepIndex}
    preview={controller.stash.dialogs.createPreview}
    loading={controller.stash.dialogs.loadingPreview}
    submitting={controller.stash.dialogs.submitting}
    acknowledgeSessions={controller.stash.dialogs.acknowledgeSessions}
    dialogError={controller.stash.dialogs.dialogError}
    onMessageChange={(val) => controller.stash.dialogs.setCreateMessage(val)}
    onIncludeUntrackedChange={(checked) =>
      controller.stash.dialogs.setCreateIncludeUntracked(checked)}
    onKeepIndexChange={(checked) =>
      controller.stash.dialogs.setCreateKeepIndex(checked)}
    onAcknowledgeSessionsChange={(checked) =>
      controller.stash.dialogs.setAcknowledgeSessions(checked)}
    onConfirm={() => void controller.stash.dialogs.confirmCreate()}
    onClose={() => controller.stash.dialogs.closeDialogs()}
  />
{/if}

{#if controller.stash.dialogs.applyOpen}
  <GitStashApplyDialog
    mode={controller.stash.dialogs.applyMode}
    reinstateIndex={controller.stash.dialogs.applyReinstateIndex}
    target={controller.stash.dialogs.applyTarget}
    preview={controller.stash.dialogs.applyPreview}
    loading={controller.stash.dialogs.loadingPreview}
    submitting={controller.stash.dialogs.submitting}
    acknowledgeSessions={controller.stash.dialogs.acknowledgeSessions}
    dialogError={controller.stash.dialogs.dialogError}
    onModeChange={(mode) => controller.stash.dialogs.setApplyMode(mode)}
    onReinstateIndexChange={(checked) =>
      controller.stash.dialogs.setApplyReinstateIndex(checked)}
    onAcknowledgeSessionsChange={(checked) =>
      controller.stash.dialogs.setAcknowledgeSessions(checked)}
    onConfirm={() => void controller.stash.dialogs.confirmApply()}
    onClose={() => controller.stash.dialogs.closeDialogs()}
  />
{/if}

{#if controller.stash.dialogs.dropOpen}
  <GitStashDropDialog
    target={controller.stash.dialogs.dropTarget}
    preview={controller.stash.dialogs.dropPreview}
    loading={controller.stash.dialogs.loadingPreview}
    submitting={controller.stash.dialogs.submitting}
    dialogError={controller.stash.dialogs.dialogError}
    onConfirm={() => void controller.stash.dialogs.confirmDrop()}
    onClose={() => controller.stash.dialogs.closeDialogs()}
  />
{/if}

{#if controller.refsManage.renameOpen}
  <GitBranchRenameDialog
    oldName={controller.refsManage.renameOldName}
    newName={controller.refsManage.renameNewName}
    preview={controller.refsManage.renamePreview}
    loading={controller.refsManage.loadingPreview}
    submitting={controller.refsManage.submitting}
    acknowledgeSessions={controller.refsManage.renameAckSessions}
    dialogError={controller.refsManage.dialogError}
    onNewNameChange={(val) => controller.refsManage.setRenameNewName(val)}
    onAcknowledgeSessionsChange={(checked) =>
      controller.refsManage.setRenameAckSessions(checked)}
    onConfirm={() => void controller.refsManage.confirmRename()}
    onClose={() => controller.refsManage.closeDialogs()}
  />
{/if}

{#if controller.refsManage.upstreamOpen}
  <GitBranchUpstreamDialog
    branchName={controller.refsManage.upstreamBranch}
    currentUpstream={controller.refsManage.upstreamCurrent}
    selection={controller.refsManage.upstreamSelection}
    {remoteBranches}
    submitting={controller.refsManage.submitting}
    dialogError={controller.refsManage.dialogError}
    onSelectionChange={(val) => controller.refsManage.setUpstreamSelection(val)}
    onConfirmSet={() => void controller.refsManage.confirmSetUpstream(false)}
    onConfirmUnset={() => void controller.refsManage.confirmSetUpstream(true)}
    onClose={() => controller.refsManage.closeDialogs()}
  />
{/if}

{#if controller.refsManage.tagCreateOpen}
  <GitTagCreateDialog
    targetOid={controller.refsManage.tagTargetOid}
    targetLabel={controller.refsManage.tagTargetLabel}
    tagName={controller.refsManage.tagName}
    tagKind={controller.refsManage.tagKind}
    tagMessage={controller.refsManage.tagMessage}
    submitting={controller.refsManage.submitting}
    dialogError={controller.refsManage.dialogError}
    onTagNameChange={(val) => controller.refsManage.setTagName(val)}
    onTagKindChange={(val) => controller.refsManage.setTagKind(val)}
    onTagMessageChange={(val) => controller.refsManage.setTagMessage(val)}
    onConfirm={() => void controller.refsManage.confirmCreateTag()}
    onClose={() => controller.refsManage.closeDialogs()}
  />
{/if}

{#if controller.refsManage.tagDeleteOpen && controller.refsManage.tagDeleteTarget}
  <GitTagDeleteDialog
    target={controller.refsManage.tagDeleteTarget}
    submitting={controller.refsManage.submitting}
    dialogError={controller.refsManage.dialogError}
    onConfirm={() => void controller.refsManage.confirmDeleteTag()}
    onClose={() => controller.refsManage.closeDialogs()}
  />
{/if}

<GitRemotesDialogs remotes={controller.refsManage.remotes} />

{#if controller.fileInspector.open}
  <GitFileInspectorDialog
    inspector={controller.fileInspector}
    hasWorktree={Boolean(worktreeId)}
    onJumpToCommit={(oid) => controller.inspectCommitInLog(oid)}
  />
{/if}
