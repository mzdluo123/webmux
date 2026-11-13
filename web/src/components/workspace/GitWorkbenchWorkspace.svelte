<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import * as Tabs from "$lib/components/ui/tabs";
  import type { GitWorkbenchMode } from "../../contracts/git-workbench-schemas";
  import type {
    Repository,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
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
    onUpdateDeviceLayout,
    onResetDeviceLayout,
    onAddRepository,
    onSelectRepository,
    onSwitchWorktree,
    onNewWorktree,
    onManageWorktrees,
    onRequestCheckout,
    onOpenPush,
  }: {
    controller: ReturnType<typeof createGitWorkbenchController>;
    repositories: readonly Repository[];
    repository: Repository;
    worktree: Worktree | undefined;
    sessions: readonly SessionInfo[];
    commitGuard: "pick-worktree" | "broken-worktree" | null;
    canAddRepository: boolean;
    deviceLayout: GitDeviceLayout;
    onUpdateDeviceLayout: (patch: Partial<GitDeviceLayout>) => void;
    onResetDeviceLayout: () => void;
    onAddRepository: () => void;
    onSelectRepository: (repoId: string) => void;
    onSwitchWorktree: (wtId?: string, forceMode?: GitWorkbenchMode) => void;
    onNewWorktree: () => void;
    onManageWorktrees: () => void;
    onRequestCheckout: (branch: string) => void;
    onOpenPush: (branch?: string) => void;
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
      historyState={controller.view.historyState}
      historyReason={controller.view.historyReason}
      scope={controller.view.scope}
      branchFilter={controller.view.branchFilter}
      searchQuery={controller.view.searchQuery}
      pathFilter={controller.view.pathFilter}
      commits={controller.view.commits}
      commitDetails={controller.view.commitDetails}
      selectedCommitOid={controller.view.selectedCommitOid}
      selectedParentByCommit={controller.view.selectedParentByCommit}
      selectedCommitFilePath={controller.view.selectedCommitFilePath}
      branches={controller.view.branches}
      checkedOutBranch={controller.view.context.checkedOutBranch}
      preferredShowBranches={deviceLayout.showBranchesPanel}
      showAuthorColumn={deviceLayout.showAuthorColumn}
      threePaneSizes={deviceLayout.logThreePaneSizes}
      twoPaneSizes={deviceLayout.logTwoPaneSizes}
      diffMode={deviceLayout.diffMode}
      listScrollTop={controller.scrollOffsets.logList}
      diffScrollTop={controller.scrollOffsets.logDiff}
      onShowBranchesChange={(showBranchesPanel) =>
        onUpdateDeviceLayout({ showBranchesPanel })}
      onThreePaneSizesChange={(logThreePaneSizes) =>
        onUpdateDeviceLayout({ logThreePaneSizes })}
      onTwoPaneSizesChange={(logTwoPaneSizes) =>
        onUpdateDeviceLayout({ logTwoPaneSizes })}
      onResetLayout={onResetDeviceLayout}
      onDiffModeChange={(diffMode) => onUpdateDeviceLayout({ diffMode })}
      onListScrollChange={(top) => controller.setScroll("logList", top)}
      onDiffScrollChange={(top) => controller.setScroll("logDiff", top)}
      onScopeChange={(scope) =>
        controller.setScope(scope, repository, worktree, sessions)}
      onBranchFilterChange={(branch) =>
        controller.setBranchFilter(branch, repository, worktree, sessions)}
      onSearchChange={(q) => controller.setSearchQuery(q)}
      onPathFilterChange={(p) => controller.setPathFilter(p)}
      onSelectCommit={(oid) => controller.selectCommit(oid)}
      onSelectParent={(cOid, pOid) => controller.selectParent(cOid, pOid)}
      onSelectCommitFile={(p) => controller.selectCommitFile(p)}
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
        onToggleReviewed={(p) => controller.toggleReviewed(p)}
        onStageChange={(id) => void controller.stageOrUnstage("stage", id)}
        onUnstageChange={(id) => void controller.stageOrUnstage("unstage", id)}
        onStageHunk={(id, hId) =>
          void controller.stageOrUnstage("stage", id, hId)}
        onUnstageHunk={(id, hId) =>
          void controller.stageOrUnstage("unstage", id, hId)}
        onCommitMessageChange={(msg) => controller.setCommitMessage(msg)}
        onOpenCommitPreview={(followUpPush) =>
          void controller.openCommitPreview(followUpPush)}
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
    />
  </Tabs.Content>
</Tabs.Root>
