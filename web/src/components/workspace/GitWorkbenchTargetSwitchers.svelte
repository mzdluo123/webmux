<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconFolder from "~icons/tabler/folder";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconPlus from "~icons/tabler/plus";
  import IconSettings from "~icons/tabler/settings";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";

  const READONLY_WORKTREE = "__readonly__";

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

  const repoAvailable = $derived(repository?.availability === "available");
  const repositoryButtonLabel = $derived(
    repository?.name ?? (repositories.length === 0 ? "未登记仓库" : "选择仓库"),
  );
  const worktreeButtonLabel = $derived(
    !worktreeId
      ? "只读仓库（无工作树）"
      : worktree
        ? worktreeTitle(worktree)
        : "工作树缺失",
  );
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger>
    {#snippet child({ props })}
      <Button
        {...props}
        size="sm"
        variant="outline"
        aria-label="切换仓库"
        title={repository?.anchorPath ?? "切换仓库"}
      >
        <IconFolder aria-hidden="true" />
        <span class="max-w-28 truncate sm:max-w-44">
          {repositoryButtonLabel}
        </span>
        <IconChevronDown class="text-muted-foreground" aria-hidden="true" />
      </Button>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content
    align="start"
    class="w-64"
    data-testid="git-repository-switcher-popover"
  >
    <DropdownMenu.Label>已登记仓库</DropdownMenu.Label>
    {#if repositories.length === 0}
      <DropdownMenu.Item disabled>暂无已登记仓库</DropdownMenu.Item>
    {:else}
      <DropdownMenu.RadioGroup
        value={repository?.id ?? ""}
        onValueChange={(id) => {
          if (id) onSelectRepository(id);
        }}
      >
        {#each repositories as repo (repo.id)}
          <DropdownMenu.RadioItem value={repo.id} closeOnSelect>
            <span class="min-w-0 flex-1 truncate">{repo.name}</span>
            {#if repo.availability !== "available"}
              <Badge variant="destructive">不可用</Badge>
            {/if}
          </DropdownMenu.RadioItem>
        {/each}
      </DropdownMenu.RadioGroup>
    {/if}
    {#if canAddRepository}
      <DropdownMenu.Separator />
      <DropdownMenu.Item onSelect={onAddRepository}>
        <IconPlus aria-hidden="true" />
        <span>登记已有仓库…</span>
      </DropdownMenu.Item>
    {/if}
  </DropdownMenu.Content>
</DropdownMenu.Root>

{#if repository}
  <DropdownMenu.Root>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          size="sm"
          variant="outline"
          aria-label="切换操作目标工作树"
          title={worktree?.path ?? "切换操作目标工作树"}
        >
          <IconGitBranch aria-hidden="true" />
          <span class="max-w-28 truncate sm:max-w-44">
            {worktreeButtonLabel}
          </span>
          <IconChevronDown class="text-muted-foreground" aria-hidden="true" />
        </Button>
      {/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content
      align="start"
      class="w-72"
      data-testid="git-worktree-switcher-popover"
    >
      <DropdownMenu.Label>操作目标工作树</DropdownMenu.Label>
      <DropdownMenu.RadioGroup
        value={worktreeId ?? READONLY_WORKTREE}
        onValueChange={(value) =>
          onSelectWorktree(value === READONLY_WORKTREE ? undefined : value)}
      >
        <DropdownMenu.RadioItem value={READONLY_WORKTREE} closeOnSelect>
          <span class="min-w-0 flex-1 truncate">只读仓库视图（无工作树）</span>
          <Badge variant="outline">只读</Badge>
        </DropdownMenu.RadioItem>
        {#each repository.worktrees as wt (wt.id)}
          <DropdownMenu.RadioItem value={wt.id} closeOnSelect>
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
          </DropdownMenu.RadioItem>
        {/each}
      </DropdownMenu.RadioGroup>
      <DropdownMenu.Separator />
      {#if repoAvailable}
        <DropdownMenu.Item onSelect={onNewWorktree}>
          <IconPlus aria-hidden="true" />
          <span>新建工作树…</span>
        </DropdownMenu.Item>
      {/if}
      <DropdownMenu.Item onSelect={onManageWorktrees}>
        <IconSettings aria-hidden="true" />
        <span>管理工作树…</span>
      </DropdownMenu.Item>
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{/if}
