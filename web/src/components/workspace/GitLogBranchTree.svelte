<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import {
    GitWorktreeContextSchema,
    type GitBranchItem,
    type GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import {
    groupAndSortGitBranches,
    type GitSharedBranchActionKind,
  } from "../../domains/workspace/git-branches-model";
  import GitBranchListItem from "./GitBranchListItem.svelte";

  let {
    branches,
    context,
    branchFilter,
    checkedOutBranch,
    selectedCommitOid = null,
    onBranchFilterChange,
    onBranchAction,
    onToggleFavorite,
    onCompareWithSelected,
    onRequestCheckout,
    onOpenOccupiedWorktree,
  }: {
    branches: readonly GitBranchItem[];
    context?: GitWorktreeContext;
    branchFilter: string | null;
    checkedOutBranch: string | null;
    selectedCommitOid?: string | null;
    onBranchFilterChange: (branch: string | null) => void;
    onBranchAction?: (
      kind: GitSharedBranchActionKind,
      branch: GitBranchItem,
    ) => void;
    onToggleFavorite?: (branch: string) => void;
    onCompareWithSelected?: (refHeadOid: string) => void;
    onRequestCheckout: (branch: string) => void;
    onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
  } = $props();

  let refQuery = $state("");
  let collapsed = $state<Record<string, boolean>>({});

  const effectiveContext = $derived<GitWorktreeContext>(
    context ??
      GitWorktreeContextSchema.parse({
        repositoryId: "",
        repositoryName: "",
        worktreePath: checkedOutBranch ? "/worktree" : null,
        worktreeKind: checkedOutBranch ? "main" : "none",
        headState: checkedOutBranch ? "branch" : "readonly",
        checkedOutBranch,
      }),
  );

  const grouped = $derived(
    groupAndSortGitBranches(
      branches,
      effectiveContext.checkedOutBranch,
      refQuery,
    ),
  );
  const treeSections = $derived(
    [
      { id: "current", label: "当前分支", items: grouped.current },
      { id: "favorites", label: "收藏分支", items: grouped.favorites },
      { id: "local", label: "本地分支", items: grouped.local },
      { id: "remote", label: "远端跟踪分支", items: grouped.remote },
      { id: "tags", label: "标签 (Tags)", items: grouped.tags },
    ].filter((sec) => sec.items.length > 0),
  );

  function toggleSection(id: string) {
    collapsed = { ...collapsed, [id]: !collapsed[id] };
  }

  function handleTreeAction(
    kind: GitSharedBranchActionKind,
    target: GitBranchItem,
  ) {
    if (kind === "filter-log") return onBranchFilterChange(target.name);
    if (kind === "clear-filter") return onBranchFilterChange(null);
    if (kind === "toggle-favorite" && onToggleFavorite) {
      return onToggleFavorite(target.name);
    }
    if (kind === "compare" && onCompareWithSelected) {
      return onCompareWithSelected(target.head);
    }
    if (onBranchAction) return onBranchAction(kind, target);
    if (kind === "checkout") return onRequestCheckout(target.name);
    if (kind === "open-occupied-worktree" && target.occupiedBy) {
      onOpenOccupiedWorktree(
        target.occupiedBy.worktreeId,
        target.occupiedBy.worktreePath,
      );
    }
  }
</script>

<aside
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label="历史引用筛选树"
>
  <div class="space-y-1.5 border-b px-2.5 py-2">
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
      variant="compact-row"
      aria-selected={branchFilter === null}
      onclick={() => onBranchFilterChange(null)}
    >
      <IconGitBranch
        class="size-3.5 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
      <span class="min-w-0 flex-1 truncate">全部引用 (All)</span>
    </ListButton>

    {#if treeSections.length === 0}
      <p class="px-2 py-2 text-xs text-muted-foreground">
        {refQuery.trim() ? "没有匹配的分支或标签。" : "暂无分支或标签引用。"}
      </p>
    {:else}
      {#each treeSections as section (section.id)}
        <div class="space-y-0.5">
          <ListButton
            variant="section-header"
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
              <GitBranchListItem
                {branch}
                context={effectiveContext}
                selected={branchFilter === branch.name}
                {branchFilter}
                sectionId={section.id}
                hideFavoriteInSection={section.id === "favorites"}
                compareBaseOid={selectedCommitOid ??
                  effectiveContext.headOid ??
                  null}
                compareTargetLabel={selectedCommitOid
                  ? "与当前选中提交比较"
                  : "与当前 HEAD 比较"}
                menuLabel={`引用操作 ${branch.name}`}
                onSelectRow={() => onBranchFilterChange(branch.name)}
                onAction={handleTreeAction}
              />
            {/each}
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</aside>
