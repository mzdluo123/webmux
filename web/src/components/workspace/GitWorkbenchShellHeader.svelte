<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { Snippet } from "svelte";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconFolder from "~icons/tabler/folder";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
  import GitTopBarBranchPopover from "./GitTopBarBranchPopover.svelte";

  let {
    operations,
    context,
    branches,
    branchFilter,
    compareBaseOid,
    readonlyMode,
    onSelectAllBranches,
    onBranchAction,
    onOpenPush,
  }: {
    operations?: Snippet;
    context: GitWorktreeContext;
    branches: readonly GitBranchItem[];
    branchFilter: string | null;
    compareBaseOid: string | null;
    readonlyMode: boolean;
    onSelectAllBranches: () => void;
    onBranchAction: (
      kind: GitSharedBranchActionKind,
      branch: GitBranchItem,
    ) => void;
    onOpenPush: () => void;
  } = $props();
</script>

<header
  class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b bg-background px-3 py-2"
>
  <div class="flex min-w-0 flex-wrap items-center gap-2">
    <div class="flex min-w-0 items-center gap-1.5">
      <IconFolder
        class="size-4 shrink-0 text-muted-foreground"
        aria-hidden="true"
      />
      <span class="truncate text-sm font-semibold">
        {context.repositoryName}
      </span>
    </div>

    {#if readonlyMode || context.headState !== "branch"}
      <Badge variant={readonlyMode ? "warning" : "info"} class="max-w-full">
        <span
          class="truncate"
          title={readonlyMode
            ? m.s7_readonly_repo_no_wt()
            : context.headState === "detached"
              ? m.s7_detached_head()
              : context.headState === "unborn"
                ? m.s7_unborn_head_branch({
                    branch: context.checkedOutBranch ?? "main",
                  })
                : m.s7_shell_header_checked_out({
                    branch: context.checkedOutBranch ?? "HEAD",
                  })}
        >
          {#if readonlyMode}
            {m.s7_readonly_repo_no_wt()}
          {:else if context.headState === "detached"}
            {m.s7_detached_head()}
          {:else if context.headState === "unborn"}
            {m.s7_unborn_head_branch({
              branch: context.checkedOutBranch ?? "main",
            })}
          {:else}
            {m.s7_shell_header_checked_out({
              branch: context.checkedOutBranch ?? "HEAD",
            })}
          {/if}
        </span>
      </Badge>
    {/if}

    {#if branchFilter}
      <Badge variant="outline" class="max-w-full">
        <span
          class="truncate"
          title={m.s7_shell_header_browse_filter({ branch: branchFilter })}
        >
          {m.s7_shell_header_browse_filter({ branch: branchFilter })}
        </span>
      </Badge>
    {/if}

    <GitTopBarBranchPopover
      {branches}
      {context}
      {branchFilter}
      {compareBaseOid}
      compareTargetLabel={m.s7_compare_with_head()}
      onSelectAll={onSelectAllBranches}
      {onBranchAction}
    />
  </div>

  <div class="flex min-w-0 max-w-full flex-wrap items-center gap-2">
    {@render operations?.()}
    <Button
      size="sm"
      variant="outline"
      disabled={readonlyMode}
      onclick={onOpenPush}
    >
      <IconArrowUp aria-hidden="true" />
      {m.s7_push_ellipsis_btn()}
    </Button>
  </div>
</header>
