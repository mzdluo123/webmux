<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import * as Tabs from "$lib/components/ui/tabs";
  import type {
    GitBranchItem,
    GitCommitOutcome,
    GitPrecedingCommit,
    GitPushPreviewModel,
    GitWorkbenchMode,
    GitWorkbenchViewState,
  } from "../../contracts/git-workbench-schemas";
  import type { GitReworkItem, SessionInfo } from "../../contracts/schemas";
  import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
  import type { GitCommitDraftConflictState } from "../../domains/workspace/git-commit-draft-model";
  import { createReworkDraftItem } from "../../domains/workspace/git-rework-model";
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
  import type { GitSessionHandoffMode } from "../../hooks/workspace/git-rework-controller.svelte";
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
  import GitWorkbenchStatusBar from "./GitWorkbenchStatusBar.svelte";

  let {
    view = $bindable(),
    simulatedCommitOutcome = null,
    defaultPushPreview = null,
    initialDraftConflict = null,
    initialReworkItems = [],
    simulatedActiveSessions = [],
    initialReworkOpen = false,
    onOpenWorktree,
  }: {
    view: GitWorkbenchViewState;
    simulatedCommitOutcome?: GitCommitOutcome | null;
    defaultPushPreview?: GitPushPreviewModel | null;
    initialDraftConflict?: GitCommitDraftConflictState | null;
    initialReworkItems?: readonly GitReworkItem[];
    simulatedActiveSessions?: readonly SessionInfo[];
    initialReworkOpen?: boolean;
    onOpenWorktree?: (worktreeId: string, path: string) => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  let draftConflict = $derived(initialDraftConflict);
  let draftRevision = $derived(initialDraftConflict?.remoteDraft.revision ?? 0);
  let reworkItems = $derived([...initialReworkItems]);
  let reworkOpen = $derived(initialReworkOpen);
  let handoffMode = $state<GitSessionHandoffMode>("rework");
  let acknowledgeSessions = $state(false);
  let selectedStashFileKey = $state<string | null>(null);
  let selectedSessionId = $derived(
    simulatedActiveSessions.length === 1
      ? (simulatedActiveSessions[0]?.id ?? "")
      : "",
  );
  const readonlyMode = $derived(
    view.context.headState === "readonly" || !view.context.worktreePath,
  );

  function setMode(next: string) {
    if (
      next === "log" ||
      next === "commit" ||
      next === "branches" ||
      next === "stash"
    ) {
      view = { ...view, mode: next as GitWorkbenchMode };
    }
  }
  const handleBranchFilter = (branch: string | null) =>
    (view = selectLogBranchFilter(view, branch));
  const handleRequestCheckout = (branch: string) =>
    (view = {
      ...view,
      statusBanner: `已打开分支检出预览：${branch}（不会静默覆盖未提交改动）`,
    });
  function handleOpenOccupiedWorktree(worktreeId: string, path: string) {
    onOpenWorktree?.(worktreeId, path);
    view = {
      ...view,
      statusBanner: `已定位到占用该分支的工作树：${path} (${worktreeId})`,
    };
  }
  const openCommitPreview = (followUpPush: boolean) =>
    (view = {
      ...view,
      commitPreview: buildCommitPreviewModel(view, followUpPush),
    });
  const openPushDialog = (
    branchOverride?: string,
    precedingCommit?: GitPrecedingCommit,
  ) =>
    (view = {
      ...view,
      pushPreview: buildShellPushPreview(
        view,
        readonlyMode,
        defaultPushPreview,
        branchOverride,
        precedingCommit,
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
  const reloadVerifyOutcome = () =>
    (view = {
      ...view,
      lastCommitOutcome: null,
      statusBanner: "已刷新历史与暂存区核实状态",
    });
  function resolveDraft(
    strategy: "keep-local" | "adopt-remote" | "merge-both",
  ) {
    const res = applyShellDraftResolution(view, draftConflict, strategy);
    if (!res) return;
    draftRevision = res.nextRevision;
    draftConflict = null;
    view = res.nextView;
  }

  function handleAddReworkItem(
    input: Parameters<typeof createReworkDraftItem>[0],
    options?: { openSheet?: boolean },
  ) {
    reworkItems = [...reworkItems, createReworkDraftItem(input)];
    if (options?.openSheet ?? true) {
      handoffMode = "rework";
      reworkOpen = true;
    }
  }
</script>

<div
  class="git-workbench flex h-full min-h-0 min-w-0 flex-col border border-git-border bg-git-surface text-foreground"
  data-testid="git-workbench"
>
  <GitWorkbenchShellHeader
    context={view.context}
    branches={view.branches}
    branchFilter={view.branchFilter}
    compareBaseOid={view.context.headOid ?? view.commits[0]?.oid ?? null}
    {readonlyMode}
    reworkCount={reworkItems.length}
    onSelectAllBranches={() => handleBranchFilter(null)}
    onBranchAction={handleBranchAction}
    onOpenRework={() => {
      handoffMode = "rework";
      reworkOpen = true;
    }}
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
      onContinue={() =>
        (view = {
          ...view,
          inProgress: null,
          conflictOpen: false,
          statusBanner: "已完成冲突解决并继续完成操作",
        })}
      onOpenAbortPreview={() =>
        (view = { ...view, statusBanner: "已打开中止回退确认预览" })}
    />
  {/if}

  {#if view.conflictOpen}
    <GitConflictView
      inProgress={view.inProgress ?? null}
      fixtureDetail={view.conflictDetail ?? null}
      onContinueOperation={() =>
        (view = {
          ...view,
          inProgress: null,
          conflictOpen: false,
          statusBanner: "已完成冲突解决并继续完成操作",
        })}
      onClose={() => (view = { ...view, conflictOpen: false })}
    />
  {:else}
    <Tabs.Root
      value={view.mode}
      onValueChange={setMode}
      orientation={desktop.current ? "vertical" : "horizontal"}
      class="min-h-0 min-w-0 flex-1"
    >
      <GitWorkbenchModeTabs
        desktop={desktop.current}
        stashCount={view.stashes?.length ?? 0}
      />

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
          onAddReworkItem={handleAddReworkItem}
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
          onAddReworkItem={!readonlyMode ? handleAddReworkItem : undefined}
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
          onCommitMessageChange={(commitMessage) =>
            (view = { ...view, commitMessage })}
          onResolveDraftKeepLocal={() => resolveDraft("keep-local")}
          onResolveDraftAdoptRemote={() => resolveDraft("adopt-remote")}
          onResolveDraftMergeBoth={() => resolveDraft("merge-both")}
          onRequestCommitAssist={simulatedActiveSessions.length > 0
            ? () => {
                handoffMode = "commit-assist";
                reworkOpen = true;
              }
            : undefined}
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
            (view = { ...view, statusBanner: "已打开新建贮藏对话框" })}
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
            ? () => (view = { ...view, statusBanner: "已打开新建贮藏对话框" })
            : undefined}
          onOpenApply={!readonlyMode
            ? (entry, mode) =>
                (view = {
                  ...view,
                  statusBanner: `已打开贮藏${mode === "pop" ? "安全弹出" : "应用"}确认：${entry.stashRef}`,
                })
            : undefined}
          onOpenDrop={(entry) =>
            (view = {
              ...view,
              statusBanner: `已打开丢弃贮藏确认：${entry.stashRef}`,
            })}
        />
      </Tabs.Content>
    </Tabs.Root>
  {/if}

  <GitWorkbenchStatusBar
    worktreePath={view.context.worktreePath}
    statusBanner={view.statusBanner}
    lastFetchedAt={view.lastFetchedAt}
    activeOperation={view.activeOperation ?? null}
    operations={view.operations ?? []}
    onInspectCommit={inspectCommitInLog}
    onReloadVerify={reloadVerifyOutcome}
  />

  <GitWorkbenchShellDialogs
    bind:view
    bind:reworkOpen
    bind:reworkItems
    {handoffMode}
    activeSessions={simulatedActiveSessions}
    {simulatedCommitOutcome}
    bind:selectedSessionId
    bind:acknowledgeSessions
    onOpenFollowUpPush={(branch, preceding) =>
      openPushDialog(branch, preceding)}
  />
</div>
