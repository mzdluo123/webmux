<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import * as Resizable from "$lib/components/ui/resizable";
  import {
    filterLogCommits,
    resolveCommitParentFiles,
  } from "../../domains/workspace/git-workbench-state";
  import {
    dragHeight,
    dragWidth,
    keyHeight,
    keyWidth,
  } from "../../hooks/shared/splitter";
  import GitLogBranchTree from "./GitLogBranchTree.svelte";
  import GitLogDetailPane from "./GitLogDetailPane.svelte";
  import GitLogDiffPanel from "./GitLogDiffPanel.svelte";
  import GitLogFilesPane from "./GitLogFilesPane.svelte";
  import GitLogListPane from "./GitLogListPane.svelte";
  import type { GitLogPaneProps } from "./git-log-pane-types";

  const TREE_BOUNDS = { min: 180, max: 360 };
  const DIFF_BOUNDS = { min: 140, max: 640 };

  let props: GitLogPaneProps = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  const multiColumn = new MediaQuery("(min-width: 640px)");

  let localShowBranches = $state(true);
  let localTreeWidth = $state(240);
  let localDiffHeight = $state(280);
  let localDiffCollapsed = $state(false);
  let refDrawerOpen = $state(false);
  let diffMaximized = $state(false);
  let mobileStep = $state<"list" | "detail" | "diff">("list");
  let paneResetKey = $state(0);
  let diffPanelRef = $state<ReturnType<typeof GitLogDiffPanel>>();

  const showBranchesPanel = $derived(
    props.preferredShowBranches ?? localShowBranches,
  );
  const effectiveTreeWidth = $derived(
    props.onBranchTreeWidthChange
      ? (props.branchTreeWidth ?? 240)
      : localTreeWidth,
  );
  const effectiveDiffHeight = $derived(
    props.onDiffHeightChange ? (props.diffHeight ?? 280) : localDiffHeight,
  );
  const effectiveDiffCollapsed = $derived(
    props.onDiffCollapsedChange
      ? (props.diffCollapsed ?? false)
      : localDiffCollapsed,
  );
  const twoPaneSizes = $derived(props.twoPaneSizes ?? [58, 42]);
  const diffMode = $derived(props.diffMode ?? "unified");

  const visibleCommits = $derived(
    props.liveFiltered
      ? props.commits
      : filterLogCommits(
          props.commits,
          props.branchFilter,
          props.searchQuery,
          props.pathFilter,
          {
            authorFilter: props.authorFilter,
            sinceFilter: props.sinceFilter,
            untilFilter: props.untilFilter,
          },
        ),
  );
  const selectedDetail = $derived(
    props.selectedCommitOid
      ? (props.commitDetails[props.selectedCommitOid] ?? null)
      : null,
  );
  const resolvedParent = $derived(
    selectedDetail
      ? resolveCommitParentFiles(
          selectedDetail,
          props.selectedCommitOid
            ? props.selectedParentByCommit[props.selectedCommitOid]
            : undefined,
        )
      : { selectedParent: "", files: [] },
  );
  const activeFile = $derived(
    resolvedParent.files.find(
      (f) => (f.newPath || f.oldPath) === props.selectedCommitFilePath,
    ) ??
      resolvedParent.files[0] ??
      null,
  );

  function setTreeWidth(raw: number) {
    const clamped = Math.min(
      TREE_BOUNDS.max,
      Math.max(TREE_BOUNDS.min, Math.round(raw)),
    );
    localTreeWidth = clamped;
    props.onBranchTreeWidthChange?.(clamped);
  }

  function setDiffHeight(raw: number) {
    const clamped = Math.min(
      DIFF_BOUNDS.max,
      Math.max(DIFF_BOUNDS.min, Math.round(raw)),
    );
    localDiffHeight = clamped;
    props.onDiffHeightChange?.(clamped);
  }

  function setDiffCollapsed(next: boolean) {
    localDiffCollapsed = next;
    props.onDiffCollapsedChange?.(next);
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if (!diffMaximized || e.key !== "Escape") return;
    if (diffPanelRef?.hasOpenReaderPanel()) return;
    e.preventDefault();
    diffMaximized = false;
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#snippet commitListSection()}
  <GitLogListPane
    desktop={desktop.current}
    historyState={props.historyState}
    historyReason={props.historyReason}
    taskBaseline={props.taskBaseline}
    nextCursor={props.nextCursor}
    refsChanged={props.refsChanged}
    historyLoadingMore={props.historyLoadingMore}
    scope={props.scope}
    branchFilter={props.branchFilter}
    searchQuery={props.searchQuery}
    authorFilter={props.authorFilter}
    sinceFilter={props.sinceFilter}
    untilFilter={props.untilFilter}
    pathFilter={props.pathFilter}
    {visibleCommits}
    selectedCommitOid={props.selectedCommitOid}
    selectionNotice={props.selectionNotice}
    compareBaseMarkOid={props.compareBaseMarkOid}
    compareMode={props.compareMode}
    {showBranchesPanel}
    showAuthorColumn={props.showAuthorColumn ?? true}
    listScrollTop={props.listScrollTop ?? 0}
    onListScrollChange={props.onListScrollChange}
    onOpenRefDrawer={() => (refDrawerOpen = true)}
    onToggleBranchesPanel={() => {
      const next = !showBranchesPanel;
      localShowBranches = next;
      props.onShowBranchesChange?.(next);
    }}
    onResetLayout={() => {
      localTreeWidth = 240;
      localDiffHeight = 280;
      localDiffCollapsed = false;
      props.onResetLayout?.();
      paneResetKey++;
    }}
    onRefreshHistory={props.onRefreshHistory}
    onLoadMoreHistory={props.onLoadMoreHistory}
    onScopeChange={props.onScopeChange}
    onSearchChange={props.onSearchChange}
    onAuthorChange={props.onAuthorChange}
    onSinceChange={props.onSinceChange}
    onUntilChange={props.onUntilChange}
    onPathFilterChange={props.onPathFilterChange}
    onClearFilters={props.onClearFilters}
    onJumpToHead={props.onJumpToHead}
    onDismissSelectionNotice={props.onDismissSelectionNotice}
    onSelectCommit={(oid) => {
      props.onSelectCommit(oid);
      if (!multiColumn.current) mobileStep = "detail";
    }}
    onMarkCompareBase={props.onMarkCompareBase}
    onStartCompare={props.onStartCompare
      ? (baseOid, targetOid) => {
          props.onStartCompare?.(baseOid, targetOid);
          if (!multiColumn.current) mobileStep = "detail";
        }
      : undefined}
    onSetTaskBaseline={props.onSetTaskBaseline}
    onCherryPickCommit={props.onCherryPickCommit}
    onRevertCommit={props.onRevertCommit}
  />
{/snippet}

{#snippet detailPaneContent()}
  <GitLogDetailPane
    detail={selectedDetail}
    selectedParentOverride={props.selectedCommitOid
      ? props.selectedParentByCommit[props.selectedCommitOid]
      : undefined}
    compareMode={props.compareMode}
    commits={props.commits}
    loading={props.detailLoading}
    onSelectParent={props.onSelectParent}
    onJumpToCommit={props.onSelectCommit}
    onCherryPickCommit={props.onCherryPickCommit}
    onRevertCommit={props.onRevertCommit}
    onCreateTagCommit={props.onCreateTagCommit}
  />
{/snippet}

{#snippet filesPaneContent(isPhone: boolean)}
  <GitLogFilesPane
    files={resolvedParent.files}
    selectedFilePath={props.selectedCommitFilePath}
    leftEndpoint={resolvedParent.selectedParent
      ? resolvedParent.selectedParent.slice(0, 7)
      : "空树"}
    rightEndpoint={selectedDetail?.shortOid ?? ""}
    loading={props.detailLoading}
    compareMode={props.compareMode}
    phone={isPhone}
    onSelectFile={props.onSelectCommitFile}
    onPreviewFile={() => {
      if (isPhone) mobileStep = "diff";
      else if (effectiveDiffCollapsed) setDiffCollapsed(false);
    }}
    onExpandFile={(path) => {
      props.onSelectCommitFile(path);
      if (isPhone) mobileStep = "diff";
      else diffMaximized = true;
    }}
    onInspectFile={props.onInspectCommitFile}
    onSwapCompareDirection={props.onSwapCompareDirection}
    onExitCompare={props.onExitCompare
      ? () => {
          props.onExitCompare?.();
          if (isPhone) mobileStep = "list";
        }
      : undefined}
    onLoadMoreCompareFiles={props.onLoadMoreCompareFiles}
  />
{/snippet}

{#snippet branchTreeContent(inDrawer: boolean)}
  <GitLogBranchTree
    branches={props.branches}
    context={props.context}
    branchFilter={props.branchFilter}
    checkedOutBranch={props.checkedOutBranch}
    selectedCommitOid={props.selectedCommitOid}
    onBranchFilterChange={(branch) => {
      props.onBranchFilterChange(branch);
      if (inDrawer) refDrawerOpen = false;
    }}
    onBranchAction={props.onBranchAction
      ? (kind, branch) => {
          if (inDrawer) refDrawerOpen = false;
          props.onBranchAction?.(kind, branch);
        }
      : undefined}
    onToggleFavorite={props.onToggleFavorite}
    onCompareWithSelected={props.onStartCompare && props.selectedCommitOid
      ? (refHead) => {
          props.onStartCompare?.(refHead, props.selectedCommitOid!);
          if (inDrawer) refDrawerOpen = false;
          if (!multiColumn.current) mobileStep = "detail";
        }
      : undefined}
    onRequestCheckout={(branch) => {
      if (inDrawer) refDrawerOpen = false;
      props.onRequestCheckout(branch);
    }}
    onOpenOccupiedWorktree={(wtId, path) => {
      if (inDrawer) refDrawerOpen = false;
      props.onOpenOccupiedWorktree(wtId, path);
    }}
  />
{/snippet}

{#if multiColumn.current}
  {#if diffMaximized}
    <GitLogDiffPanel
      bind:this={diffPanelRef}
      logDiff={props.logDiff}
      fallbackFile={activeFile}
      fallbackLeftLabel={resolvedParent.selectedParent.slice(0, 7)}
      fallbackRightLabel={selectedDetail?.shortOid ?? ""}
      presentation="maximized"
      {diffMode}
      onAddReworkItem={props.onAddReworkItem}
      onDiffModeChange={props.onDiffModeChange}
      onToggleMaximized={() => (diffMaximized = false)}
      onBack={() => (diffMaximized = false)}
    />
  {:else}
    <div class="flex h-full min-h-0 min-w-0 flex-col">
      <div class="flex min-h-0 min-w-0 flex-1 overflow-hidden">
        {#if desktop.current && showBranchesPanel}
          <div
            class="git-branch-tree-pane h-full min-h-0 overflow-hidden"
            style:--git-branch-tree-width={`${effectiveTreeWidth}px`}
          >
            {@render branchTreeContent(false)}
          </div>
          <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
          <div
            class="git-v-splitter"
            role="separator"
            aria-orientation="vertical"
            aria-label="调整引用筛选栏宽度"
            title="拖动调整引用筛选栏宽度（双击恢复默认宽度）"
            aria-valuemin={TREE_BOUNDS.min}
            aria-valuemax={TREE_BOUNDS.max}
            aria-valuenow={effectiveTreeWidth}
            tabindex="0"
            ondblclick={() => setTreeWidth(240)}
            onpointerdown={(e) =>
              dragWidth(e, effectiveTreeWidth, setTreeWidth)}
            onkeydown={(e) =>
              keyWidth(e, effectiveTreeWidth, TREE_BOUNDS, setTreeWidth)}
          ></div>
        {/if}

        <div class="min-h-0 min-w-0 flex-1 overflow-hidden">
          {#key paneResetKey}
            <Resizable.PaneGroup
              direction="horizontal"
              class="h-full min-h-0 min-w-0"
              onLayoutChange={(sizes) => {
                if (sizes.length === 2) {
                  props.onTwoPaneSizesChange?.([sizes[0]!, sizes[1]!]);
                }
              }}
            >
              <Resizable.Pane
                defaultSize={twoPaneSizes[0]}
                minSize={32}
                class="git-pane"
              >
                {@render commitListSection()}
              </Resizable.Pane>
              <Resizable.Handle withHandle class="git-divider" />
              <Resizable.Pane
                defaultSize={twoPaneSizes[1]}
                minSize={26}
                class="git-pane"
              >
                <Resizable.PaneGroup
                  direction="vertical"
                  class="h-full min-h-0 min-w-0"
                >
                  <Resizable.Pane
                    defaultSize={52}
                    minSize={25}
                    class="git-pane"
                  >
                    {@render filesPaneContent(false)}
                  </Resizable.Pane>
                  <Resizable.Handle withHandle class="git-divider" />
                  <Resizable.Pane
                    defaultSize={48}
                    minSize={24}
                    class="git-pane"
                  >
                    {@render detailPaneContent()}
                  </Resizable.Pane>
                </Resizable.PaneGroup>
              </Resizable.Pane>
            </Resizable.PaneGroup>
          {/key}
        </div>
      </div>

      {#if !effectiveDiffCollapsed}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
        <div
          class="git-h-splitter"
          role="separator"
          aria-orientation="horizontal"
          aria-label="调整底部差异面板高度"
          title="拖动调整底部差异面板高度（双击恢复默认高度）"
          aria-valuemin={DIFF_BOUNDS.min}
          aria-valuemax={DIFF_BOUNDS.max}
          aria-valuenow={effectiveDiffHeight}
          tabindex="0"
          ondblclick={() => setDiffHeight(280)}
          onpointerdown={(e) =>
            dragHeight(e, effectiveDiffHeight, setDiffHeight, true)}
          onkeydown={(e) =>
            keyHeight(e, effectiveDiffHeight, DIFF_BOUNDS, setDiffHeight, true)}
        ></div>
      {/if}

      <div
        class={effectiveDiffCollapsed
          ? "shrink-0 border-t border-git-border"
          : "git-log-diff-dock min-w-0 overflow-hidden"}
        style:--git-log-diff-height={effectiveDiffCollapsed
          ? undefined
          : `${effectiveDiffHeight}px`}
      >
        <GitLogDiffPanel
          bind:this={diffPanelRef}
          logDiff={props.logDiff}
          fallbackFile={activeFile}
          fallbackLeftLabel={resolvedParent.selectedParent.slice(0, 7)}
          fallbackRightLabel={selectedDetail?.shortOid ?? ""}
          presentation="docked"
          collapsed={effectiveDiffCollapsed}
          {diffMode}
          onAddReworkItem={props.onAddReworkItem}
          onDiffModeChange={props.onDiffModeChange}
          onToggleCollapsed={() => setDiffCollapsed(!effectiveDiffCollapsed)}
          onToggleMaximized={() => {
            setDiffCollapsed(false);
            diffMaximized = true;
          }}
        />
      </div>
    </div>
  {/if}
{:else if mobileStep === "list"}
  {@render commitListSection()}
{:else if mobileStep === "detail"}
  <div class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface">
    <div class="border-b border-git-border bg-git-panel px-2.5 py-1.5">
      <Button size="xs" variant="ghost" onclick={() => (mobileStep = "list")}>
        <IconArrowLeft aria-hidden="true" />
        返回提交列表
      </Button>
    </div>
    <div class="min-h-0 flex-1 overflow-y-auto">
      <div class="border-b border-git-border">
        {@render detailPaneContent()}
      </div>
      {@render filesPaneContent(true)}
    </div>
  </div>
{:else}
  <GitLogDiffPanel
    bind:this={diffPanelRef}
    logDiff={props.logDiff}
    fallbackFile={activeFile}
    fallbackLeftLabel={resolvedParent.selectedParent.slice(0, 7)}
    fallbackRightLabel={selectedDetail?.shortOid ?? ""}
    presentation="mobile"
    diffMode="unified"
    onAddReworkItem={props.onAddReworkItem}
    onBack={() => (mobileStep = "detail")}
  />
{/if}

<Drawer.Root direction="left" noBodyStyles bind:open={refDrawerOpen}>
  <Drawer.Content class="w-80 max-w-[85vw] xl:hidden">
    <Drawer.Title class="sr-only">引用与分支筛选</Drawer.Title>
    {@render branchTreeContent(true)}
  </Drawer.Content>
</Drawer.Root>
