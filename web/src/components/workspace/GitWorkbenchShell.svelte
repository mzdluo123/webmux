<script lang="ts">
  import * as Tabs from "$lib/components/ui/tabs";
  import type {
    GitBranchItem,
    GitCommitOutcome,
    GitPrecedingCommit,
    GitPushPreviewModel,
    GitWorkbenchMode,
    GitWorkbenchViewState,
  } from "../../contracts/git-workbench-schemas";
  import type { SessionInfo } from "../../contracts/schemas";
  import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
  import type { GitCommitDraftConflictState } from "../../domains/workspace/git-commit-draft-model";
  import {
    applyShellAbortPreviewBanner,
    applyShellCheckoutBanner,
    applyShellContinueInProgress,
    applyShellOccupiedWorktreeBanner,
    applyShellReloadVerify,
    applyShellStageLines,
    applyShellStashBanner,
    buildShellDiscardPreview,
    type PendingShellDiscard,
  } from "../../domains/workspace/git-shell-v3-actions";
  import {
    applyShellBranchAction,
    applyShellClearTaskBaseline,
    applyShellDraftResolution,
    applyShellResolveAmendDraft,
    applyShellSetTaskBaseline,
    applyShellStageGroup,
    applyShellToggleAmend,
    buildShellPushPreview,
  } from "../../domains/workspace/git-shell-actions";
  import {
    applyStageHunk,
    applyUnstageHunk,
    buildCommitPreviewModel,
    selectLogBranchFilter,
    toggleReviewedChange,
  } from "../../domains/workspace/git-workbench-state";
  import { stashFileKey } from "../../hooks/workspace/git-stash-controller.svelte";
  import GitBranchesPane from "./GitBranchesPane.svelte";
  import GitCommitOutcomeBanner from "./GitCommitOutcomeBanner.svelte";
  import GitCommitPane from "./GitCommitPane.svelte";
  import GitConflictView from "./GitConflictView.svelte";
  import GitInProgressBanner from "./GitInProgressBanner.svelte";
  import GitStashPane from "./GitStashPane.svelte";
  import GitWorkbenchModeTabs from "./GitWorkbenchModeTabs.svelte";
  import GitWorkbenchShellDialogs from "./GitWorkbenchShellDialogs.svelte";
  import GitWorkbenchShellHeader from "./GitWorkbenchShellHeader.svelte";
  import GitWorkbenchShellLogTab from "./GitWorkbenchShellLogTab.svelte";
  import GitOperationsPopover from "./GitOperationsPopover.svelte";

  let {
    view = $bindable(),
    simulatedCommitOutcome = null,
    defaultPushPreview = null,
    initialDraftConflict = null,
    simulatedActiveSessions = [],
    onOpenWorktree,
  }: {
    view: GitWorkbenchViewState;
    simulatedCommitOutcome?: GitCommitOutcome | null;
    defaultPushPreview?: GitPushPreviewModel | null;
    initialDraftConflict?: GitCommitDraftConflictState | null;
    simulatedActiveSessions?: readonly SessionInfo[];
    onOpenWorktree?: (worktreeId: string, path: string) => void;
  } = $props();

  let draftConflict = $derived(initialDraftConflict);
  let draftRevision = $derived(initialDraftConflict?.remoteDraft.revision ?? 0);
  let acknowledgeSessions = $state(false);
  let selectedStashFileKey = $state<string | null>(null);
  let pendingDiscard = $state<PendingShellDiscard | null>(null);
  const readonlyMode = $derived(
    view.context.headState === "readonly" || !view.context.worktreePath,
  );
  const stageLinesInShell = (
    id: string,
    sel: Parameters<typeof applyShellStageLines>[2],
    op: "stage" | "unstage",
  ) => (view = applyShellStageLines(view, id, sel, op));

  function setMode(next: string) {
    if (["log", "commit", "branches", "stash"].includes(next)) {
      view = { ...view, mode: next as GitWorkbenchMode };
    }
  }
  const handleBranchFilter = (branch: string | null) =>
    (view = selectLogBranchFilter(view, branch));
  const handleRequestCheckout = (branch: string) =>
    (view = applyShellCheckoutBanner(view, branch));
  const handleOpenOccupiedWorktree = (worktreeId: string, path: string) =>
    (view = applyShellOccupiedWorktreeBanner(
      view,
      worktreeId,
      path,
      onOpenWorktree,
    ));
  const openCommitPreview = (followUpPush: boolean) =>
    (view = {
      ...view,
      commitPreview: buildCommitPreviewModel(view, followUpPush),
    });
  const openPushDialog = (branchOverride?: string, pre?: GitPrecedingCommit) =>
    (view = {
      ...view,
      pushPreview: buildShellPushPreview(
        view,
        readonlyMode,
        defaultPushPreview,
        branchOverride,
        pre,
      ),
    });
  function handleBranchAction(
    kind: GitSharedBranchActionKind,
    branch: GitBranchItem,
  ) {
    const next = applyShellBranchAction({
      view,
      kind,
      branch,
      onOpenWorktree,
      onOpenPush: (name) => openPushDialog(name),
    });
    if (next) view = next;
  }
  const inspectCommitInLog = (oid: string) =>
    (view = {
      ...view,
      mode: "log",
      selectedCommitOid: oid,
      selectedCommitFilePath: null,
    });
  const reloadVerifyOutcome = () => (view = applyShellReloadVerify(view));
  function resolveDraft(
    strategy: "keep-local" | "adopt-remote" | "merge-both",
  ) {
    const res = applyShellDraftResolution(view, draftConflict, strategy);
    if (!res) return;
    draftRevision = res.nextRevision;
    draftConflict = null;
    view = res.nextView;
  }
  function openShellDiscard(p: PendingShellDiscard) {
    pendingDiscard = p;
    acknowledgeSessions = false;
    view = {
      ...view,
      discardPreview: buildShellDiscardPreview(
        view,
        p,
        simulatedActiveSessions,
      ),
    };
  }
</script>

<div
  class="git-workbench flex h-full min-h-0 min-w-0 flex-col border bg-background text-foreground"
  data-testid="git-workbench"
>
  {#snippet operationLedger()}
    <GitOperationsPopover
      worktreePath={view.context.worktreePath}
      statusBanner={view.statusBanner}
      lastFetchedAt={view.lastFetchedAt}
      activeOperation={view.activeOperation ?? null}
      operations={view.operations ?? []}
      onInspectCommit={inspectCommitInLog}
      onReloadVerify={reloadVerifyOutcome}
    />
  {/snippet}

  <GitWorkbenchShellHeader
    operations={operationLedger}
    context={view.context}
    branches={view.branches}
    branchFilter={view.branchFilter}
    compareBaseOid={view.context.headOid ?? view.commits[0]?.oid ?? null}
    {readonlyMode}
    onSelectAllBranches={() => handleBranchFilter(null)}
    onBranchAction={handleBranchAction}
    onOpenPush={() => openPushDialog()}
  />

  {#if view.lastCommitOutcome && view.lastCommitOutcome.status !== "matched"}
    <GitCommitOutcomeBanner
      outcome={view.lastCommitOutcome}
      onInspectCommit={inspectCommitInLog}
      onReloadVerify={reloadVerifyOutcome}
      onDismiss={() => (view = { ...view, lastCommitOutcome: null })}
    />
  {/if}

  {#if view.inProgress?.active}
    <GitInProgressBanner
      operation={view.inProgress}
      conflictResolverOpen={Boolean(view.conflictOpen)}
      hasActiveWorktreeSession={simulatedActiveSessions.length > 0}
      onOpenConflictResolver={() => (view = { ...view, conflictOpen: true })}
      onContinue={() => (view = applyShellContinueInProgress(view))}
      onOpenAbortPreview={() => (view = applyShellAbortPreviewBanner(view))}
    />
  {/if}

  {#if view.conflictOpen}
    <GitConflictView
      inProgress={view.inProgress ?? null}
      fixtureDetail={view.conflictDetail ?? null}
      onContinueOperation={() => (view = applyShellContinueInProgress(view))}
      onClose={() => (view = { ...view, conflictOpen: false })}
    />
  {:else}
    <Tabs.Root
      value={view.mode}
      onValueChange={setMode}
      orientation="horizontal"
      class="min-h-0 min-w-0 flex-1"
    >
      <GitWorkbenchModeTabs stashCount={view.stashes?.length ?? 0} />

      <Tabs.Content
        value="log"
        class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
      >
        <GitWorkbenchShellLogTab
          bind:view
          {readonlyMode}
          onBranchFilter={handleBranchFilter}
          onBranchAction={handleBranchAction}
          onSetTaskBaseline={(oid) =>
            (view = applyShellSetTaskBaseline(view, oid))}
          onRequestCheckout={handleRequestCheckout}
          onOpenOccupiedWorktree={handleOpenOccupiedWorktree}
        />
      </Tabs.Content>

      <Tabs.Content
        value="commit"
        class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
      >
        <GitCommitPane
          context={view.context}
          changes={view.workingChanges}
          activeChangeId={view.activeChangeId}
          reviewedPaths={view.reviewedPaths}
          commitMessage={view.commitMessage}
          reviewScope={view.reviewScope}
          taskReview={view.taskReview}
          uncertainBlocked={view.lastCommitOutcome?.status === "uncertain"}
          {draftRevision}
          {draftConflict}
          commitAmend={Boolean(view.commitAmend)}
          commitAmendPushed={Boolean(view.commitAmendPushed)}
          commitAuthorOverride={view.commitAuthorOverride ?? ""}
          commitSignoff={Boolean(view.commitSignoff)}
          commitSignoffTrailer={view.commitSignoffTrailer ??
            (view.context.committerName && view.context.committerEmail
              ? `Signed-off-by: ${view.context.committerName} <${view.context.committerEmail}>`
              : "")}
          amendDraftPrompt={view.amendDraftConflict ?? null}
          hasActiveWorktreeSession={simulatedActiveSessions.length > 0}
          defaultLinePickerOpen={Boolean(view.defaultLinePickerOpen)}
          activeSessionCount={simulatedActiveSessions.length}
          onReviewScopeChange={(reviewScope) =>
            (view = { ...view, reviewScope })}
          onSelectChange={(activeChangeId) =>
            (view = { ...view, activeChangeId })}
          onToggleReviewed={(c) =>
            (view = {
              ...view,
              reviewedPaths: toggleReviewedChange(view.reviewedPaths, c),
            })}
          onStageChange={(id) =>
            (view = {
              ...view,
              workingChanges: applyStageHunk(view.workingChanges, id),
            })}
          onUnstageChange={(id) =>
            (view = {
              ...view,
              workingChanges: applyUnstageHunk(view.workingChanges, id),
            })}
          onStageGroup={(op, group) =>
            (view = {
              ...view,
              workingChanges: applyShellStageGroup(
                view.workingChanges,
                op,
                group,
              ),
            })}
          onStageHunk={(id, hId) =>
            (view = {
              ...view,
              workingChanges: applyStageHunk(view.workingChanges, id, hId),
            })}
          onUnstageHunk={(id, hId) =>
            (view = {
              ...view,
              workingChanges: applyUnstageHunk(view.workingChanges, id, hId),
            })}
          onStageLines={(id, sel) => stageLinesInShell(id, sel, "stage")}
          onUnstageLines={(id, sel) => stageLinesInShell(id, sel, "unstage")}
          onDiscardChanges={!readonlyMode
            ? (_ids, paths) =>
                openShellDiscard({ scope: "files", paths: [...paths] })
            : undefined}
          onDiscardHunk={!readonlyMode
            ? (changeId, hunkId) =>
                openShellDiscard({ scope: "hunks", changeId, hunkId })
            : undefined}
          onDiscardLines={!readonlyMode
            ? (changeId, lineSelections) =>
                openShellDiscard({
                  scope: "lines",
                  changeId,
                  lineSelections: [...lineSelections],
                })
            : undefined}
          onCommitMessageChange={(commitMessage) =>
            (view = { ...view, commitMessage })}
          onResolveDraftKeepLocal={() => resolveDraft("keep-local")}
          onResolveDraftAdoptRemote={() => resolveDraft("adopt-remote")}
          onResolveDraftMergeBoth={() => resolveDraft("merge-both")}
          onOpenCommitPreview={openCommitPreview}
          onOpenPush={() => openPushDialog()}
          onToggleCommitAmend={(checked) =>
            (view = applyShellToggleAmend(view, checked))}
          onResolveAmendDraft={(choice) =>
            (view = applyShellResolveAmendDraft(view, choice))}
          onCommitAuthorOverrideChange={(commitAuthorOverride) =>
            (view = { ...view, commitAuthorOverride })}
          onToggleCommitSignoff={(commitSignoff) =>
            (view = { ...view, commitSignoff })}
          onOpenStashCreate={() =>
            (view = applyShellStashBanner(view, "create"))}
          onInspectFile={() => (view = { ...view, fileInspectorOpen: true })}
          onSelectTaskFile={(selectedPath) =>
            view.taskReview &&
            (view = {
              ...view,
              taskReview: { ...view.taskReview, selectedPath },
            })}
          onSetTaskBaseline={!readonlyMode
            ? (oid) => (view = applyShellSetTaskBaseline(view, oid))
            : undefined}
          onClearTaskBaseline={!readonlyMode
            ? () => (view = applyShellClearTaskBaseline(view))
            : undefined}
          onOpenLogToPickBaseline={() =>
            (view = { ...view, mode: "log", scope: "all" })}
          onInspectCommit={inspectCommitInLog}
          onOpenConflictResolver={() =>
            (view = { ...view, conflictOpen: true })}
        />
      </Tabs.Content>

      <Tabs.Content
        value="branches"
        class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
      >
        <GitBranchesPane
          context={view.context}
          branches={view.branches}
          remotes={view.remotes}
          branchFilter={view.branchFilter}
          onFilterInLog={(branch) => {
            view = {
              ...selectLogBranchFilter(view, branch),
              mode: "log",
            };
          }}
          onRequestCheckout={handleRequestCheckout}
          onOpenOccupiedWorktree={handleOpenOccupiedWorktree}
          onOpenPush={(branch) => openPushDialog(branch)}
          onBranchAction={handleBranchAction}
        />
      </Tabs.Content>

      <Tabs.Content
        value="stash"
        class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
      >
        <GitStashPane
          context={view.context}
          entries={view.stashes ?? []}
          selectedStashOid={view.selectedStashOid ?? null}
          detail={view.selectedStashDetail ?? null}
          selectedFileKey={selectedStashFileKey}
          onSelectStash={(selectedStashOid) =>
            (view = { ...view, selectedStashOid })}
          onSelectFile={(f) => (selectedStashFileKey = stashFileKey(f))}
          onOpenCreate={!readonlyMode
            ? () => (view = applyShellStashBanner(view, "create"))
            : undefined}
          onOpenApply={!readonlyMode
            ? (entry, mode) =>
                (view = applyShellStashBanner(view, mode, entry.stashRef))
            : undefined}
          onOpenDrop={(entry) =>
            (view = applyShellStashBanner(view, "drop", entry.stashRef))}
        />
      </Tabs.Content>
    </Tabs.Root>
  {/if}

  <GitWorkbenchShellDialogs
    bind:view
    bind:pendingDiscard
    {simulatedCommitOutcome}
    bind:acknowledgeSessions
    onOpenFollowUpPush={(branch, preceding) =>
      openPushDialog(branch, preceding)}
  />
</div>
