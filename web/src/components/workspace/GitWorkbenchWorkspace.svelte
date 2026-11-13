<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import * as Tabs from "$lib/components/ui/tabs";
  import type {
    GitBranchItem,
    GitWorkbenchMode,
  } from "../../contracts/git-workbench-schemas";
  import type {
    ArchivedSession,
    Repository,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
  import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
  import type { GitDeviceLayout } from "../../domains/workspace/git-workbench-memory";
  import type { createGitWorkbenchController } from "../../hooks/workspace/git-workbench-controller.svelte";
  import GitBranchesPane from "./GitBranchesPane.svelte";
  import GitCommitPane from "./GitCommitPane.svelte";
  import GitConflictView from "./GitConflictView.svelte";
  import GitInProgressBanner from "./GitInProgressBanner.svelte";
  import GitLogPane from "./GitLogPane.svelte";
  import GitStashPane from "./GitStashPane.svelte";
  import GitWorkbenchModeTabs from "./GitWorkbenchModeTabs.svelte";
  import GitWorktreeGuardState from "./GitWorktreeGuardState.svelte";

  let {
    controller,
    repositories,
    repository,
    worktree,
    sessions,
    archivedRecords = [],
    returnSession,
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
    onOpenSession,
    onArchiveSession,
    onRemoveWorktree,
  }: {
    controller: ReturnType<typeof createGitWorkbenchController>;
    repositories: readonly Repository[];
    repository: Repository;
    worktree: Worktree | undefined;
    sessions: readonly SessionInfo[];
    archivedRecords?: readonly ArchivedSession[];
    returnSession?: SessionInfo;
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
    onOpenSession?: (session: SessionInfo) => void;
    onArchiveSession?: (session: SessionInfo) => void;
    onRemoveWorktree?: () => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  const writableWorktree = $derived(
    Boolean(
      worktree &&
      worktree.available &&
      !worktree.prunable &&
      !worktree.locked &&
      worktree.kind !== "bare",
    ),
  );
  const worktreeSessions = $derived(
    worktree
      ? sessions.filter((s) => s.workspace?.worktreeId === worktree.id)
      : [],
  );
  const activeWorktreeSessions = $derived(
    worktreeSessions.filter((s) => s.status !== "exited"),
  );
  const exitedWorktreeSessions = $derived(
    worktreeSessions.filter((s) => s.status === "exited"),
  );
  const archivedWorktreeCount = $derived(
    worktree
      ? archivedRecords.filter((r) => r.workspace?.worktreeId === worktree.id)
          .length
      : 0,
  );
  const preferredReturnSession = $derived(
    returnSession ?? activeWorktreeSessions[0] ?? undefined,
  );
</script>

<div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
  {#if controller.divergence.inProgress?.active}
    <GitInProgressBanner
      operation={controller.divergence.inProgress}
      busy={controller.divergence.submitting}
      conflictResolverOpen={controller.conflict.open}
      {hasActiveWorktreeSession}
      onOpenConflictResolver={(path) =>
        void controller.conflict.openResolver(path)}
      onContinue={() => void controller.divergence.continueOperation()}
      onOpenAbortPreview={() => void controller.divergence.openAbortPreview()}
      {onOpenWorktreeTerminal}
    />
  {/if}

  {#if controller.conflict.open}
    <GitConflictView
      conflict={controller.conflict}
      inProgress={controller.divergence.inProgress}
      divergenceBusy={controller.divergence.submitting}
      onContinueOperation={() => void controller.divergence.continueOperation()}
      onClose={() => controller.conflict.closeResolver()}
    />
  {:else}
    <Tabs.Root
      value={controller.view.mode}
      onValueChange={(next) => {
        if (
          next === "log" ||
          next === "commit" ||
          next === "branches" ||
          next === "stash"
        ) {
          controller.setMode(next);
        }
      }}
      orientation={desktop.current ? "vertical" : "horizontal"}
      class="min-h-0 min-w-0 flex-1"
    >
      <GitWorkbenchModeTabs
        desktop={desktop.current}
        stashCount={controller.stash.entries.length}
      />

      <Tabs.Content
        value="log"
        class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
      >
        <GitLogPane
          context={controller.view.context}
          historyState={controller.view.historyState}
          historyReason={controller.view.historyReason}
          taskBaseline={controller.view.taskBaseline}
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
          onDiffHeightChange={(diffHeight) =>
            onUpdateDeviceLayout({ diffHeight })}
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
          onStartCompare={(bOid, tOid) =>
            void controller.startCompare(bOid, tOid)}
          onLoadMoreCompareFiles={() => void controller.loadMoreCompareFiles()}
          onSwapCompareDirection={() => controller.swapCompareDirection()}
          onExitCompare={() => controller.exitCompare()}
          onSetTaskBaseline={writableWorktree
            ? (oid) => void controller.taskReview.setBaseline(oid)
            : undefined}
          onCherryPickCommit={writableWorktree
            ? (oid) => void controller.divergence.openCherryPickPreview(oid)
            : undefined}
          onRevertCommit={writableWorktree
            ? (oid) => void controller.divergence.openRevertPreview(oid)
            : undefined}
          onCreateTagCommit={(oid, subject) =>
            controller.refsManage.openCreateTagDialog(
              oid,
              `${oid.slice(0, 7)} · ${subject}`,
            )}
          onInspectCommitFile={(path, tab) =>
            controller.fileInspector.openInspector({
              path,
              tab,
              rev: controller.view.selectedCommitOid ?? undefined,
            })}
          onAddReworkItem={writableWorktree
            ? (input, opts) => {
                controller.rework.addItem(input, opts);
                if (opts?.openSheet) {
                  controller.rework.syncDefaultSessionSelection(
                    activeWorktreeSessions,
                  );
                }
              }
            : undefined}
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
            reviewScope={controller.view.reviewScope}
            taskReview={controller.view.taskReview}
            taskDiffReader={controller.taskReview.diffReader}
            stagingDiff={controller.staging.reader}
            stagingBusy={controller.stagingBusy}
            uncertainBlocked={controller.operations.hasUncertainUnresolved ||
              controller.view.lastCommitOutcome?.status === "uncertain"}
            draftSaving={controller.commitDraft.saving}
            draftRevision={controller.commitDraft.revision}
            draftConflict={controller.commitDraft.conflict}
            commitAmend={controller.commitOptions.amend}
            commitAmendPushed={controller.commitOptions.amendPushed}
            commitAuthorOverride={controller.commitOptions.authorOverride}
            commitSignoff={controller.commitOptions.signoff}
            commitSignoffTrailer={controller.commitOptions.signoffTrailer}
            amendDraftPrompt={controller.commitOptions.draftPrompt}
            activeDiffStale={controller.staging.activeDiffStale}
            activeDiffStaleReason={controller.staging.activeDiffStaleReason}
            uncommittedProgress={controller.reviews.uncommittedProgress}
            taskProgress={controller.reviews.taskProgress}
            liveWorktreeIdentityForPath={(path) =>
              controller.reviews.liveWorktreeIdentityForPath(path)}
            {hasActiveWorktreeSession}
            activeSessionCount={activeWorktreeSessions.length}
            exitedSessionCount={exitedWorktreeSessions.length}
            archivedSessionCount={archivedWorktreeCount}
            paneSizes={deviceLayout.commitPaneSizes}
            diffMode={deviceLayout.diffMode}
            listScrollTop={controller.scrollOffsets.commitList}
            diffScrollTop={controller.scrollOffsets.commitDiff}
            onPaneSizesChange={(commitPaneSizes) =>
              onUpdateDeviceLayout({ commitPaneSizes })}
            onResetLayout={onResetDeviceLayout}
            onDiffModeChange={(diffMode) => onUpdateDeviceLayout({ diffMode })}
            onListScrollChange={(top) =>
              controller.setScroll("commitList", top)}
            onDiffScrollChange={(top) =>
              controller.setScroll("commitDiff", top)}
            onReviewScopeChange={(s) => controller.taskReview.setReviewScope(s)}
            onSelectChange={(id) => controller.selectChange(id)}
            onToggleReviewed={(change) => controller.toggleReviewed(change)}
            onToggleTaskReviewed={(path) =>
              void controller.toggleTaskReviewed(path)}
            onJumpNextUnreviewed={(scope) =>
              controller.jumpNextUnreviewed(scope)}
            onAddReworkItem={(input, opts) => {
              controller.rework.addItem(input, opts);
              if (opts?.openSheet) {
                controller.rework.syncDefaultSessionSelection(
                  activeWorktreeSessions,
                );
              }
            }}
            onStageChange={(id) => void controller.stageOrUnstage("stage", id)}
            onUnstageChange={(id) =>
              void controller.stageOrUnstage("unstage", id)}
            onStageGroup={(_op, group) =>
              void controller.stageOrUnstageGroup(group)}
            onStageHunk={(id, hId) =>
              void controller.stageOrUnstage("stage", id, hId)}
            onUnstageHunk={(id, hId) =>
              void controller.stageOrUnstage("unstage", id, hId)}
            onStageLines={(id, sel) =>
              void controller.stageOrUnstage("stage", id, undefined, sel)}
            onUnstageLines={(id, sel) =>
              void controller.stageOrUnstage("unstage", id, undefined, sel)}
            onDiscardChanges={(ids, paths) =>
              controller.discard.openDiscardFiles(ids, paths)}
            onDiscardHunk={(id, hId) =>
              controller.discard.openDiscardHunk(id, hId)}
            onDiscardLines={(id, sel) =>
              controller.discard.openDiscardLines(id, sel)}
            onRefreshSnapshot={() => void controller.refreshActiveSnapshot()}
            onCommitMessageChange={(msg) => controller.setCommitMessage(msg)}
            onOpenCommitPreview={(followUpPush) =>
              void controller.openCommitPreview(followUpPush)}
            onOpenPush={() => onOpenPush()}
            {onCreateBranchFromHead}
            onResolveDraftKeepLocal={() =>
              void controller.commitDraft.resolveKeepLocal()}
            onResolveDraftAdoptRemote={() =>
              controller.commitDraft.resolveAdoptRemote()}
            onResolveDraftMergeBoth={() =>
              void controller.commitDraft.resolveMergeBoth()}
            onRequestCommitAssist={() =>
              controller.rework.openCommitAssistSheet(activeWorktreeSessions)}
            onToggleCommitAmend={(checked) =>
              void controller.commitOptions.toggleAmend(checked)}
            onResolveAmendDraft={(choice) =>
              controller.commitOptions.resolveAmendDraft(choice)}
            onCommitAuthorOverrideChange={(v) =>
              controller.commitOptions.setAuthorOverride(v)}
            onToggleCommitSignoff={(checked) =>
              controller.commitOptions.setSignoff(checked)}
            onOpenStashCreate={(incUntracked) =>
              controller.stash.dialogs.openCreateDialog(incUntracked)}
            onInspectFile={(path, tab) =>
              controller.fileInspector.openInspector({ path, tab })}
            onOpenConflictResolver={(path) =>
              void controller.conflict.openResolver(path)}
            {onOpenWorktreeTerminal}
            {onNewWorktreeTerminal}
            onSelectTaskFile={(p) => controller.taskReview.selectFile(p)}
            onTaskSearchChange={(q) => controller.taskReview.setSearchQuery(q)}
            onLoadMoreTaskFiles={() => void controller.taskReview.loadMore()}
            onRefreshTaskReview={() => void controller.taskReview.refresh()}
            onSetTaskBaseline={(oid) =>
              void controller.taskReview.setBaseline(oid)}
            onClearTaskBaseline={() =>
              void controller.taskReview.clearBaseline()}
            onOpenLogToPickBaseline={() => {
              if (controller.view.scope === "task") controller.setScope("all");
              controller.setMode("log");
            }}
            onInspectCommit={(oid) => controller.inspectCommitInLog(oid)}
            onReturnSession={preferredReturnSession && onOpenSession
              ? () => onOpenSession(preferredReturnSession)
              : undefined}
            onArchiveSession={exitedWorktreeSessions[0] && onArchiveSession
              ? () => onArchiveSession(exitedWorktreeSessions[0]!)
              : undefined}
            {onRemoveWorktree}
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
          remotes={controller.view.remotes}
          remotesController={controller.refsManage.remotes}
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
          onOpenCreateTagFromHead={controller.view.context.headOid
            ? () =>
                controller.refsManage.openCreateTagDialog(
                  controller.view.context.headOid!,
                  controller.view.context.checkedOutBranch || "HEAD",
                )
            : undefined}
        />
      </Tabs.Content>

      <Tabs.Content
        value="stash"
        class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
      >
        <GitStashPane
          context={controller.view.context}
          entries={controller.stash.entries}
          selectedStashOid={controller.stash.selectedStashOid}
          detail={controller.stash.detail}
          selectedFileKey={controller.stash.selectedFileKey}
          diffReader={controller.stash.reader}
          loadingList={controller.stash.loadingList}
          loadingDetail={controller.stash.loadingDetail}
          diffMode={deviceLayout.diffMode}
          onDiffModeChange={(diffMode) => onUpdateDeviceLayout({ diffMode })}
          onRefresh={() => void controller.stash.refreshList()}
          onSelectStash={(oid) => void controller.stash.selectStash(oid)}
          onSelectFile={(f) => controller.stash.selectFile(f)}
          onOpenCreate={writableWorktree
            ? () => controller.stash.dialogs.openCreateDialog(false)
            : undefined}
          onOpenApply={writableWorktree
            ? (entry, mode) =>
                controller.stash.dialogs.openApplyDialog(entry, mode)
            : undefined}
          onOpenDrop={(entry) =>
            void controller.stash.dialogs.openDropDialog(entry)}
        />
      </Tabs.Content>
    </Tabs.Root>
  {/if}
</div>
