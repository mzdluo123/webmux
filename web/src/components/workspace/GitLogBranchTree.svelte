<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
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
      {
        id: "current",
        label: m.s6_log_tree_current_branch(),
        items: grouped.current,
      },
      {
        id: "favorites",
        label: m.s6_branches_group_favorites(),
        items: grouped.favorites,
      },
      {
        id: "local",
        label: m.s6_branches_group_local(),
        items: grouped.local,
      },
      {
        id: "remote",
        label: m.s6_log_tree_remote_branches(),
        items: grouped.remote,
      },
      {
        id: "tags",
        label: m.s6_branches_group_tags(),
        items: grouped.tags,
      },
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
  aria-label={m.s6_log_tree_aria()}
>
  <div class="space-y-1.5 border-b px-2.5 py-2">
    <div class="flex items-center justify-between gap-2">
      <span class="text-xs font-semibold text-foreground"
        >{m.s6_log_tree_heading()}</span
      >
      {#if branchFilter}
        <Button
          size="sm"
          variant="ghost"
          onclick={() => onBranchFilterChange(null)}
        >
          {m.s6_log_tree_clear_filter()}
        </Button>
      {/if}
    </div>
    <Input
      size="sm"
      value={refQuery}
      placeholder={m.s6_log_tree_search_placeholder()}
      aria-label={m.s6_log_tree_search_aria()}
      oninput={(e) => (refQuery = e.currentTarget.value)}
    />
  </div>

  <ScrollArea class="min-h-8 flex-1">
    <div class="space-y-1.5 p-1.5">
      <ListButton
        variant="compact-row"
        aria-selected={branchFilter === null}
        onclick={() => onBranchFilterChange(null)}
      >
        <IconGitBranch
          class="size-3.5 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        <span class="min-w-0 flex-1 truncate">{m.s6_log_tree_all_refs()}</span>
      </ListButton>

      {#if treeSections.length === 0}
        <p class="px-2 py-2 text-xs text-muted-foreground">
          {refQuery.trim()
            ? m.s6_branches_no_matches()
            : m.s6_log_tree_empty_refs()}
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
                <IconChevronRight
                  class="size-3.5 shrink-0"
                  aria-hidden="true"
                />
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
                    ? m.s6_compare_with_selected_commit()
                    : m.s6_compare_with_head()}
                  menuLabel={m.s6_log_tree_ref_actions_label({
                    name: branch.name,
                  })}
                  onSelectRow={() => onBranchFilterChange(branch.name)}
                  onAction={handleTreeAction}
                />
              {/each}
            {/if}
          </div>
        {/each}
      {/if}
    </div>
  </ScrollArea>
</aside>
