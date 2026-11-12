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
    applyShellClearFilters,
    applyShellClearTaskBaseline,
    applyShellCommitConfirm,
    applyShellDraftResolution,
    applyShellSetTaskBaseline,
    applyShellStageGroup,
    applyShellStartCompare,
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
  import GitBranchesPane from "./GitBranchesPane.svelte";
  import GitCommitOutcomeBanner from "./GitCommitOutcomeBanner.svelte";
  import GitCommitPane from "./GitCommitPane.svelte";
  import GitLogPane from "./GitLogPane.svelte";
  import GitWorkbenchModeTabs from "./GitWorkbenchModeTabs.svelte";
  import GitWorkbenchShellDialogs from "./GitWorkbenchShellDialogs.svelte";
  import GitWorkbenchShellHeader from "./GitWorkbenchShellHeader.svelte";
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
  let selectedSessionId = $derived(
    simulatedActiveSessions.length === 1
      ? (simulatedActiveSessions[0]?.id ?? "")
      : "",
  );

  const readonlyMode = $derived(
    view.context.headState === "readonly" || !view.context.worktreePath,
  );

  function setMode(next: string) {
    if (next === "log" || next === "commit" || next === "branches") {
      view = { ...view, mode: next as GitWorkbenchMode };
    }
  }

  function handleBranchFilter(branch: string | null) {
    view = selectLogBranchFilter(view, branch);
  }

  function handleRequestCheckout(branch: string) {
    view = {
      ...view,
      statusBanner: `已打开分支检出预览：${branch}（不会静默覆盖未提交改动）`,
    };
  }

  function handleOpenOccupiedWorktree(worktreeId: string, path: string) {
    onOpenWorktree?.(worktreeId, path);
    view = {
      ...view,
      statusBanner: `已定位到占用该分支的工作树：${path} (${worktreeId})`,
    };
  }

  function openCommitPreview(followUpPush: boolean) {
    view = {
      ...view,
      commitPreview: buildCommitPreviewModel(view, followUpPush),
    };
  }

  function openPushDialog(
    branchOverride?: string,
    precedingCommit?: GitPrecedingCommit,
  ) {
    view = {
      ...view,
      pushPreview: buildShellPushPreview(
        view,
        readonlyMode,
        defaultPushPreview,
        branchOverride,
        precedingCommit,
      ),
    };
  }

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

  function confirmCommit() {
    const res = applyShellCommitConfirm(view, simulatedCommitOutcome);
    if (!res) return;
    view = res.nextView;
    if (res.followUpPush) {
      openPushDialog(res.followUpPush.branch, res.followUpPush.preceding);
    }
  }

  function confirmPush() {
    const push = view.pushPreview;
    if (!push) return;
    view = {
      ...view,
      pushPreview: null,
      statusBanner: `已完成推送确认：${push.sourceBranch} → ${push.remote}/${push.targetBranch}`,
    };
  }

  function inspectCommitInLog(oid: string) {
    view = {
      ...view,
      mode: "log",
      selectedCommitOid: oid,
      selectedCommitFilePath: null,
    };
  }

  function reloadVerifyOutcome() {
    view = {
      ...view,
      lastCommitOutcome: null,
      statusBanner: "已刷新历史与暂存区核实状态",
    };
  }

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

  <Tabs.Root
    value={view.mode}
    onValueChange={setMode}
    orientation={desktop.current ? "vertical" : "horizontal"}
    class="min-h-0 min-w-0 flex-1"
  >
    <GitWorkbenchModeTabs desktop={desktop.current} />

    <Tabs.Content
      value="log"
      class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
    >
      <GitLogPane
        context={view.context}
        historyState={view.historyState}
        historyReason={view.historyReason}
        taskBaseline={view.taskBaseline}
        nextCursor={view.nextCursor}
        refsChanged={view.refsChanged}
        historyLoadingMore={view.historyLoadingMore}
        scope={view.scope}
        branchFilter={view.branchFilter}
        searchQuery={view.searchQuery}
        authorFilter={view.authorFilter}
        sinceFilter={view.sinceFilter}
        untilFilter={view.untilFilter}
        pathFilter={view.pathFilter}
        commits={view.commits}
        commitDetails={view.commitDetails}
        selectedCommitOid={view.selectedCommitOid}
        selectedParentByCommit={view.selectedParentByCommit}
        selectedCommitFilePath={view.selectedCommitFilePath}
        selectionNotice={view.selectionNotice}
        compareBaseMarkOid={view.compareBaseMarkOid}
        compareMode={view.compareMode}
        branches={view.branches}
        checkedOutBranch={view.context.checkedOutBranch}
        onScopeChange={(scope) => (view = { ...view, scope })}
        onBranchFilterChange={handleBranchFilter}
        onBranchAction={handleBranchAction}
        onSearchChange={(searchQuery) => (view = { ...view, searchQuery })}
        onAuthorChange={(authorFilter) => (view = { ...view, authorFilter })}
        onSinceChange={(sinceFilter) => (view = { ...view, sinceFilter })}
        onUntilChange={(untilFilter) => (view = { ...view, untilFilter })}
        onPathFilterChange={(pathFilter) => (view = { ...view, pathFilter })}
        onClearFilters={() => (view = applyShellClearFilters(view))}
        onToggleFavorite={(name) =>
          (view = {
            ...view,
            branches: view.branches.map((b) =>
              b.name === name ? { ...b, favorite: !b.favorite } : b,
            ),
          })}
        onSelectCommit={(selectedCommitOid) =>
          (view = {
            ...view,
            selectedCommitOid,
            selectedCommitFilePath: null,
            compareMode: null,
          })}
        onSelectParent={(commitOid, parentOid) =>
          (view = {
            ...view,
            selectedParentByCommit: {
              ...view.selectedParentByCommit,
              [commitOid]: parentOid,
            },
            selectedCommitFilePath: null,
          })}
        onSelectCommitFile={(selectedCommitFilePath) =>
          (view = { ...view, selectedCommitFilePath })}
        onMarkCompareBase={(compareBaseMarkOid) =>
          (view = { ...view, compareBaseMarkOid })}
        onStartCompare={(baseOid, targetOid) =>
          (view = applyShellStartCompare(view, baseOid, targetOid))}
        onSwapCompareDirection={() => {
          const cmp = view.compareMode;
          if (cmp) {
            view = {
              ...view,
              compareMode: {
                ...cmp,
                baseOid: cmp.targetOid,
                targetOid: cmp.baseOid,
              },
            };
          }
        }}
        onExitCompare={() => (view = { ...view, compareMode: null })}
        onSetTaskBaseline={!readonlyMode
          ? (oid) => (view = applyShellSetTaskBaseline(view, oid))
          : undefined}
        onAddReworkItem={!readonlyMode ? handleAddReworkItem : undefined}
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
        hasActiveWorktreeSession={simulatedActiveSessions.length > 0}
        activeSessionCount={simulatedActiveSessions.length}
        onReviewScopeChange={(reviewScope) => (view = { ...view, reviewScope })}
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
      />
    </Tabs.Content>

    <Tabs.Content
      value="branches"
      class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
    >
      <GitBranchesPane
        context={view.context}
        branches={view.branches}
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
  </Tabs.Root>

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
    {view}
    {reworkOpen}
    {handoffMode}
    {reworkItems}
    activeSessions={simulatedActiveSessions}
    bind:selectedSessionId
    onConfirmCommit={confirmCommit}
    onCloseCommitPreview={() => (view = { ...view, commitPreview: null })}
    onConfirmPush={confirmPush}
    onClosePushPreview={() => {
      const hadPreceding = view.pushPreview?.precedingCommit;
      view = {
        ...view,
        pushPreview: null,
        statusBanner: hadPreceding
          ? `本地已提交（${hadPreceding.shortOid}），已取消推送；本地提交已保留。`
          : view.statusBanner,
      };
    }}
    onUpdateReworkNote={(id, note) =>
      (reworkItems = reworkItems.map((it) =>
        it.id === id ? { ...it, userNote: note } : it,
      ))}
    onRemoveReworkItem={(id) =>
      (reworkItems = reworkItems.filter((it) => it.id !== id))}
    onClearRework={() => (reworkItems = [])}
    onCopyReworkMarkdown={() =>
      (view = { ...view, statusBanner: "已复制结构化审阅意见 Markdown" })}
    onDeliverToSession={(session) => {
      reworkOpen = false;
      if (handoffMode === "rework") reworkItems = [];
      view = {
        ...view,
        statusBanner: `已将意见放入会话「${session.title || session.id}」输入草稿（未自动执行）`,
      };
    }}
    onStartNewSessionWithDraft={() => {
      reworkOpen = false;
      view = { ...view, statusBanner: "已打开带预填草稿的新建工作树会话面板" };
    }}
    onCloseRework={() => (reworkOpen = false)}
  />
</div>
