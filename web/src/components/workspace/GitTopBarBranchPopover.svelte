<script lang="ts">
  import IconArrowsSplit2 from "~icons/tabler/arrows-split-2";
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as Popover from "$lib/components/ui/popover";
  import type {
    GitBranchItem,
    GitWorktreeContext,
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
    compareBaseOid = null,
    compareTargetLabel = "与当前 HEAD 比较",
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

  const grouped = $derived(
    groupAndSortGitBranches(branches, context.checkedOutBranch, query),
  );
  const popoverGroups = $derived(
    [
      { key: "current", title: "当前检出", rows: grouped.current },
      { key: "favorites", title: "常用收藏", rows: grouped.favorites },
      { key: "local", title: "本地分支", rows: grouped.local },
      { key: "remote", title: "远程分支", rows: grouped.remote },
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
        aria-label="分支快选"
        title="分支快选"
        data-testid="git-topbar-branch-popover-trigger"
      >
        <IconArrowsSplit2 aria-hidden="true" />
        <span class="max-sm:hidden">分支快选</span>
      </Button>
    {/snippet}
  </Popover.Trigger>
  <Popover.Content align="start" class="w-80 max-w-[92vw] p-2">
    <div class="space-y-2">
      <Input
        size="sm"
        value={query}
        placeholder="搜索本地或远程分支…"
        aria-label="顶栏搜索分支"
        oninput={(e) => (query = e.currentTarget.value)}
      />

      <div class="max-h-80 space-y-2 overflow-y-auto pr-0.5">
        <ListButton
          variant="git-row"
          aria-selected={branchFilter === null}
          onclick={() => {
            open = false;
            onSelectAll();
          }}
        >
          <IconGitBranch
            class="size-3.5 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <span class="min-w-0 flex-1 truncate">全部可达引用 (All)</span>
        </ListButton>

        {#if popoverGroups.length === 0}
          <p class="px-2 py-2 text-xs text-muted-foreground">
            {query.trim() ? "没有匹配的分支。" : "暂无分支引用。"}
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
                  {compareTargetLabel}
                  menuLabel={`顶栏分支操作 ${branch.name}`}
                  onSelectRow={() => triggerAction("filter-log", branch)}
                  onAction={triggerAction}
                />
              {/each}
            </div>
          {/each}
        {/if}
      </div>
    </div>
  </Popover.Content>
</Popover.Root>
