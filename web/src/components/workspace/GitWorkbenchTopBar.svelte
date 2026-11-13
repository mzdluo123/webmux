<script lang="ts">
  import IconArrowBackUp from "~icons/tabler/arrow-back-up";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconDots from "~icons/tabler/dots";
  import IconMessageForward from "~icons/tabler/message-forward";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import type {
    Repository,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
  import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
  import SidebarButton from "../shell/SidebarButton.svelte";
  import GitTopBarBranchPopover from "./GitTopBarBranchPopover.svelte";
  import GitWorkbenchTargetSwitchers from "./GitWorkbenchTargetSwitchers.svelte";

  let {
    repositories,
    repository,
    worktreeId,
    worktree,
    context,
    branches = [],
    branchFilter,
    fetching = false,
    canAddRepository = true,
    returnSessionId,
    returnSession,
    returnSessionArchived = false,
    sessionsReady = false,
    reworkCount = 0,
    onOpenRework,
    onSelectRepository,
    onSelectWorktree,
    onAddRepository,
    onNewWorktree,
    onManageWorktrees,
    onBranchFilterChange,
    onBranchAction,
    onFetch,
    onUpdate,
    onPush,
    onReturnSession,
  }: {
    repositories: readonly Repository[];
    repository: Repository | undefined;
    worktreeId?: string;
    worktree: Worktree | undefined;
    context: GitWorktreeContext;
    branches?: readonly GitBranchItem[];
    branchFilter: string | null;
    fetching?: boolean;
    canAddRepository?: boolean;
    returnSessionId?: string;
    returnSession?: SessionInfo;
    returnSessionArchived?: boolean;
    sessionsReady?: boolean;
    reworkCount?: number;
    onOpenRework?: () => void;
    onSelectRepository: (repositoryId: string) => void;
    onSelectWorktree: (worktreeId?: string) => void;
    onAddRepository: () => void;
    onNewWorktree: () => void;
    onManageWorktrees: () => void;
    onBranchFilterChange?: (branch: string | null) => void;
    onBranchAction?: (
      kind: GitSharedBranchActionKind,
      branch: GitBranchItem,
    ) => void;
    onFetch: () => void;
    onUpdate: () => void;
    onPush: () => void;
    onReturnSession: (sessionId: string) => void;
  } = $props();

  const repoAvailable = $derived(repository?.availability === "available");
  const worktreeUsable = $derived(
    Boolean(
      repoAvailable &&
      worktreeId &&
      worktree &&
      worktree.available &&
      !worktree.prunable &&
      !worktree.locked &&
      worktree.kind !== "bare",
    ),
  );
  const canSyncBranch = $derived(
    Boolean(
      worktreeUsable &&
      context.headState === "branch" &&
      context.checkedOutBranch,
    ),
  );
  const pushLabel = $derived(
    canSyncBranch && (!context.upstream || context.upstreamGone)
      ? "发布…"
      : "推送…",
  );
  const syncDisabledReason = $derived(
    !repoAvailable
      ? "仓库不可用"
      : !worktreeId
        ? "请先选择具体工作树后再执行分支更新或推送"
        : !worktreeUsable
          ? "当前工作树不可用、已锁定或为裸仓库"
          : context.headState !== "branch" || !context.checkedOutBranch
            ? "游离或未诞生 HEAD 无法直接同步分支"
            : undefined,
  );

  const returnSessionNotice = $derived.by(() => {
    if (!returnSessionId) return "";
    if (returnSession) {
      return returnSession.status === "exited"
        ? "原会话已退出，仍可返回查看输出"
        : "";
    }
    if (!sessionsReady) return "";
    return returnSessionArchived
      ? "原会话已归档，Git 工作台继续独立可用"
      : "原会话已关闭或删除，Git 工作台继续独立可用";
  });
</script>

<header
  class="flex min-w-0 flex-col gap-1.5 border-b border-git-border bg-git-panel px-2.5 py-2 sm:px-3"
  data-testid="git-workbench-topbar"
>
  <div
    class="flex min-w-0 flex-nowrap items-center justify-between gap-1.5 sm:flex-wrap sm:gap-2"
  >
    <div
      class="flex min-w-0 flex-1 flex-nowrap items-center gap-1 overflow-hidden sm:flex-wrap sm:gap-2 sm:overflow-visible"
    >
      <SidebarButton class="sm:hidden" />

      <GitWorkbenchTargetSwitchers
        {repositories}
        {repository}
        {worktreeId}
        {worktree}
        {canAddRepository}
        {onSelectRepository}
        {onSelectWorktree}
        {onAddRepository}
        {onNewWorktree}
        {onManageWorktrees}
      />

      {#if repository}
        {#if !repoAvailable}
          <Badge variant="destructive">仓库不可用</Badge>
        {:else if !worktreeId}
          <Badge variant="warning" class="max-sm:hidden">只读仓库</Badge>
        {:else if !worktree || !worktree.available || worktree.prunable}
          <Badge variant="destructive">工作树不可用</Badge>
        {:else if worktree.locked}
          <Badge variant="warning">工作树已锁定</Badge>
        {:else if worktree.kind === "bare"}
          <Badge variant="warning">裸仓库</Badge>
        {:else if context.headState === "detached"}
          <Badge variant="warning">游离 HEAD</Badge>
        {:else if context.headState === "unborn"}
          <Badge variant="info" class="max-w-full">
            <span class="truncate">
              未诞生 HEAD ({context.checkedOutBranch ?? "main"})
            </span>
          </Badge>
        {:else}
          <Badge
            variant="info"
            class={branchFilter
              ? "max-w-44 max-sm:hidden"
              : "max-w-48 max-lg:hidden"}
            data-testid="git-target-branch-badge"
          >
            <span
              class="truncate"
              title={`当前检出：${context.checkedOutBranch ?? "HEAD"}`}
            >
              检出：{context.checkedOutBranch ?? "HEAD"}
            </span>
          </Badge>
        {/if}

        {#if repoAvailable && onBranchFilterChange && onBranchAction}
          <GitTopBarBranchPopover
            {branches}
            {context}
            {branchFilter}
            compareBaseOid={context.headOid ?? null}
            compareTargetLabel="与当前 HEAD 比较"
            onSelectAll={() => onBranchFilterChange(null)}
            {onBranchAction}
          />
        {/if}

        {#if branchFilter}
          <Badge
            variant="outline"
            class="max-w-28 sm:max-w-44"
            data-testid="git-filter-branch-badge"
          >
            <span class="truncate" title={`历史筛选：${branchFilter}`}>
              历史筛选：{branchFilter}
            </span>
          </Badge>
        {/if}
      {/if}
    </div>

    <div
      class="flex min-h-(--control-touch) shrink-0 items-center gap-1.5 sm:min-h-0"
    >
      {#if worktreeUsable && onOpenRework}
        <Button
          size="sm"
          variant="outline"
          onclick={onOpenRework}
          aria-label={reworkCount > 0
            ? `返工意见草稿箱 (${reworkCount})`
            : "返工意见草稿箱"}
          title="打开返工意见草稿箱并交回指定会话"
          data-testid="git-open-rework-btn"
        >
          <IconMessageForward aria-hidden="true" />
          <span class="max-sm:hidden">返工意见</span>
          {#if reworkCount > 0}
            <Badge variant="info">{reworkCount}</Badge>
          {/if}
        </Button>
      {/if}

      {#if returnSession}
        <Button
          size="sm"
          variant="git-primary"
          onclick={() => onReturnSession(returnSession.id)}
          aria-label="返回会话"
          title={`返回会话：${returnSession.title || returnSession.id}`}
        >
          <IconArrowBackUp aria-hidden="true" />
          <span class="max-sm:hidden">返回会话</span>
        </Button>
      {/if}

      <div class="flex items-center gap-1.5 max-md:hidden">
        <Button
          size="sm"
          variant="outline"
          disabled={!repoAvailable || fetching}
          onclick={onFetch}
        >
          <IconRefresh
            class={fetching ? "motion-safe:animate-spin" : ""}
            aria-hidden="true"
          />
          <span>获取远程</span>
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={!canSyncBranch}
          title={syncDisabledReason}
          onclick={onUpdate}
        >
          <IconArrowDown aria-hidden="true" />
          <span>快进更新</span>
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={!canSyncBranch}
          title={syncDisabledReason}
          onclick={onPush}
        >
          <IconArrowUp aria-hidden="true" />
          <span>{pushLabel}</span>
        </Button>
      </div>

      <div class="md:hidden">
        <DropdownMenu.Root>
          <DropdownMenu.Trigger>
            {#snippet child({ props })}
              <Button
                {...props}
                size="icon-sm"
                variant="outline"
                aria-label="更多同步操作"
                title="更多同步操作"
              >
                <IconDots aria-hidden="true" />
              </Button>
            {/snippet}
          </DropdownMenu.Trigger>
          <DropdownMenu.Content align="end">
            <DropdownMenu.Item
              disabled={!repoAvailable || fetching}
              onSelect={onFetch}
            >
              获取远程
            </DropdownMenu.Item>
            <DropdownMenu.Item disabled={!canSyncBranch} onSelect={onUpdate}>
              快进更新
            </DropdownMenu.Item>
            <DropdownMenu.Item disabled={!canSyncBranch} onSelect={onPush}>
              {pushLabel}
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Root>
      </div>
    </div>
  </div>

  {#if returnSessionNotice}
    <p
      class="text-xs text-warning"
      role="status"
      data-testid="git-return-session-notice"
    >
      {returnSessionNotice}
    </p>
  {/if}
</header>
