<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconCloud from "~icons/tabler/cloud";
  import IconDots from "~icons/tabler/dots";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconStarFilled from "~icons/tabler/star-filled";
  import IconTag from "~icons/tabler/tag";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { GitBranchItem } from "../../contracts/git-workbench-schemas";

  let {
    branches,
    branchFilter,
    checkedOutBranch,
    selectedCommitOid = null,
    onBranchFilterChange,
    onToggleFavorite,
    onCompareWithSelected,
    onRequestCheckout,
    onOpenOccupiedWorktree,
  }: {
    branches: readonly GitBranchItem[];
    branchFilter: string | null;
    checkedOutBranch: string | null;
    selectedCommitOid?: string | null;
    onBranchFilterChange: (branch: string | null) => void;
    onToggleFavorite?: (branch: string) => void;
    onCompareWithSelected?: (refHeadOid: string) => void;
    onRequestCheckout: (branch: string) => void;
    onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
  } = $props();

  let refQuery = $state("");
  let collapsed = $state<Record<string, boolean>>({});

  const normalizedQuery = $derived(refQuery.trim().toLowerCase());
  const filteredItems = $derived(
    normalizedQuery
      ? branches.filter(
          (item) =>
            item.name.toLowerCase().includes(normalizedQuery) ||
            item.shortHead.toLowerCase().includes(normalizedQuery),
        )
      : branches,
  );

  const favoriteBranches = $derived(
    filteredItems.filter((item) => item.favorite && item.kind !== "tag"),
  );
  const currentBranches = $derived(
    filteredItems.filter(
      (item) => item.kind === "local" && item.name === checkedOutBranch,
    ),
  );
  const localBranches = $derived(
    filteredItems.filter((item) => item.kind === "local"),
  );
  const remoteBranches = $derived(
    filteredItems.filter((item) => item.kind === "remote"),
  );
  const tagItems = $derived(
    filteredItems.filter((item) => item.kind === "tag"),
  );

  const sections = $derived(
    [
      { id: "favorites", label: "收藏分支", items: favoriteBranches },
      { id: "current", label: "当前分支", items: currentBranches },
      { id: "local", label: "本地分支", items: localBranches },
      { id: "remote", label: "远端跟踪分支", items: remoteBranches },
      { id: "tags", label: "标签 (Tags)", items: tagItems },
    ].filter((sec) => sec.items.length > 0),
  );

  function toggleSection(id: string) {
    collapsed = { ...collapsed, [id]: !collapsed[id] };
  }
</script>

{#snippet refRow(sectionId: string, branch: GitBranchItem)}
  <div class="group/row relative" data-ref-section={sectionId}>
    <ListButton
      variant="git-row"
      data-has-action="true"
      aria-selected={branchFilter === branch.name}
      onclick={() => onBranchFilterChange(branch.name)}
    >
      {#if branch.kind === "tag"}
        <IconTag
          class="size-3.5 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
      {:else if branch.kind === "remote"}
        <IconCloud
          class="size-3.5 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
      {:else}
        <IconGitBranch
          class="size-3.5 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
      {/if}

      {#if branch.favorite && sectionId !== "favorites"}
        <IconStarFilled
          class="size-3 shrink-0 text-warning"
          aria-hidden="true"
        />
      {/if}

      <span class="min-w-0 flex-1 truncate" title={branch.name}>
        {branch.name}
      </span>

      {#if branch.kind === "local" && branch.name === checkedOutBranch}
        <Badge variant="info">HEAD</Badge>
      {/if}

      {#if branch.occupiedBy}
        <Badge
          variant="warning"
          title={`由另一工作树检出：${branch.occupiedBy.worktreePath}`}
        >
          已占用{branch.occupiedBy.activeSessions > 0
            ? `·${branch.occupiedBy.activeSessions}`
            : ""}
        </Badge>
      {/if}

      {#if branch.upstreamGone}
        <Badge variant="destructive">上游已删</Badge>
      {:else if (branch.ahead ?? 0) > 0 || (branch.behind ?? 0) > 0}
        <span
          class="shrink-0 font-mono text-xs text-muted-foreground"
          title={`领先 ${branch.ahead ?? 0}，落后 ${branch.behind ?? 0}`}
        >
          {#if (branch.ahead ?? 0) > 0}↑{branch.ahead}{/if}{#if (branch.behind ?? 0) > 0}↓{branch.behind}{/if}
        </span>
      {/if}
    </ListButton>

    <DropdownMenu.Root>
      <DropdownMenu.Trigger>
        {#snippet child({ props })}
          <ListButton
            {...props}
            variant="git-row-action"
            aria-label={`引用操作 ${branch.name}`}
            title={`引用操作 ${branch.name}`}
          >
            <IconDots aria-hidden="true" />
          </ListButton>
        {/snippet}
      </DropdownMenu.Trigger>
      <DropdownMenu.Content align="end">
        <DropdownMenu.Item onclick={() => onBranchFilterChange(branch.name)}>
          {branch.kind === "tag" ? "仅按此标签过滤历史" : "仅按此分支过滤历史"}
        </DropdownMenu.Item>
        {#if branchFilter === branch.name}
          <DropdownMenu.Item onclick={() => onBranchFilterChange(null)}>
            清除引用筛选
          </DropdownMenu.Item>
        {/if}
        {#if branch.kind !== "tag" && onToggleFavorite}
          <DropdownMenu.Item onclick={() => onToggleFavorite(branch.name)}>
            {branch.favorite ? "取消收藏分支" : "收藏分支"}
          </DropdownMenu.Item>
        {/if}
        {#if onCompareWithSelected && selectedCommitOid && selectedCommitOid !== branch.head}
          <DropdownMenu.Item onclick={() => onCompareWithSelected(branch.head)}>
            与当前选中提交比较
          </DropdownMenu.Item>
        {/if}
        {#if branch.occupiedBy}
          <DropdownMenu.Separator />
          <DropdownMenu.Item
            onclick={() =>
              branch.occupiedBy &&
              onOpenOccupiedWorktree(
                branch.occupiedBy.worktreeId,
                branch.occupiedBy.worktreePath,
              )}
          >
            打开所在工作树
          </DropdownMenu.Item>
        {:else if branch.kind === "local" && branch.name !== checkedOutBranch}
          <DropdownMenu.Separator />
          <DropdownMenu.Item onclick={() => onRequestCheckout(branch.name)}>
            检出分支…
          </DropdownMenu.Item>
        {/if}
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  </div>
{/snippet}

<aside
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-panel"
  aria-label="历史引用筛选树"
>
  <div class="space-y-1.5 border-b border-git-border px-2.5 py-2">
    <div class="flex items-center justify-between gap-2">
      <span class="text-xs font-semibold text-foreground">引用筛选</span>
      {#if branchFilter}
        <Button
          size="xs"
          variant="ghost"
          onclick={() => onBranchFilterChange(null)}
        >
          清除筛选
        </Button>
      {/if}
    </div>
    <Input
      size="sm"
      value={refQuery}
      placeholder="搜索分支或标签…"
      aria-label="筛选分支或标签"
      oninput={(e) => (refQuery = e.currentTarget.value)}
    />
  </div>

  <div class="min-h-0 flex-1 space-y-1.5 overflow-y-auto p-1.5">
    <ListButton
      variant="git-row"
      aria-selected={branchFilter === null}
      onclick={() => onBranchFilterChange(null)}
    >
      <IconGitBranch
        class="size-3.5 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
      <span class="min-w-0 flex-1 truncate">全部引用 (All)</span>
    </ListButton>

    {#if sections.length === 0}
      <p class="px-2 py-2 text-xs text-muted-foreground">
        {normalizedQuery ? "没有匹配的分支或标签。" : "暂无分支或标签引用。"}
      </p>
    {:else}
      {#each sections as section (section.id)}
        <div class="space-y-0.5">
          <ListButton
            variant="git-section"
            aria-expanded={!collapsed[section.id]}
            onclick={() => toggleSection(section.id)}
          >
            {#if collapsed[section.id]}
              <IconChevronRight class="size-3.5 shrink-0" aria-hidden="true" />
            {:else}
              <IconChevronDown class="size-3.5 shrink-0" aria-hidden="true" />
            {/if}
            <span class="min-w-0 flex-1 truncate">{section.label}</span>
            <span class="shrink-0 font-mono text-xs">
              {section.items.length}
            </span>
          </ListButton>

          {#if !collapsed[section.id]}
            {#each section.items as branch (`${section.id}:${branch.kind}:${branch.name}`)}
              {@render refRow(section.id, branch)}
            {/each}
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</aside>
