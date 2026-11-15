<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import type { Snippet } from "svelte";
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
    operations,
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
    operations?: Snippet;
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
      ? m.s7_publish_ellipsis_btn()
      : m.s7_push_ellipsis_btn(),
  );
  const syncDisabledReason = $derived(
    !repoAvailable
      ? m.s7_topbar_repo_unavailable()
      : !worktreeId
        ? m.s7_topbar_sync_select_wt()
        : !worktreeUsable
          ? m.s7_topbar_sync_wt_unusable()
          : context.headState !== "branch" || !context.checkedOutBranch
            ? m.s7_topbar_sync_detached_unborn()
            : undefined,
  );

  const returnSessionNotice = $derived.by(() => {
    if (!returnSessionId) return "";
    if (returnSession) {
      return returnSession.status === "exited"
        ? m.s7_topbar_session_exited_notice()
        : "";
    }
    if (!sessionsReady) return "";
    return returnSessionArchived
      ? m.s7_topbar_session_archived_notice()
      : m.s7_topbar_session_closed_notice();
  });
</script>

<header
  class="flex min-w-0 flex-col gap-1.5 border-b bg-background px-2.5 py-2 sm:px-3"
  data-testid="git-workbench-topbar"
>
  <div
    class="flex min-w-0 flex-wrap items-center justify-between gap-1.5 sm:gap-2"
  >
    <div class="flex min-w-0 max-w-full flex-wrap items-center gap-1.5">
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
          <Badge variant="destructive">{m.s7_topbar_repo_unavailable()}</Badge>
        {:else if !worktreeId}
          <Badge variant="warning" class="max-sm:hidden"
            >{m.s7_topbar_readonly_repo_badge()}</Badge
          >
        {:else if !worktree || !worktree.available || worktree.prunable}
          <Badge variant="destructive"
            >{m.s7_topbar_wt_unavailable_badge()}</Badge
          >
        {:else if worktree.locked}
          <Badge variant="warning">{m.s7_topbar_wt_locked_badge()}</Badge>
        {:else if worktree.kind === "bare"}
          <Badge variant="warning">{m.s7_wt_bare_badge()}</Badge>
        {:else if context.headState === "detached"}
          <Badge variant="warning">{m.s7_detached_head()}</Badge>
        {:else if context.headState === "unborn"}
          <Badge variant="info" class="max-w-full">
            <span class="truncate">
              {m.s7_unborn_head_branch({
                branch: context.checkedOutBranch ?? "main",
              })}
            </span>
          </Badge>
        {/if}

        {#if repoAvailable && onBranchFilterChange && onBranchAction}
          <GitTopBarBranchPopover
            {branches}
            {context}
            {branchFilter}
            compareBaseOid={context.headOid ?? null}
            compareTargetLabel={m.s7_compare_with_head()}
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
            <span
              class="truncate"
              title={m.s7_topbar_history_filter_badge({ branch: branchFilter })}
            >
              {m.s7_topbar_history_filter_badge({ branch: branchFilter })}
            </span>
          </Badge>
        {/if}
      {/if}
    </div>

    <div
      class="ml-auto flex min-w-0 max-w-full flex-wrap items-center justify-end gap-1.5"
    >
      {@render operations?.()}
      {#if worktreeUsable && onOpenRework}
        <Button
          size="sm"
          variant="outline"
          onclick={onOpenRework}
          aria-label={reworkCount > 0
            ? m.s7_topbar_rework_count_aria({ count: reworkCount })
            : m.s7_topbar_rework_aria()}
          title={m.s7_topbar_rework_title()}
          data-testid="git-open-rework-btn"
        >
          <IconMessageForward aria-hidden="true" />
          <span class="max-sm:hidden">{m.s7_topbar_rework_btn()}</span>
          {#if reworkCount > 0}
            <Badge variant="info">{reworkCount}</Badge>
          {/if}
        </Button>
      {/if}

      {#if returnSession}
        <Button
          size="sm"
          variant="outline"
          onclick={() => onReturnSession(returnSession.id)}
          aria-label={m.s7_delivery_return_session()}
          title={m.s7_topbar_return_session_title({
            title: returnSession.title || returnSession.id,
          })}
        >
          <IconArrowBackUp aria-hidden="true" />
          <span class="max-sm:hidden">{m.s7_delivery_return_session()}</span>
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
          <span>{m.s7_op_kind_fetch()}</span>
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={!canSyncBranch}
          title={syncDisabledReason}
          onclick={onUpdate}
        >
          <IconArrowDown aria-hidden="true" />
          <span>{m.s7_op_kind_fast_forward()}</span>
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
                aria-label={m.s7_topbar_more_sync_aria()}
                title={m.s7_topbar_more_sync_aria()}
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
              {m.s7_op_kind_fetch()}
            </DropdownMenu.Item>
            <DropdownMenu.Item disabled={!canSyncBranch} onSelect={onUpdate}>
              {m.s7_op_kind_fast_forward()}
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
