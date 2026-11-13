<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconFolder from "~icons/tabler/folder";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconPlus from "~icons/tabler/plus";
  import IconSettings from "~icons/tabler/settings";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Popover from "$lib/components/ui/popover";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";

  let {
    repositories,
    repository,
    worktreeId,
    worktree,
    canAddRepository = true,
    onSelectRepository,
    onSelectWorktree,
    onAddRepository,
    onNewWorktree,
    onManageWorktrees,
  }: {
    repositories: readonly Repository[];
    repository: Repository | undefined;
    worktreeId?: string;
    worktree: Worktree | undefined;
    canAddRepository?: boolean;
    onSelectRepository: (repositoryId: string) => void;
    onSelectWorktree: (worktreeId?: string) => void;
    onAddRepository: () => void;
    onNewWorktree: () => void;
    onManageWorktrees: () => void;
  } = $props();

  let repoPopoverOpen = $state(false);
  let wtPopoverOpen = $state(false);

  const repoAvailable = $derived(repository?.availability === "available");
  const worktreeButtonLabel = $derived(
    !worktreeId
      ? "只读仓库（无工作树）"
      : worktree
        ? worktreeTitle(worktree)
        : "工作树缺失",
  );
</script>

<Popover.Root bind:open={repoPopoverOpen}>
  <Popover.Trigger>
    {#snippet child({ props })}
      <Button
        {...props}
        size="sm"
        variant="outline"
        class="max-w-full min-w-0"
        aria-label="切换仓库"
        title={repository?.anchorPath ?? "切换仓库"}
      >
        <IconFolder class="text-muted-foreground" aria-hidden="true" />
        <span class="max-w-36 truncate font-semibold sm:max-w-48">
          {repository?.name ??
            (repositories.length === 0 ? "未登记仓库" : "选择仓库")}
        </span>
        <IconChevronDown class="text-muted-foreground" aria-hidden="true" />
      </Button>
    {/snippet}
  </Popover.Trigger>
  <Popover.Content
    align="start"
    class="w-72 p-2"
    data-testid="git-repository-switcher-popover"
  >
    <div class="space-y-1">
      <p class="px-1.5 py-1 text-xs font-semibold text-muted-foreground">
        已登记仓库
      </p>
      {#if repositories.length === 0}
        <p class="px-1.5 py-2 text-xs text-muted-foreground">暂无已登记仓库</p>
      {:else}
        {#each repositories as repo (repo.id)}
          <ListButton
            variant="git-row"
            aria-selected={repository?.id === repo.id}
            onclick={() => {
              repoPopoverOpen = false;
              onSelectRepository(repo.id);
            }}
          >
            <span class="min-w-0 flex-1 truncate">{repo.name}</span>
            {#if repo.availability !== "available"}
              <Badge variant="destructive">不可用</Badge>
            {/if}
          </ListButton>
        {/each}
      {/if}
      {#if canAddRepository}
        <div class="border-t border-border pt-1">
          <ListButton
            variant="git-row"
            onclick={() => {
              repoPopoverOpen = false;
              onAddRepository();
            }}
          >
            <IconPlus aria-hidden="true" />
            <span>登记已有仓库…</span>
          </ListButton>
        </div>
      {/if}
    </div>
  </Popover.Content>
</Popover.Root>

{#if repository}
  <Popover.Root bind:open={wtPopoverOpen}>
    <Popover.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          size="sm"
          variant="outline"
          class="max-w-full min-w-0"
          aria-label="切换操作目标工作树"
          title={worktree?.path ?? "切换操作目标工作树"}
        >
          <IconGitBranch class="text-muted-foreground" aria-hidden="true" />
          <span class="max-w-36 truncate sm:max-w-48">
            {worktreeButtonLabel}
          </span>
          <IconChevronDown class="text-muted-foreground" aria-hidden="true" />
        </Button>
      {/snippet}
    </Popover.Trigger>
    <Popover.Content
      align="start"
      class="w-80 p-2"
      data-testid="git-worktree-switcher-popover"
    >
      <div class="space-y-1">
        <p class="px-1.5 py-1 text-xs font-semibold text-muted-foreground">
          操作目标工作树
        </p>
        <ListButton
          variant="git-row"
          aria-selected={!worktreeId}
          onclick={() => {
            wtPopoverOpen = false;
            onSelectWorktree(undefined);
          }}
        >
          <span class="min-w-0 flex-1 truncate">
            只读仓库视图（无工作树）
          </span>
          <Badge variant="outline">只读</Badge>
        </ListButton>
        {#each repository.worktrees as wt (wt.id)}
          <ListButton
            variant="git-row"
            aria-selected={worktreeId === wt.id}
            onclick={() => {
              wtPopoverOpen = false;
              onSelectWorktree(wt.id);
            }}
          >
            <span class="min-w-0 flex-1 truncate">{worktreeTitle(wt)}</span>
            {#if !wt.available || wt.prunable}
              <Badge variant="destructive">缺失</Badge>
            {:else if wt.kind === "bare"}
              <Badge variant="warning">裸仓库</Badge>
            {:else if wt.locked}
              <Badge variant="warning">锁定</Badge>
            {:else if wt.kind === "main"}
              <Badge variant="outline">主</Badge>
            {/if}
          </ListButton>
        {/each}
        <div class="space-y-0.5 border-t border-border pt-1">
          {#if repoAvailable}
            <ListButton
              variant="git-row"
              onclick={() => {
                wtPopoverOpen = false;
                onNewWorktree();
              }}
            >
              <IconPlus aria-hidden="true" />
              <span>新建工作树…</span>
            </ListButton>
          {/if}
          <ListButton
            variant="git-row"
            onclick={() => {
              wtPopoverOpen = false;
              onManageWorktrees();
            }}
          >
            <IconSettings aria-hidden="true" />
            <span>管理工作树…</span>
          </ListButton>
        </div>
      </div>
    </Popover.Content>
  </Popover.Root>
{/if}
