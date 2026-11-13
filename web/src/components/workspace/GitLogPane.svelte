<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconLayoutSidebar from "~icons/tabler/layout-sidebar";
  import IconRestore from "~icons/tabler/restore";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import * as Resizable from "$lib/components/ui/resizable";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { computeGitGraphLayout } from "../../domains/workspace/git-graph-layout";
  import {
    filterLogCommits,
    resolveCommitParentFiles,
  } from "../../domains/workspace/git-workbench-state";
  import {
    syncElementScrollTop,
    type GitPaneScrollCallbacks,
  } from "../../domains/workspace/git-workbench-memory";
  import type {
    GitBranchItem,
    GitCommitDetail,
    GitHistoryRow,
    GitHistoryScope,
  } from "../../contracts/git-workbench-schemas";
  import GitDiffViewer from "./GitDiffViewer.svelte";
  import GitLogBranchTree from "./GitLogBranchTree.svelte";
  import GitLogCommitRow from "./GitLogCommitRow.svelte";
  import GitLogDetailPane from "./GitLogDetailPane.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    historyState,
    historyReason = "",
    scope,
    branchFilter,
    searchQuery,
    pathFilter,
    commits,
    commitDetails,
    selectedCommitOid,
    selectedParentByCommit,
    selectedCommitFilePath,
    branches,
    checkedOutBranch,
    preferredShowBranches,
    showAuthorColumn = true,
    threePaneSizes = [20, 46, 34],
    twoPaneSizes = [60, 40],
    diffMode,
    listScrollTop = 0,
    diffScrollTop = 0,
    onShowBranchesChange,
    onThreePaneSizesChange,
    onTwoPaneSizesChange,
    onResetLayout,
    onDiffModeChange,
    onListScrollChange,
    onDiffScrollChange,
    onScopeChange,
    onBranchFilterChange,
    onSearchChange,
    onPathFilterChange,
    onSelectCommit,
    onSelectParent,
    onSelectCommitFile,
    onRequestCheckout,
    onOpenOccupiedWorktree,
  }: {
    historyState: "ready" | "empty" | "unborn" | "no_upstream" | "no_worktree";
    historyReason?: string;
    scope: GitHistoryScope;
    branchFilter: string | null;
    searchQuery: string;
    pathFilter: string;
    commits: readonly GitHistoryRow[];
    commitDetails: Record<string, GitCommitDetail>;
    selectedCommitOid: string | null;
    selectedParentByCommit: Record<string, string>;
    selectedCommitFilePath: string | null;
    branches: readonly GitBranchItem[];
    checkedOutBranch: string | null;
    preferredShowBranches?: boolean;
    showAuthorColumn?: boolean;
    threePaneSizes?: [number, number, number];
    twoPaneSizes?: [number, number];
    onShowBranchesChange?: (open: boolean) => void;
    onThreePaneSizesChange?: (sizes: [number, number, number]) => void;
    onTwoPaneSizesChange?: (sizes: [number, number]) => void;
    onScopeChange: (scope: GitHistoryScope) => void;
    onBranchFilterChange: (branch: string | null) => void;
    onSearchChange: (q: string) => void;
    onPathFilterChange: (p: string) => void;
    onSelectCommit: (oid: string) => void;
    onSelectParent: (commitOid: string, parentOid: string) => void;
    onSelectCommitFile: (path: string) => void;
    onRequestCheckout: (branch: string) => void;
    onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
  } & GitPaneScrollCallbacks = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  let localShowBranches = $state(true);
  const showBranchesPanel = $derived(
    preferredShowBranches ?? localShowBranches,
  );
  let mobileStep = $state<"list" | "detail" | "diff">("list");
  let paneResetKey = $state(0);
  let listScrollEl = $state<HTMLDivElement>();

  $effect(() => syncElementScrollTop(listScrollEl, listScrollTop));

  const visibleCommits = $derived(
    filterLogCommits(commits, branchFilter, searchQuery, pathFilter),
  );
  const graphLayouts = $derived(computeGitGraphLayout(visibleCommits));
  const selectedDetail = $derived(
    selectedCommitOid ? (commitDetails[selectedCommitOid] ?? null) : null,
  );
  const resolvedParent = $derived(
    selectedDetail
      ? resolveCommitParentFiles(
          selectedDetail,
          selectedCommitOid
            ? selectedParentByCommit[selectedCommitOid]
            : undefined,
        )
      : { selectedParent: "", files: [] },
  );
  const activeFile = $derived(
    resolvedParent.files.find((f) => f.newPath === selectedCommitFilePath) ??
      resolvedParent.files[0] ??
      null,
  );
</script>

{#snippet commitListPane()}
  <section
    class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
    aria-label="提交历史列表"
  >
    <div class="space-y-2 border-b border-git-border bg-git-panel px-2.5 py-2">
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
        <SegmentedControl.Root
          wrap
          value={scope}
          onValueChange={(val) => {
            if (
              val === "all" ||
              val === "branch" ||
              val === "to-push" ||
              val === "incoming"
            ) {
              onScopeChange(val);
            }
          }}
          aria-label="提交范围"
        >
          <SegmentedControl.Item value="all">全部</SegmentedControl.Item>
          <SegmentedControl.Item value="branch">当前分支</SegmentedControl.Item>
          <SegmentedControl.Item value="to-push">待推送</SegmentedControl.Item>
          <SegmentedControl.Item value="incoming">待拉取</SegmentedControl.Item>
        </SegmentedControl.Root>

        {#if desktop.current}
          <div class="flex items-center gap-1">
            <Button
              size="icon-sm"
              variant="ghost"
              aria-pressed={showBranchesPanel}
              aria-label="切换引用筛选栏"
              title="切换引用筛选栏"
              onclick={() => {
                const next = !showBranchesPanel;
                localShowBranches = next;
                onShowBranchesChange?.(next);
              }}
            >
              <IconLayoutSidebar aria-hidden="true" />
            </Button>
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label="重置分栏比例"
              title="重置分栏比例"
              onclick={() => {
                onResetLayout?.();
                paneResetKey++;
              }}
            >
              <IconRestore aria-hidden="true" />
            </Button>
          </div>
        {/if}
      </div>

      <div class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2">
        <Input
          size="sm"
          value={searchQuery}
          placeholder="搜索提交信息、作者或 OID…"
          aria-label="搜索提交关键字"
          oninput={(e) => onSearchChange(e.currentTarget.value)}
        />
        <Input
          size="sm"
          mono
          value={pathFilter}
          placeholder="按字面路径筛选，例如 internal/workspace"
          aria-label="按路径筛选提交"
          oninput={(e) => onPathFilterChange(e.currentTarget.value)}
        />
      </div>

      {#if !desktop.current && branches.length > 0}
        <div class="flex min-w-0 flex-wrap items-center gap-1.5">
          <Button
            size="xs"
            variant="outline"
            aria-pressed={branchFilter === null}
            onclick={() => onBranchFilterChange(null)}
          >
            全部引用
          </Button>
          {#each branches as branch (branch.name)}
            <Button
              size="xs"
              variant="outline"
              class="max-w-full"
              aria-pressed={branchFilter === branch.name}
              onclick={() => onBranchFilterChange(branch.name)}
            >
              <span class="truncate" title={branch.name}>{branch.name}</span>
            </Button>
          {/each}
        </div>
      {/if}
    </div>

    <div
      bind:this={listScrollEl}
      class="min-h-0 flex-1 overflow-y-auto p-1.5"
      role="listbox"
      aria-label="提交记录"
      onscroll={(e) => onListScrollChange?.(e.currentTarget.scrollTop)}
    >
      {#if historyState !== "ready"}
        <div class="p-2">
          <InlineNotice tone="info">
            {historyReason ||
              (historyState === "unborn"
                ? "当前分支尚未产生首个提交（unborn HEAD）。"
                : historyState === "no_upstream"
                  ? "当前分支尚未关联远程上游分支。"
                  : "当前范围暂无提交记录。")}
          </InlineNotice>
        </div>
      {:else if visibleCommits.length === 0}
        <p class="p-3 text-xs text-muted-foreground">
          没有匹配筛选条件的提交记录。
        </p>
      {:else}
        {#each visibleCommits as row, idx (row.oid)}
          <GitLogCommitRow
            {row}
            layout={graphLayouts[idx]}
            selected={selectedCommitOid === row.oid}
            {showAuthorColumn}
            onSelect={(oid) => {
              onSelectCommit(oid);
              if (!desktop.current) mobileStep = "detail";
            }}
          />
        {/each}
      {/if}
    </div>
  </section>
{/snippet}

{#if desktop.current}
  {#key paneResetKey}
    <Resizable.PaneGroup
      direction="horizontal"
      class="h-full min-h-0 min-w-0"
      onLayoutChange={(sizes) => {
        if (showBranchesPanel && sizes.length === 3) {
          onThreePaneSizesChange?.([sizes[0]!, sizes[1]!, sizes[2]!]);
        } else if (!showBranchesPanel && sizes.length === 2) {
          onTwoPaneSizesChange?.([sizes[0]!, sizes[1]!]);
        }
      }}
    >
      {#if showBranchesPanel}
        <Resizable.Pane
          defaultSize={threePaneSizes[0]}
          minSize={14}
          class="git-pane"
        >
          <GitLogBranchTree
            {branches}
            {branchFilter}
            {checkedOutBranch}
            {onBranchFilterChange}
            {onRequestCheckout}
            {onOpenOccupiedWorktree}
          />
        </Resizable.Pane>
        <Resizable.Handle withHandle class="git-divider" />
      {/if}
      <Resizable.Pane
        defaultSize={showBranchesPanel ? threePaneSizes[1] : twoPaneSizes[0]}
        minSize={30}
        class="git-pane"
      >
        {@render commitListPane()}
      </Resizable.Pane>
      <Resizable.Handle withHandle class="git-divider" />
      <Resizable.Pane
        defaultSize={showBranchesPanel ? threePaneSizes[2] : twoPaneSizes[1]}
        minSize={24}
        class="git-pane"
      >
        <GitLogDetailPane
          detail={selectedDetail}
          selectedParentOverride={selectedCommitOid
            ? selectedParentByCommit[selectedCommitOid]
            : undefined}
          {selectedCommitFilePath}
          desktop={true}
          {diffMode}
          {diffScrollTop}
          {onDiffModeChange}
          {onDiffScrollChange}
          {onSelectParent}
          {onSelectCommitFile}
        />
      </Resizable.Pane>
    </Resizable.PaneGroup>
  {/key}
{:else if mobileStep === "list"}
  {@render commitListPane()}
{:else if mobileStep === "detail"}
  <GitLogDetailPane
    detail={selectedDetail}
    selectedParentOverride={selectedCommitOid
      ? selectedParentByCommit[selectedCommitOid]
      : undefined}
    {selectedCommitFilePath}
    desktop={false}
    {diffMode}
    {diffScrollTop}
    {onDiffModeChange}
    {onDiffScrollChange}
    onBackToList={() => (mobileStep = "list")}
    {onSelectParent}
    {onSelectCommitFile}
    onOpenMobileDiff={() => (mobileStep = "diff")}
  />
{:else}
  <div class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface">
    <div class="border-b border-git-border bg-git-panel p-2">
      <Button size="xs" variant="ghost" onclick={() => (mobileStep = "detail")}>
        <IconArrowLeft aria-hidden="true" />
        返回提交详情
      </Button>
    </div>
    <div class="min-h-0 flex-1">
      {#if activeFile}
        <GitDiffViewer
          file={activeFile}
          preferredViewMode={diffMode}
          scrollTop={diffScrollTop}
          onViewModeChange={onDiffModeChange}
          onScrollChange={onDiffScrollChange}
        />
      {/if}
    </div>
  </div>
{/if}
