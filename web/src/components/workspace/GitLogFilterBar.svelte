<script lang="ts">
  import IconFilter from "~icons/tabler/filter";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconLayoutSidebar from "~icons/tabler/layout-sidebar";
  import IconRefresh from "~icons/tabler/refresh";
  import IconRestore from "~icons/tabler/restore";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import type {
    GitLogFilterCallbacks,
    GitLogFilterFields,
  } from "./git-log-pane-types";

  let {
    desktop = true,
    scope,
    branchFilter,
    searchQuery,
    authorFilter = "",
    sinceFilter = "",
    untilFilter = "",
    pathFilter,
    refsChanged = false,
    selectionNotice = null,
    compareBaseMarkOid = null,
    showBranchesPanel = true,
    onOpenRefDrawer,
    onToggleBranchesPanel,
    onResetLayout,
    onRefreshHistory,
    onScopeChange,
    onSearchChange,
    onAuthorChange,
    onSinceChange,
    onUntilChange,
    onPathFilterChange,
    onClearFilters,
    onJumpToHead,
    onDismissSelectionNotice,
    onCancelCompareMark,
  }: GitLogFilterFields &
    GitLogFilterCallbacks & {
      desktop?: boolean;
      showBranchesPanel?: boolean;
      onOpenRefDrawer?: () => void;
      onToggleBranchesPanel?: () => void;
      onResetLayout?: () => void;
      onCancelCompareMark?: () => void;
    } = $props();

  let manualAdvancedOpen = $state(false);
  const hasAdvancedFilter = $derived(
    Boolean(authorFilter.trim() || sinceFilter.trim() || untilFilter.trim()),
  );
  const showAdvanced = $derived(manualAdvancedOpen || hasAdvancedFilter);
  const hasAnyActiveFilter = $derived(
    scope !== "all" ||
      branchFilter !== null ||
      Boolean(searchQuery.trim()) ||
      Boolean(pathFilter.trim()) ||
      hasAdvancedFilter,
  );
</script>

<div class="space-y-2 border-b bg-background px-2.5 py-2">
  <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      {#if !desktop && onOpenRefDrawer}
        <Button
          size="sm"
          variant="outline"
          aria-label="打开引用筛选抽屉"
          onclick={onOpenRefDrawer}
        >
          <IconGitBranch aria-hidden="true" />
          <span class="max-w-28 truncate">
            {branchFilter ?? "引用"}
          </span>
        </Button>
      {/if}

      <SegmentedControl.Root
        wrap
        value={scope}
        onValueChange={(val) => {
          if (
            val === "all" ||
            val === "branch" ||
            val === "task" ||
            val === "to-push" ||
            val === "incoming"
          ) {
            onScopeChange(val);
          }
        }}
        aria-label="提交范围"
      >
        <SegmentedControl.Item value="all">所有历史</SegmentedControl.Item>
        <SegmentedControl.Item value="branch">当前分支</SegmentedControl.Item>
        <SegmentedControl.Item value="task">当前任务</SegmentedControl.Item>
        <SegmentedControl.Item value="to-push">待推送</SegmentedControl.Item>
        <SegmentedControl.Item value="incoming">远端新增</SegmentedControl.Item>
      </SegmentedControl.Root>
    </div>

    <div class="flex items-center gap-1">
      <Button
        size="icon-sm"
        variant="ghost"
        aria-pressed={showAdvanced}
        aria-label="作者与日期筛选"
        title="作者与日期筛选"
        onclick={() => (manualAdvancedOpen = !showAdvanced)}
      >
        <IconFilter aria-hidden="true" />
      </Button>

      {#if onRefreshHistory}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label="刷新提交历史"
          title="刷新提交历史"
          onclick={onRefreshHistory}
        >
          <IconRefresh aria-hidden="true" />
        </Button>
      {/if}

      {#if desktop && onToggleBranchesPanel}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-pressed={showBranchesPanel}
          aria-label="切换引用筛选栏"
          title="切换引用筛选栏"
          onclick={onToggleBranchesPanel}
        >
          <IconLayoutSidebar aria-hidden="true" />
        </Button>
      {/if}

      {#if desktop && onResetLayout}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label="重置分栏比例"
          title="重置分栏比例"
          onclick={onResetLayout}
        >
          <IconRestore aria-hidden="true" />
        </Button>
      {/if}
    </div>
  </div>

  <div class="grid min-w-0 grid-cols-1 gap-1.5 sm:grid-cols-2">
    <Input
      size="sm"
      value={searchQuery}
      placeholder="搜索提交信息或关键字…"
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

  {#if showAdvanced}
    <div class="grid min-w-0 grid-cols-1 gap-1.5 sm:grid-cols-3">
      <Input
        size="sm"
        value={authorFilter}
        placeholder="按作者姓名或邮箱筛选…"
        aria-label="按作者筛选提交"
        oninput={(e) => onAuthorChange?.(e.currentTarget.value)}
      />
      <Input
        size="sm"
        type="date"
        value={sinceFilter}
        aria-label="起始日期"
        title="起始日期 (Since)"
        oninput={(e) => onSinceChange?.(e.currentTarget.value)}
      />
      <Input
        size="sm"
        type="date"
        value={untilFilter}
        aria-label="截止日期"
        title="截止日期 (Until)"
        oninput={(e) => onUntilChange?.(e.currentTarget.value)}
      />
    </div>
  {/if}

  {#if hasAnyActiveFilter && onClearFilters}
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <span class="min-w-0 truncate text-xs text-muted-foreground">
        已启用筛选{#if branchFilter}（引用：{branchFilter}）{/if}
      </span>
      <Button size="sm" variant="ghost" onclick={onClearFilters}>
        <IconX aria-hidden="true" />
        重置全部筛选
      </Button>
    </div>
  {/if}

  {#if refsChanged && onRefreshHistory}
    <InlineNotice tone="info">
      <span class="mr-2">分支引用已更新，可刷新获取最新提交图。</span>
      <Button size="sm" variant="outline" onclick={onRefreshHistory}>
        刷新历史
      </Button>
    </InlineNotice>
  {/if}

  {#if selectionNotice}
    <InlineNotice tone="warning">
      <span class="mr-2">{selectionNotice.message}</span>
      {#if selectionNotice.canJumpHead && onJumpToHead}
        <Button size="sm" variant="outline" onclick={onJumpToHead}>
          定位当前 HEAD
        </Button>
      {/if}
      {#if onDismissSelectionNotice}
        <Button size="sm" variant="ghost" onclick={onDismissSelectionNotice}>
          关闭
        </Button>
      {/if}
    </InlineNotice>
  {/if}

  {#if compareBaseMarkOid}
    <InlineNotice tone="info">
      <span class="mr-2">
        已设比较起点 <code class="font-mono"
          >{compareBaseMarkOid.slice(0, 7)}</code
        >，请在目标提交菜单点击「与起点比较」。
      </span>
      {#if onCancelCompareMark}
        <Button size="sm" variant="ghost" onclick={onCancelCompareMark}>
          取消
        </Button>
      {/if}
    </InlineNotice>
  {/if}
</div>
