<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconFolder from "~icons/tabler/folder";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Tabs from "$lib/components/ui/tabs";
  import type {
    GitBranchItem,
    GitCommitOutcome,
    GitPrecedingCommit,
    GitPushPreviewModel,
    GitWorkbenchMode,
    GitWorkbenchViewState,
  } from "../../contracts/git-workbench-schemas";
  import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
  import {
    applyShellBranchAction,
    applyShellCommitConfirm,
    buildShellPushPreview,
  } from "../../domains/workspace/git-shell-actions";
  import {
    applyStageHunk,
    applyUnstageHunk,
    buildCommitPreviewModel,
    selectLogBranchFilter,
    toggleReviewedChange,
  } from "../../domains/workspace/git-workbench-state";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitBranchesPane from "./GitBranchesPane.svelte";
  import GitCommitPane from "./GitCommitPane.svelte";
  import GitCommitPreviewDialog from "./GitCommitPreviewDialog.svelte";
  import GitLogPane from "./GitLogPane.svelte";
  import GitPushDialog from "./GitPushDialog.svelte";
  import GitTopBarBranchPopover from "./GitTopBarBranchPopover.svelte";
  import GitWorkbenchModeTabs from "./GitWorkbenchModeTabs.svelte";

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

      <GitTopBarBranchPopover
        branches={view.branches}
        context={view.context}
        branchFilter={view.branchFilter}
        compareBaseOid={view.context.headOid ?? view.commits[0]?.oid ?? null}
        compareTargetLabel="与当前 HEAD 比较"
        onSelectAll={() => handleBranchFilter(null)}
        onBranchAction={handleBranchAction}
      />
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
        context={view.context}
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
        onBranchAction={handleBranchAction}
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
          const d = view.commitDetails[targetOid];
          const files = d?.filesByParent[d.selectedParent ?? ""] ?? [];
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
        onToggleReviewed={(c) =>
          (view = {
            ...view,
            reviewedPaths: toggleReviewedChange(view.reviewedPaths, c),
          })}
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
        onStageGroup={(op, group) => {
          let next = view.workingChanges;
          for (const it of view.workingChanges.filter(
            (c) => c.group === group,
          )) {
            next =
              op === "stage"
                ? applyStageHunk(next, it.id)
                : applyUnstageHunk(next, it.id);
          }
          view = { ...view, workingChanges: next };
        }}
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
        onOpenCommitPreview={openCommitPreview}
        onOpenPush={() => openPushDialog()}
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
      onClose={() => {
        const hadPreceding = view.pushPreview?.precedingCommit;
        view = {
          ...view,
          pushPreview: null,
          statusBanner: hadPreceding
            ? `本地已提交（${hadPreceding.shortOid}），已取消推送；本地提交已保留。`
            : view.statusBanner,
        };
      }}
    />
  {/if}
</div>
