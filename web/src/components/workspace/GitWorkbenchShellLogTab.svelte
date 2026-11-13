<script lang="ts">
  import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
  import {
    applyShellClearFilters,
    applyShellStartCompare,
  } from "../../domains/workspace/git-shell-actions";
  import GitLogPane from "./GitLogPane.svelte";
  import type { GitLogPaneProps } from "./git-log-pane-types";

  let {
    view = $bindable(),
    readonlyMode = false,
    onBranchFilter,
    onBranchAction,
    onSetTaskBaseline,
    onAddReworkItem,
    onRequestCheckout,
    onOpenOccupiedWorktree,
  }: {
    view: GitWorkbenchViewState;
    readonlyMode?: boolean;
    onBranchFilter: GitLogPaneProps["onBranchFilterChange"];
    onBranchAction: GitLogPaneProps["onBranchAction"];
    onSetTaskBaseline?: GitLogPaneProps["onSetTaskBaseline"];
    onAddReworkItem?: GitLogPaneProps["onAddReworkItem"];
    onRequestCheckout: GitLogPaneProps["onRequestCheckout"];
    onOpenOccupiedWorktree: GitLogPaneProps["onOpenOccupiedWorktree"];
  } = $props();
</script>

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
  onBranchFilterChange={onBranchFilter}
  {onBranchAction}
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
  onSetTaskBaseline={!readonlyMode ? onSetTaskBaseline : undefined}
  onAddReworkItem={!readonlyMode ? onAddReworkItem : undefined}
  onCherryPickCommit={(oid) =>
    (view = {
      ...view,
      statusBanner: `已打开遴选预览：${oid.slice(0, 7)}`,
    })}
  onRevertCommit={(oid) =>
    (view = {
      ...view,
      statusBanner: `已打开还原预览：${oid.slice(0, 7)}`,
    })}
  onCreateTagCommit={(oid, subject) =>
    (view = {
      ...view,
      statusBanner: `已打开新建标签对话框：${oid.slice(0, 7)} · ${subject}`,
    })}
  onInspectCommitFile={() => (view = { ...view, fileInspectorOpen: true })}
  {onRequestCheckout}
  {onOpenOccupiedWorktree}
/>
