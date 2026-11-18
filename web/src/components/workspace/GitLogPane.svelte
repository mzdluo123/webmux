<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import { dockGroup, dockSplit } from "$lib/dock-layout";
  import { m } from "$lib/paraglide/messages.js";
  import {
    filterLogCommits,
    resolveCommitParentFiles,
  } from "../../domains/workspace/git-workbench-state";
  import DockPanel from "../shared/DockPanel.svelte";
  import DockWorkspace from "../shared/DockWorkspace.svelte";
  import GitCompactPanelContent from "./GitCompactPanelContent.svelte";
  import GitLogBranchTree from "./GitLogBranchTree.svelte";
  import GitLogDetailPane from "./GitLogDetailPane.svelte";
  import GitLogDiffPanel from "./GitLogDiffPanel.svelte";
  import GitLogFilesPane from "./GitLogFilesPane.svelte";
  import GitLogListPane from "./GitLogListPane.svelte";
  import type { GitLogPaneProps } from "./git-log-pane-types";

  let props: GitLogPaneProps = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  const multiColumn = new MediaQuery("(min-width: 640px)");

  const commitPanes = dockSplit(
    "vertical",
    dockGroup("files"),
    dockGroup("detail"),
    0.52,
  );
  const initial = $derived(
    dockSplit(
      "vertical",
      desktop.current
        ? dockSplit(
            "horizontal",
            dockGroup("refs"),
            dockSplit("horizontal", dockGroup("list"), commitPanes, 0.6),
            0.19,
          )
        : dockSplit("horizontal", dockGroup("list", "refs"), commitPanes, 0.56),
      dockGroup("diff"),
      desktop.current ? 0.64 : 0.6,
    ),
  );
  let dock = $state<ReturnType<typeof DockWorkspace>>();
  let showPicker = $state(false);
  let mobilePanel = $state("list");
  let backFromDiff = $state<"detail" | "files">("detail");
  let refDrawerOpen = $state(false);
  let drawerRefTarget = $state<HTMLDivElement>();
  let compactFilesTarget = $state<HTMLDivElement>();
  let diffPanelRef = $state<ReturnType<typeof GitLogDiffPanel>>();

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

  function hasVisibleReaderPanel() {
    return dock?.isVisible("diff") && diffPanelRef?.hasOpenReaderPanel();
  }

  function closeLayer() {
    if (
      multiColumn.current ||
      showPicker ||
      refDrawerOpen ||
      !dock ||
      dock.isVisible("list")
    )
      return false;
    dock.show(dock.isVisible("diff") ? backFromDiff : "list");
    return true;
  }

  function handleWindowKeydown(e: KeyboardEvent) {
    if (e.defaultPrevented || e.key !== "Escape" || hasVisibleReaderPanel())
      return;
    if (closeLayer()) e.preventDefault();
  }

  function returnFromRefs() {
    if (refDrawerOpen) refDrawerOpen = false;
    else if (!multiColumn.current) dock?.show("list");
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#snippet commitListSection()}
  <GitLogListPane
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
    showAuthorColumn={multiColumn.current}
    listScrollTop={props.listScrollTop ?? 0}
    onListScrollChange={props.onListScrollChange}
    onOpenRefDrawer={!desktop.current
      ? () => (refDrawerOpen = true)
      : undefined}
    onResetLayout={multiColumn.current ? () => dock?.reset() : undefined}
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
      if (!multiColumn.current) dock?.show("detail");
    }}
    onMarkCompareBase={props.onMarkCompareBase}
    onStartCompare={props.onStartCompare
      ? (baseOid, targetOid) => {
          props.onStartCompare?.(baseOid, targetOid);
          if (!multiColumn.current) dock?.show("detail");
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
  >
    {#snippet afterDetails()}
      <div bind:this={compactFilesTarget}></div>
    {/snippet}
  </GitLogDetailPane>
{/snippet}

{#snippet filesPaneContent()}
  <GitLogFilesPane
    files={resolvedParent.files}
    selectedFilePath={props.selectedCommitFilePath}
    leftEndpoint={resolvedParent.selectedParent
      ? resolvedParent.selectedParent.slice(0, 7)
      : m.s6_empty_tree()}
    rightEndpoint={selectedDetail?.shortOid ?? ""}
    loading={props.detailLoading}
    compareMode={props.compareMode}
    onSelectFile={props.onSelectCommitFile}
    onPreviewFile={() => {
      if (!multiColumn.current)
        backFromDiff = dock?.isVisible("files") ? "files" : "detail";
      if (!dock?.isVisible("diff")) dock?.show("diff");
    }}
    onInspectFile={props.onInspectCommitFile}
    onSwapCompareDirection={props.onSwapCompareDirection}
    onExitCompare={props.onExitCompare
      ? () => {
          props.onExitCompare?.();
          if (!multiColumn.current) dock?.show("list");
        }
      : undefined}
    onLoadMoreCompareFiles={props.onLoadMoreCompareFiles}
  />
{/snippet}

{#snippet branchTreeContent()}
  <GitLogBranchTree
    branches={props.branches}
    context={props.context}
    branchFilter={props.branchFilter}
    checkedOutBranch={props.checkedOutBranch}
    selectedCommitOid={props.selectedCommitOid}
    onBranchFilterChange={(branch) => {
      props.onBranchFilterChange(branch);
      returnFromRefs();
    }}
    onBranchAction={props.onBranchAction
      ? (kind, branch) => {
          returnFromRefs();
          props.onBranchAction?.(kind, branch);
        }
      : undefined}
    onToggleFavorite={props.onToggleFavorite}
    onCompareWithSelected={props.onStartCompare && props.selectedCommitOid
      ? (refHead) => {
          props.onStartCompare?.(refHead, props.selectedCommitOid!);
          returnFromRefs();
          if (!multiColumn.current) dock?.show("detail");
        }
      : undefined}
    onRequestCheckout={(branch) => {
      returnFromRefs();
      props.onRequestCheckout(branch);
    }}
    onOpenOccupiedWorktree={(wtId, path) => {
      returnFromRefs();
      props.onOpenOccupiedWorktree(wtId, path);
    }}
  />
{/snippet}

<div class="h-full min-h-0 min-w-0">
  <DockWorkspace
    {initial}
    storageKey={props.storageKey ?? "git.log"}
    compact={!multiColumn.current}
    bind:mobilePanel
    bind:showPicker
    bind:this={dock}
  >
    <DockPanel id="refs" title={m.s6_log_pane_drawer_title()}>
      {#if !multiColumn.current}
        {@render backToCommitList()}
      {/if}
      <GitCompactPanelContent
        target={refDrawerOpen ? drawerRefTarget : undefined}
      >
        <div class="h-full min-h-0 min-w-0 flex-1 overflow-hidden">
          {@render branchTreeContent()}
        </div>
      </GitCompactPanelContent>
    </DockPanel>

    <DockPanel id="list" title={m.s7_mode_tab_log()}>
      {@render commitListSection()}
    </DockPanel>

    <DockPanel id="detail" title={m.s6_log_detail_commit_heading()}>
      <div class="flex h-full min-h-0 min-w-0 flex-col bg-background">
        {#if !multiColumn.current}
          {@render backToCommitList()}
        {/if}
        {@render detailPaneContent()}
      </div>
    </DockPanel>

    <DockPanel id="files" title={m.s5_files_tab()}>
      {#snippet children(visible)}
        {#if !multiColumn.current}
          {@render backToCommitList()}
        {/if}
        <GitCompactPanelContent
          target={!multiColumn.current && !visible
            ? compactFilesTarget
            : undefined}
        >
          <div
            class="min-h-0 min-w-0"
            class:border-t={!multiColumn.current && !visible}
            class:flex-1={multiColumn.current || visible}
          >
            {@render filesPaneContent()}
          </div>
        </GitCompactPanelContent>
      {/snippet}
    </DockPanel>

    <DockPanel id="diff" title={m.s6_log_diff_preview_placeholder()}>
      <GitLogDiffPanel
        bind:this={diffPanelRef}
        logDiff={props.logDiff}
        fallbackFile={activeFile}
        fallbackLeftLabel={resolvedParent.selectedParent.slice(0, 7)}
        fallbackRightLabel={selectedDetail?.shortOid ?? ""}
        presentation={multiColumn.current ? "docked" : "mobile"}
        onAddReworkItem={props.onAddReworkItem}
        onBack={!multiColumn.current ? closeLayer : undefined}
      />
    </DockPanel>
  </DockWorkspace>
</div>

{#snippet backToCommitList()}
  <div class="border-b bg-background px-2.5 py-1.5">
    <Button size="sm" variant="ghost" onclick={() => dock?.show("list")}>
      <IconArrowLeft aria-hidden="true" />
      {m.s6_back_to_commit_list()}
    </Button>
  </div>
{/snippet}

<Drawer.Root direction="left" noBodyStyles bind:open={refDrawerOpen}>
  <Drawer.Content class="w-80 max-w-[85vw]">
    <Drawer.Title class="sr-only">{m.s6_log_pane_drawer_title()}</Drawer.Title>
    <div class="min-h-0 flex-1" bind:this={drawerRefTarget}></div>
  </Drawer.Content>
</Drawer.Root>
