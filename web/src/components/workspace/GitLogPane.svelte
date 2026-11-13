<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconDots from "~icons/tabler/dots";
  import IconLayoutSidebar from "~icons/tabler/layout-sidebar";
  import IconRestore from "~icons/tabler/restore";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Resizable from "$lib/components/ui/resizable";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import {
    computeGitGraphLayout,
    type GitLaneTone,
  } from "../../domains/workspace/git-graph-layout";
  import {
    filterLogCommits,
    resolveCommitParentFiles,
  } from "../../domains/workspace/git-workbench-state";
  import type {
    GitBranchItem,
    GitCommitDetail,
    GitHistoryRow,
    GitHistoryScope,
  } from "../../contracts/git-workbench-schemas";
  import GitDiffViewer from "./GitDiffViewer.svelte";
  import GitLogBranchTree from "./GitLogBranchTree.svelte";
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
    onScopeChange: (scope: GitHistoryScope) => void;
    onBranchFilterChange: (branch: string | null) => void;
    onSearchChange: (q: string) => void;
    onPathFilterChange: (p: string) => void;
    onSelectCommit: (oid: string) => void;
    onSelectParent: (commitOid: string, parentOid: string) => void;
    onSelectCommitFile: (path: string) => void;
    onRequestCheckout: (branch: string) => void;
    onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  let showBranchesPanel = $state(true);
  let mobileStep = $state<"list" | "detail" | "diff">("list");
  let paneResetKey = $state(0);

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

  function laneStrokeClass(tone: GitLaneTone): string {
    switch (tone) {
      case 0:
        return "stroke-git-primary";
      case 1:
        return "stroke-success";
      case 2:
        return "stroke-warning";
      case 3:
        return "stroke-info";
      case 4:
        return "stroke-destructive";
      default:
        return "stroke-muted-foreground";
    }
  }

  function laneFillClass(tone: GitLaneTone): string {
    switch (tone) {
      case 0:
        return "fill-git-primary";
      case 1:
        return "fill-success";
      case 2:
        return "fill-warning";
      case 3:
        return "fill-info";
      case 4:
        return "fill-destructive";
      default:
        return "fill-muted-foreground";
    }
  }

  function copyOid(oid: string) {
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      void navigator.clipboard.writeText(oid);
    }
  }
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
              onclick={() => (showBranchesPanel = !showBranchesPanel)}
            >
              <IconLayoutSidebar aria-hidden="true" />
            </Button>
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label="重置分栏比例"
              title="重置分栏比例"
              onclick={() => paneResetKey++}
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
      class="min-h-0 flex-1 overflow-y-auto p-1.5"
      role="listbox"
      aria-label="提交记录"
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
          {@const layout = graphLayouts[idx]}
          <div class="group/row relative">
            <ListButton
              variant="git-row"
              role="option"
              data-has-action="true"
              aria-selected={selectedCommitOid === row.oid}
              onclick={() => {
                onSelectCommit(row.oid);
                if (!desktop.current) mobileStep = "detail";
              }}
            >
              {#if layout}
                <svg
                  width={layout.width}
                  height={layout.height}
                  viewBox={`0 0 ${layout.width} ${layout.height}`}
                  class="shrink-0 overflow-visible"
                  aria-hidden="true"
                  data-graph-oid={row.oid}
                >
                  {#each layout.rails as rail (rail.lane)}
                    <line
                      x1={rail.x}
                      y1={rail.y1}
                      x2={rail.x}
                      y2={rail.y2}
                      stroke-width="1.75"
                      class={laneStrokeClass(rail.tone)}
                    />
                  {/each}
                  {#each layout.segments as seg (seg.key)}
                    <path
                      d={seg.d}
                      fill="none"
                      stroke-width="1.75"
                      stroke-dasharray={seg.dashed
                        ? "3 2"
                        : seg.isBoundary
                          ? "2 2"
                          : undefined}
                      data-edge-kind={seg.kind}
                      class={laneStrokeClass(seg.tone)}
                    />
                  {/each}
                  <circle
                    cx={layout.nodeX}
                    cy={layout.nodeY}
                    r={layout.isMerge ? 4.2 : 3.4}
                    class={laneFillClass(layout.nodeTone)}
                  />
                </svg>
              {/if}

              <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
                {#each row.refs as refBadge (`${refBadge.kind}:${refBadge.name}`)}
                  <Badge
                    variant={refBadge.kind === "head"
                      ? "info"
                      : refBadge.kind === "remote"
                        ? "secondary"
                        : "outline"}
                    class="max-w-full"
                  >
                    <span class="truncate" title={refBadge.name}>
                      {refBadge.name}
                    </span>
                  </Badge>
                {/each}
                {#if row.directParentsMissing}
                  <Badge variant="warning" class="max-w-full">
                    <span class="truncate">筛选省略中间节点</span>
                  </Badge>
                {/if}
                <span
                  class="min-w-[6ch] max-w-full flex-1 truncate font-medium"
                  title={row.subject}
                >
                  {row.subject}
                </span>
                <span class="shrink-0 font-mono text-xs text-muted-foreground">
                  {row.shortOid}
                </span>
                <span
                  class="hidden shrink-0 text-xs text-muted-foreground sm:inline"
                >
                  {row.authorName}
                </span>
              </div>
            </ListButton>

            <DropdownMenu.Root>
              <DropdownMenu.Trigger>
                {#snippet child({ props })}
                  <ListButton
                    {...props}
                    variant="git-row-action"
                    aria-label={`提交操作 ${row.shortOid}`}
                    title={`提交操作 ${row.shortOid}`}
                  >
                    <IconDots aria-hidden="true" />
                  </ListButton>
                {/snippet}
              </DropdownMenu.Trigger>
              <DropdownMenu.Content align="end">
                <DropdownMenu.Item onclick={() => copyOid(row.oid)}>
                  复制提交 OID ({row.shortOid})
                </DropdownMenu.Item>
                <DropdownMenu.Item
                  onclick={() => {
                    onSelectCommit(row.oid);
                    if (!desktop.current) mobileStep = "detail";
                  }}
                >
                  查看提交详情
                </DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Root>
          </div>
        {/each}
      {/if}
    </div>
  </section>
{/snippet}

{#if desktop.current}
  {#key paneResetKey}
    <Resizable.PaneGroup direction="horizontal" class="h-full min-h-0 min-w-0">
      {#if showBranchesPanel}
        <Resizable.Pane defaultSize={20} minSize={14} class="git-pane">
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
        defaultSize={showBranchesPanel ? 46 : 60}
        minSize={30}
        class="git-pane"
      >
        {@render commitListPane()}
      </Resizable.Pane>
      <Resizable.Handle withHandle class="git-divider" />
      <Resizable.Pane defaultSize={34} minSize={24} class="git-pane">
        <GitLogDetailPane
          detail={selectedDetail}
          selectedParentOverride={selectedCommitOid
            ? selectedParentByCommit[selectedCommitOid]
            : undefined}
          {selectedCommitFilePath}
          desktop={true}
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
          path={activeFile.newPath}
          oldPath={activeFile.oldPath}
          status={activeFile.status}
          binary={activeFile.binary}
          hunks={activeFile.hunks}
        />
      {/if}
    </div>
  </div>
{/if}
