<script lang="ts">
  import IconDots from "~icons/tabler/dots";
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { GitBranchItem } from "../../contracts/git-workbench-schemas";

  let {
    branches,
    branchFilter,
    checkedOutBranch,
    onBranchFilterChange,
    onRequestCheckout,
    onOpenOccupiedWorktree,
  }: {
    branches: readonly GitBranchItem[];
    branchFilter: string | null;
    checkedOutBranch: string | null;
    onBranchFilterChange: (branch: string | null) => void;
    onRequestCheckout: (branch: string) => void;
    onOpenOccupiedWorktree: (worktreeId: string, path: string) => void;
  } = $props();
</script>

<aside
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-panel"
  aria-label="历史引用筛选树"
>
  <div
    class="flex items-center justify-between gap-2 border-b border-git-border px-2.5 py-1.5"
  >
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
  <div class="min-h-0 flex-1 space-y-0.5 overflow-y-auto p-1.5">
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
    {#each branches as branch (branch.name)}
      <div class="group/row relative">
        <ListButton
          variant="git-row"
          data-has-action="true"
          aria-selected={branchFilter === branch.name}
          onclick={() => onBranchFilterChange(branch.name)}
        >
          <IconGitBranch
            class="size-3.5 shrink-0 text-muted-foreground"
            aria-hidden="true"
          />
          <span class="min-w-0 flex-1 truncate" title={branch.name}>
            {branch.name}
          </span>
          {#if branch.name === checkedOutBranch}
            <Badge variant="info">HEAD</Badge>
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
            <DropdownMenu.Item
              onclick={() => onBranchFilterChange(branch.name)}
            >
              仅按此分支过滤历史
            </DropdownMenu.Item>
            {#if branch.occupiedBy}
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
              <DropdownMenu.Item onclick={() => onRequestCheckout(branch.name)}>
                检出分支…
              </DropdownMenu.Item>
            {/if}
          </DropdownMenu.Content>
        </DropdownMenu.Root>
      </div>
    {/each}
  </div>
</aside>
