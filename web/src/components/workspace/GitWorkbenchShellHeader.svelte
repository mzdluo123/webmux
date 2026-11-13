<script lang="ts">
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconFolder from "~icons/tabler/folder";
  import IconMessageForward from "~icons/tabler/message-forward";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
  import GitTopBarBranchPopover from "./GitTopBarBranchPopover.svelte";

  let {
    context,
    branches,
    branchFilter,
    compareBaseOid,
    readonlyMode,
    reworkCount = 0,
    onSelectAllBranches,
    onBranchAction,
    onOpenRework,
    onOpenPush,
  }: {
    context: GitWorktreeContext;
    branches: readonly GitBranchItem[];
    branchFilter: string | null;
    compareBaseOid: string | null;
    readonlyMode: boolean;
    reworkCount?: number;
    onSelectAllBranches: () => void;
    onBranchAction: (
      kind: GitSharedBranchActionKind,
      branch: GitBranchItem,
    ) => void;
    onOpenRework?: () => void;
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

    <Badge variant={readonlyMode ? "warning" : "info"} class="max-w-full">
      <span
        class="truncate"
        title={readonlyMode
          ? "只读仓库（无工作树）"
          : context.headState === "detached"
            ? "游离 HEAD"
            : context.headState === "unborn"
              ? `未诞生 HEAD (${context.checkedOutBranch ?? "main"})`
              : `当前检出：${context.checkedOutBranch ?? "HEAD"}`}
      >
        {#if readonlyMode}
          只读仓库（无工作树）
        {:else if context.headState === "detached"}
          游离 HEAD
        {:else if context.headState === "unborn"}
          未诞生 HEAD ({context.checkedOutBranch ?? "main"})
        {:else}
          当前检出：{context.checkedOutBranch ?? "HEAD"}
        {/if}
      </span>
    </Badge>

    {#if branchFilter}
      <Badge variant="outline" class="max-w-full">
        <span class="truncate" title={`浏览筛选：${branchFilter}`}>
          浏览筛选：{branchFilter}
        </span>
      </Badge>
    {/if}

    <GitTopBarBranchPopover
      {branches}
      {context}
      {branchFilter}
      {compareBaseOid}
      compareTargetLabel="与当前 HEAD 比较"
      onSelectAll={onSelectAllBranches}
      {onBranchAction}
    />
  </div>

  <div class="flex flex-wrap items-center gap-2">
    {#if onOpenRework && !readonlyMode && reworkCount > 0}
      <Button
        size="sm"
        variant="outline"
        onclick={onOpenRework}
        data-testid="git-shell-open-rework-button"
      >
        <IconMessageForward aria-hidden="true" />
        返工意见 ({reworkCount})
      </Button>
    {/if}
    <Button
      size="sm"
      variant="outline"
      disabled={readonlyMode}
      onclick={onOpenPush}
    >
      <IconArrowUp aria-hidden="true" />
      推送…
    </Button>
  </div>
</header>
