<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconFolder from "~icons/tabler/folder";
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Popover from "$lib/components/ui/popover";
  import * as Tabs from "$lib/components/ui/tabs";
  import {
    applyStageHunk,
    applyUnstageHunk,
    buildCommitPreviewModel,
    selectLogBranchFilter,
    toggleReviewedPath,
  } from "../../domains/workspace/git-workbench-state";
  import type {
    GitCommitOutcome,
    GitPushPreviewModel,
    GitWorkbenchMode,
    GitWorkbenchViewState,
  } from "../../contracts/git-workbench-schemas";
  import GitBranchesPane from "./GitBranchesPane.svelte";
  import GitCommitPane from "./GitCommitPane.svelte";
  import GitCommitPreviewDialog from "./GitCommitPreviewDialog.svelte";
  import GitLogPane from "./GitLogPane.svelte";
  import GitPushDialog from "./GitPushDialog.svelte";
  import GitWorkbenchModeTabs from "./GitWorkbenchModeTabs.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    view = $bindable(),
    simulatedCommitOutcome = null,
    defaultPushPreview = null,
    onOpenWorktree,
  }: {
    view: GitWorkbenchViewState;
    simulatedCommitOutcome?: GitCommitOutcome | null;
    defaultPushPreview?: GitPushPreviewModel | null;
    onOpenWorktree?: (worktreeId: string, path: string) => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1280px)");
  let branchPopoverOpen = $state(false);

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
    branchPopoverOpen = false;
  }

  function handleRequestCheckout(branch: string) {
    view = {
      ...view,
      statusBanner: `已请求检出分支 ${branch}（Stage S 预览确认占位，不会静默切换工作树）`,
    };
    branchPopoverOpen = false;
  }

  function handleOpenOccupiedWorktree(worktreeId: string, path: string) {
    onOpenWorktree?.(worktreeId, path);
    view = {
      ...view,
      statusBanner: `已定位到占用该分支的工作树：${path} (${worktreeId})`,
    };
    branchPopoverOpen = false;
  }

  function openCommitPreview(followUpPush: boolean) {
    view = {
      ...view,
      commitPreview: buildCommitPreviewModel(view, followUpPush),
    };
  }

  function openPushDialog(branchOverride?: string) {
    const sourceBranch =
      branchOverride ?? view.context.checkedOutBranch ?? "main";
    const pushModel: GitPushPreviewModel = defaultPushPreview ?? {
      repositoryName: view.context.repositoryName,
      sourceBranch,
      remote: "origin",
      targetBranch: sourceBranch,
      pushUrl: `https://example.invalid/${view.context.repositoryName}.git`,
      publish: !view.context.upstream,
      commits: Object.values(view.commitDetails),
      blockers: readonlyMode
        ? [
            {
              code: "no_worktree",
              message: "无工作树只读模式下无法执行分支推送。",
            },
          ]
        : [],
      warnings: [],
    };
    view = { ...view, pushPreview: pushModel };
  }

  function confirmCommit() {
    const preview = view.commitPreview;
    if (!preview) return;
    const outcome: GitCommitOutcome = simulatedCommitOutcome ?? {
      status: "matched",
      commitOid: "f00dbabe1234567890abcdef1234567890abcdef",
      expectedTreeOid: preview.expectedTreeOid,
      actualTreeOid: preview.expectedTreeOid,
      summary: `已从真实 Git index 创建提交（${preview.stagedFiles.length} 个文件）`,
      reasons: [],
    };

    const remainingChanges =
      outcome.status === "matched"
        ? view.workingChanges.filter((c) => c.group !== "staged")
        : view.workingChanges;

    const shouldFollowUpPush =
      preview.followUpPush && outcome.status === "matched";

    view = {
      ...view,
      workingChanges: remainingChanges,
      commitMessage: outcome.status === "matched" ? "" : view.commitMessage,
      commitPreview: null,
      lastCommitOutcome: outcome,
      statusBanner: outcome.summary,
    };

    if (shouldFollowUpPush) {
      openPushDialog(preview.branch);
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
</script>

<div
  class="git-workbench flex h-full min-h-0 min-w-0 flex-col border border-git-border bg-git-surface text-foreground"
  data-testid="git-workbench"
>
  <header
    class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b border-git-border bg-git-panel px-3 py-2"
  >
    <div class="flex min-w-0 flex-wrap items-center gap-2">
      <div class="flex min-w-0 items-center gap-1.5">
        <IconFolder
          class="size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        <span class="truncate text-sm font-semibold">
          {view.context.repositoryName}
        </span>
      </div>

      <Badge variant={readonlyMode ? "warning" : "info"} class="max-w-full">
        <span
          class="truncate"
          title={readonlyMode
            ? "只读仓库（无工作树）"
            : view.context.headState === "detached"
              ? "游离 HEAD"
              : view.context.headState === "unborn"
                ? `未诞生 HEAD (${view.context.checkedOutBranch ?? "main"})`
                : `当前检出：${view.context.checkedOutBranch ?? "HEAD"}`}
        >
          {#if readonlyMode}
            只读仓库（无工作树）
          {:else if view.context.headState === "detached"}
            游离 HEAD
          {:else if view.context.headState === "unborn"}
            未诞生 HEAD ({view.context.checkedOutBranch ?? "main"})
          {:else}
            当前检出：{view.context.checkedOutBranch ?? "HEAD"}
          {/if}
        </span>
      </Badge>

      {#if view.branchFilter}
        <Badge variant="outline" class="max-w-full">
          <span class="truncate" title={`浏览筛选：${view.branchFilter}`}>
            浏览筛选：{view.branchFilter}
          </span>
        </Badge>
      {/if}

      <Popover.Root bind:open={branchPopoverOpen}>
        <Popover.Trigger>
          {#snippet child({ props })}
            <Button {...props} size="xs" variant="outline">
              <IconGitBranch aria-hidden="true" />
              分支快选
            </Button>
          {/snippet}
        </Popover.Trigger>
        <Popover.Content align="start" class="w-72 p-2">
          <div class="space-y-1">
            <p class="px-1.5 py-1 text-xs font-semibold text-muted-foreground">
              切换历史筛选或查看分支状态
            </p>
            <ListButton
              variant="git-row"
              aria-selected={view.branchFilter === null}
              onclick={() => handleBranchFilter(null)}
            >
              全部可达引用 (All)
            </ListButton>
            {#each view.branches as b (`${b.kind}:${b.name}`)}
              <ListButton
                variant="git-row"
                aria-selected={view.branchFilter === b.name}
                onclick={() => handleBranchFilter(b.name)}
              >
                <span class="min-w-0 flex-1 truncate">{b.name}</span>
                {#if b.occupiedBy}
                  <Badge variant="warning">已占用</Badge>
                {:else if b.name === view.context.checkedOutBranch}
                  <Badge variant="info">HEAD</Badge>
                {/if}
              </ListButton>
            {/each}
          </div>
        </Popover.Content>
      </Popover.Root>
    </div>

    <div class="flex items-center gap-2">
      <Button
        size="sm"
        variant="outline"
        disabled={readonlyMode}
        onclick={() => openPushDialog()}
      >
        <IconArrowUp aria-hidden="true" />
        推送…
      </Button>
    </div>
  </header>

  {#if view.lastCommitOutcome && view.lastCommitOutcome.status !== "matched"}
    <div class="border-b border-git-border bg-git-surface px-3 py-2">
      <InlineNotice
        tone={view.lastCommitOutcome.status === "busy_locked"
          ? "danger"
          : "warning"}
      >
        <span class="font-semibold">
          [{view.lastCommitOutcome.status}]
        </span>
        {view.lastCommitOutcome.summary}
        {#if view.lastCommitOutcome.reasons.length > 0}
          （{view.lastCommitOutcome.reasons.join("；")}）
        {/if}
      </InlineNotice>
    </div>
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
        historyState={view.historyState}
        historyReason={view.historyReason}
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
        onSearchChange={(searchQuery) => (view = { ...view, searchQuery })}
        onAuthorChange={(authorFilter) => (view = { ...view, authorFilter })}
        onSinceChange={(sinceFilter) => (view = { ...view, sinceFilter })}
        onUntilChange={(untilFilter) => (view = { ...view, untilFilter })}
        onPathFilterChange={(pathFilter) => (view = { ...view, pathFilter })}
        onClearFilters={() =>
          (view = {
            ...view,
            scope: "all",
            branchFilter: null,
            searchQuery: "",
            authorFilter: "",
            sinceFilter: "",
            untilFilter: "",
            pathFilter: "",
          })}
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
        onStartCompare={(baseOid, targetOid) => {
          const targetDetail = view.commitDetails[targetOid];
          const pKey = targetDetail?.selectedParent ?? "";
          const files = targetDetail?.filesByParent[pKey] ?? [];
          view = {
            ...view,
            compareBaseMarkOid: null,
            selectedCommitFilePath: files[0]?.newPath ?? null,
            compareMode: {
              baseOid,
              targetOid,
              files,
              nextCursor: "",
              loading: false,
              loadingMore: false,
              error: "",
            },
          };
        }}
        onSwapCompareDirection={() => {
          const cmp = view.compareMode;
          if (!cmp) return;
          view = {
            ...view,
            compareMode: {
              ...cmp,
              baseOid: cmp.targetOid,
              targetOid: cmp.baseOid,
            },
          };
        }}
        onExitCompare={() => (view = { ...view, compareMode: null })}
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
        onSelectChange={(activeChangeId) =>
          (view = { ...view, activeChangeId })}
        onToggleReviewed={(path) =>
          (view = {
            ...view,
            reviewedPaths: toggleReviewedPath(view.reviewedPaths, path),
          })}
        onStageChange={(changeId) =>
          (view = {
            ...view,
            workingChanges: applyStageHunk(view.workingChanges, changeId),
          })}
        onUnstageChange={(changeId) =>
          (view = {
            ...view,
            workingChanges: applyUnstageHunk(view.workingChanges, changeId),
          })}
        onStageHunk={(changeId, hunkId) =>
          (view = {
            ...view,
            workingChanges: applyStageHunk(
              view.workingChanges,
              changeId,
              hunkId,
            ),
          })}
        onUnstageHunk={(changeId, hunkId) =>
          (view = {
            ...view,
            workingChanges: applyUnstageHunk(
              view.workingChanges,
              changeId,
              hunkId,
            ),
          })}
        onCommitMessageChange={(commitMessage) =>
          (view = { ...view, commitMessage })}
        onOpenCommitPreview={openCommitPreview}
      />
    </Tabs.Content>

    <Tabs.Content
      value="branches"
      class="min-h-0 min-w-0 flex-1 overflow-hidden p-0"
    >
      <GitBranchesPane
        context={view.context}
        branches={view.branches}
        onFilterInLog={(branch) => {
          view = {
            ...selectLogBranchFilter(view, branch),
            mode: "log",
          };
        }}
        onRequestCheckout={handleRequestCheckout}
        onOpenOccupiedWorktree={handleOpenOccupiedWorktree}
        onOpenPush={(branch) => openPushDialog(branch)}
      />
    </Tabs.Content>
  </Tabs.Root>

  <footer
    class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-t border-git-border bg-git-panel px-3 py-1.5 text-xs text-muted-foreground"
  >
    <span
      class="min-w-0 truncate font-mono"
      title={view.context.worktreePath ?? ""}
    >
      工作树：{view.context.worktreePath ?? "无工作树（只读仓库）"}
    </span>
    {#if view.statusBanner}
      <span class="min-w-0 wrap-anywhere text-foreground" role="status">
        {view.statusBanner}
      </span>
    {/if}
  </footer>

  {#if view.commitPreview}
    <GitCommitPreviewDialog
      model={view.commitPreview}
      commitMessage={view.commitMessage}
      onConfirm={confirmCommit}
      onClose={() => (view = { ...view, commitPreview: null })}
    />
  {/if}

  {#if view.pushPreview}
    <GitPushDialog
      model={view.pushPreview}
      onConfirm={confirmPush}
      onClose={() => (view = { ...view, pushPreview: null })}
    />
  {/if}
</div>
