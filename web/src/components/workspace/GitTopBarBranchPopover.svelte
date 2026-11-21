<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconArrowsSplit2 from "~icons/tabler/arrows-split-2";
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Popover from "$lib/components/ui/popover";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import {
    groupAndSortGitBranches,
    type GitSharedBranchActionKind,
  } from "../../domains/workspace/git-branches-model";
  import GitBranchListItem from "./GitBranchListItem.svelte";
  import ListSearchInput from "../shared/ListSearchInput.svelte";

  let {
    branches,
    context,
    branchFilter,
    compareBaseOid = null,
    compareTargetLabel,
    onSelectAll,
    onBranchAction,
  }: {
    branches: readonly GitBranchItem[];
    context: GitWorktreeContext;
    branchFilter: string | null;
    compareBaseOid?: string | null;
    compareTargetLabel?: string | null;
    onSelectAll: () => void;
    onBranchAction: (
      kind: GitSharedBranchActionKind,
      branch: GitBranchItem,
    ) => void;
  } = $props();

  let open = $state(false);
  let query = $state("");

  const effectiveCompareTargetLabel = $derived(
    compareTargetLabel === undefined
      ? m.s7_compare_with_head()
      : compareTargetLabel,
  );
  const grouped = $derived(
    groupAndSortGitBranches(branches, context.checkedOutBranch, query),
  );
  const popoverGroups = $derived(
    [
      {
        key: "current",
        title: m.s7_branch_group_current(),
        rows: grouped.current,
      },
      {
        key: "favorites",
        title: m.s7_branch_group_favorites(),
        rows: grouped.favorites,
      },
      { key: "local", title: m.s7_branch_group_local(), rows: grouped.local },
      {
        key: "remote",
        title: m.s7_branch_group_remote(),
        rows: grouped.remote,
      },
    ].filter((g) => g.rows.length > 0),
  );

  function triggerAction(
    kind: GitSharedBranchActionKind,
    branch: GitBranchItem,
  ) {
    open = false;
    onBranchAction(kind, branch);
  }
</script>

<Popover.Root bind:open>
  <Popover.Trigger>
    {#snippet child({ props })}
      <Button
        {...props}
        size="sm"
        variant="outline"
        aria-label={m.s7_topbar_branch_quick_select()}
        title={m.s7_topbar_branch_quick_select()}
        data-testid="git-topbar-branch-popover-trigger"
      >
        <IconArrowsSplit2 aria-hidden="true" />
        <span class="max-sm:hidden">{m.s7_topbar_branch_quick_select()}</span>
      </Button>
    {/snippet}
  </Popover.Trigger>
  <Popover.Content
    align="start"
    class="max-h-(--bits-popover-content-available-height) w-80 max-w-[92vw] gap-0 overflow-hidden p-0"
  >
    <ListSearchInput
      bind:value={query}
      placeholder={m.s7_topbar_branch_search_placeholder()}
      label={m.s7_topbar_branch_search_aria()}
    />
    <ScrollArea class="m-2 min-h-0" viewportClasses="max-h-80">
      <div class="space-y-2 pr-0.5">
        <ListButton
          variant="compact-row"
          aria-current={branchFilter === null || undefined}
          onclick={() => {
            open = false;
            onSelectAll();
          }}
        >
          <IconGitBranch
            class="size-3.5 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <span class="min-w-0 flex-1 truncate"
            >{m.s7_topbar_all_reachable()}</span
          >
        </ListButton>

        {#if popoverGroups.length === 0}
          <p class="px-2 py-2 text-xs text-muted-foreground">
            {query.trim()
              ? m.s7_topbar_no_matching_branch()
              : m.s7_topbar_no_branches()}
          </p>
        {:else}
          {#each popoverGroups as grp (grp.key)}
            <div class="space-y-0.5">
              <div
                class="flex items-center justify-between px-1.5 py-0.5 text-xs font-semibold text-muted-foreground"
              >
                <span>{grp.title}</span>
                <span class="font-mono">{grp.rows.length}</span>
              </div>
              {#each grp.rows as branch (`${grp.key}:${branch.kind}:${branch.name}`)}
                <GitBranchListItem
                  {branch}
                  {context}
                  selected={branchFilter === branch.name}
                  {branchFilter}
                  sectionId={grp.key}
                  {compareBaseOid}
                  compareTargetLabel={effectiveCompareTargetLabel}
                  menuLabel={m.s7_topbar_branch_menu_aria({
                    name: branch.name,
                  })}
                  onSelectRow={() => triggerAction("filter-log", branch)}
                  onAction={triggerAction}
                />
              {/each}
            </div>
          {/each}
        {/if}
      </div>
    </ScrollArea>
  </Popover.Content>
</Popover.Root>
