<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import * as Tabs from "$lib/components/ui/tabs";
  import type {
    GitBranchItem,
    GitWorkbenchMode,
  } from "../../contracts/git-workbench-schemas";
  import type {
    Repository,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
  import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
  import type { GitDeviceLayout } from "../../domains/workspace/git-workbench-memory";
  import type { createGitWorkbenchController } from "../../hooks/workspace/git-workbench-controller.svelte";
  import GitBranchesPane from "./GitBranchesPane.svelte";
  import GitCommitPane from "./GitCommitPane.svelte";
  import GitLogPane from "./GitLogPane.svelte";
  import GitWorkbenchModeTabs from "./GitWorkbenchModeTabs.svelte";
  import GitWorktreeGuardState from "./GitWorktreeGuardState.svelte";

  let {
    controller,
    repositories,
    repository,
    worktree,
    sessions,
    commitGuard,
    canAddRepository,
    deviceLayout,
    hasActiveWorktreeSession = false,
    onUpdateDeviceLayout,
    onResetDeviceLayout,
    onAddRepository,
    onSelectRepository,
    onSwitchWorktree,
    onNewWorktree,
    onManageWorktrees,
    onRequestCheckout,
    onOpenPush,
    onBranchAction,
    onCreateBranchFromHead,
    onOpenWorktreeTerminal,
    onNewWorktreeTerminal,
  }: {
    controller: ReturnType<typeof createGitWorkbenchController>;
    repositories: readonly Repository[];
    repository: Repository;
    worktree: Worktree | undefined;
    sessions: readonly SessionInfo[];
    commitGuard: "pick-worktree" | "broken-worktree" | null;
    canAddRepository: boolean;
    deviceLayout: GitDeviceLayout;
    hasActiveWorktreeSession?: boolean;
    onUpdateDeviceLayout: (patch: Partial<GitDeviceLayout>) => void;
    onResetDeviceLayout: () => void;
    onAddRepository: () => void;
    onSelectRepository: (repoId: string) => void;
    onSwitchWorktree: (wtId?: string, forceMode?: GitWorkbenchMode) => void;
    onNewWorktree: () => void;
    onManageWorktrees: () => void;
    onRequestCheckout: (branch: string) => void;
    onOpenPush: (branch?: string) => void;
    onBranchAction: (
      kind: GitSharedBranchActionKind,
      branch: GitBranchItem,
    ) => void;
    onCreateBranchFromHead: () => void;
    onOpenWorktreeTerminal: () => void;
    onNewWorktreeTerminal: () => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
</script>

<Tabs.Root
  value={controller.view.mode}
  onValueChange={(next) => {
    if (next === "log" || next === "commit" || next === "branches") {
      controller.setMode(next);
    }
  }}
  orientation={desktop.current ? "vertical" : "horizontal"}
  class="min-h-0 min-w-0 flex-1"
>
  <GitWorkbenchModeTabs desktop={desktop.current} />

  <Tabs.Content value="log" class="min-h-0 min-w-0 flex-1 overflow-hidden p-0">
    <GitLogPane
      context={controller.view.context}
      historyState={controller.view.historyState}
      historyReason={controller.view.historyReason}
      nextCursor={controller.view.nextCursor}
      refsChanged={controller.view.refsChanged}
      historyLoadingMore={controller.view.historyLoadingMore}
      liveFiltered={true}
      detailLoading={controller.detailLoading}
      scope={controller.view.scope}
      branchFilter={controller.view.branchFilter}
      searchQuery={controller.view.searchQuery}
      authorFilter={controller.view.authorFilter}
      sinceFilter={controller.view.sinceFilter}
      untilFilter={controller.view.untilFilter}
      pathFilter={controller.view.pathFilter}
      commits={controller.view.commits}
      commitDetails={controller.view.commitDetails}
      selectedCommitOid={controller.view.selectedCommitOid}
      selectedParentByCommit={controller.view.selectedParentByCommit}
      selectedCommitFilePath={controller.view.selectedCommitFilePath}
      selectionNotice={controller.view.selectionNotice}
      compareBaseMarkOid={controller.view.compareBaseMarkOid}
      compareMode={controller.view.compareMode}
      branches={controller.view.branches}
      checkedOutBranch={controller.view.context.checkedOutBranch}
      logDiff={controller.logDiff}
      preferredShowBranches={deviceLayout.showBranchesPanel}
      branchTreeWidth={deviceLayout.branchTreeWidth}
      diffHeight={deviceLayout.diffHeight}
      diffCollapsed={deviceLayout.diffCollapsed}
      showAuthorColumn={deviceLayout.showAuthorColumn}
      twoPaneSizes={deviceLayout.logTwoPaneSizes}
      diffMode={deviceLayout.diffMode}
      listScrollTop={controller.scrollOffsets.logList}
      diffScrollTop={controller.scrollOffsets.logDiff}
      onShowBranchesChange={(showBranchesPanel) =>
        onUpdateDeviceLayout({ showBranchesPanel })}
      onBranchTreeWidthChange={(branchTreeWidth) =>
        onUpdateDeviceLayout({ branchTreeWidth })}
      onDiffHeightChange={(diffHeight) => onUpdateDeviceLayout({ diffHeight })}
      onDiffCollapsedChange={(diffCollapsed) =>
        onUpdateDeviceLayout({ diffCollapsed })}
      onTwoPaneSizesChange={(logTwoPaneSizes) =>
        onUpdateDeviceLayout({ logTwoPaneSizes })}
      onResetLayout={onResetDeviceLayout}
      onDiffModeChange={(diffMode) => onUpdateDeviceLayout({ diffMode })}
      onListScrollChange={(top) => controller.setScroll("logList", top)}
      onDiffScrollChange={(top) => controller.setScroll("logDiff", top)}
      onScopeChange={(scope) => controller.setScope(scope)}
      onBranchFilterChange={(branch) =>
        controller.setBranchFilter(branch, repository, worktree, sessions)}
      {onBranchAction}
      onSearchChange={(q) => controller.setSearchQuery(q)}
      onAuthorChange={(a) => controller.setAuthorFilter(a)}
      onSinceChange={(s) => controller.setSinceFilter(s)}
      onUntilChange={(u) => controller.setUntilFilter(u)}
      onPathFilterChange={(p) => controller.setPathFilter(p)}
      onClearFilters={() => controller.clearLogFilters()}
      onRefreshHistory={() => void controller.refreshHistory()}
      onLoadMoreHistory={() => void controller.loadMoreHistory()}
      onJumpToHead={() => controller.jumpToHead()}
      onDismissSelectionNotice={() => controller.dismissSelectionNotice()}
      onToggleFavorite={(branch) => controller.toggleFavorite(branch)}
      onSelectCommit={(oid) => controller.selectCommit(oid)}
      onSelectParent={(cOid, pOid) => controller.selectParent(cOid, pOid)}
      onSelectCommitFile={(p) => controller.selectCommitFile(p)}
      onMarkCompareBase={(oid) => controller.markCompareBase(oid)}
      onStartCompare={(bOid, tOid) => void controller.startCompare(bOid, tOid)}
      onLoadMoreCompareFiles={() => void controller.loadMoreCompareFiles()}
      onSwapCompareDirection={() => controller.swapCompareDirection()}
      onExitCompare={() => controller.exitCompare()}
      {onRequestCheckout}
      onOpenOccupiedWorktree={(wtId) => onSwitchWorktree(wtId)}
    />
  </Tabs.Content>

  <Tabs.Content
    value="commit"
    class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
  >
    {#if commitGuard}
      <GitWorktreeGuardState
        kind={commitGuard}
        {repositories}
        {repository}
        {worktree}
        {canAddRepository}
        {onAddRepository}
        {onSelectRepository}
        onSelectWorktree={(wtId) => onSwitchWorktree(wtId, "commit")}
        {onNewWorktree}
        {onManageWorktrees}
        onBackToLog={() => controller.setMode("log")}
      />
    {:else}
      <GitCommitPane
        context={controller.view.context}
        changes={controller.view.workingChanges}
        activeChangeId={controller.view.activeChangeId}
        reviewedPaths={controller.view.reviewedPaths}
        commitMessage={controller.view.commitMessage}
        stagingDiff={controller.staging.reader}
        stagingBusy={controller.stagingBusy}
        activeDiffStale={controller.staging.activeDiffStale}
        activeDiffStaleReason={controller.staging.activeDiffStaleReason}
        {hasActiveWorktreeSession}
        paneSizes={deviceLayout.commitPaneSizes}
        diffMode={deviceLayout.diffMode}
        listScrollTop={controller.scrollOffsets.commitList}
        diffScrollTop={controller.scrollOffsets.commitDiff}
        onPaneSizesChange={(commitPaneSizes) =>
          onUpdateDeviceLayout({ commitPaneSizes })}
        onResetLayout={onResetDeviceLayout}
        onDiffModeChange={(diffMode) => onUpdateDeviceLayout({ diffMode })}
        onListScrollChange={(top) => controller.setScroll("commitList", top)}
        onDiffScrollChange={(top) => controller.setScroll("commitDiff", top)}
        onSelectChange={(id) => controller.selectChange(id)}
        onToggleReviewed={(change) => controller.toggleReviewed(change)}
        onStageChange={(id) => void controller.stageOrUnstage("stage", id)}
        onUnstageChange={(id) => void controller.stageOrUnstage("unstage", id)}
        onStageGroup={(_op, group) =>
          void controller.stageOrUnstageGroup(group)}
        onStageHunk={(id, hId) =>
          void controller.stageOrUnstage("stage", id, hId)}
        onUnstageHunk={(id, hId) =>
          void controller.stageOrUnstage("unstage", id, hId)}
        onRefreshSnapshot={() => void controller.refreshActiveSnapshot()}
        onCommitMessageChange={(msg) => controller.setCommitMessage(msg)}
        onOpenCommitPreview={(followUpPush) =>
          void controller.openCommitPreview(followUpPush)}
        onOpenPush={() => onOpenPush()}
        {onCreateBranchFromHead}
        {onOpenWorktreeTerminal}
        {onNewWorktreeTerminal}
      />
    {/if}
  </Tabs.Content>

  <Tabs.Content
    value="branches"
    class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
  >
    <GitBranchesPane
      context={controller.view.context}
      branches={controller.view.branches}
      branchFilter={controller.view.branchFilter}
      onFilterInLog={(branch) =>
        controller.setBranchFilter(
          branch,
          repository,
          worktree,
          sessions,
          true,
        )}
      {onRequestCheckout}
      onOpenOccupiedWorktree={(wtId) => onSwitchWorktree(wtId)}
      onOpenPush={(branch) => onOpenPush(branch)}
      {onBranchAction}
    />
  </Tabs.Content>
</Tabs.Root>
