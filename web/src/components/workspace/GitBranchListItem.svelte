<script lang="ts">
  import IconCloud from "~icons/tabler/cloud";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconStarFilled from "~icons/tabler/star-filled";
  import IconTag from "~icons/tabler/tag";
  import { Badge } from "$lib/components/ui/badge";
  import { ListButton } from "$lib/components/ui/list-button";
  import type {
    GitBranchItem,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import {
    resolveGitBranchActions,
    resolveGitBranchUpstream,
    type GitSharedBranchActionKind,
  } from "../../domains/workspace/git-branches-model";
  import GitBranchActionMenu from "./GitBranchActionMenu.svelte";

  let {
    branch,
    context,
    selected = false,
    rowRole,
    branchFilter = null,
    sectionId,
    hideFavoriteInSection = false,
    currentLabel = "HEAD",
    occupiedLabelStyle = "compact",
    compareBaseOid = null,
    compareTargetLabel = "与当前 HEAD 比较",
    menuLabel,
    onSelectRow,
    onAction,
  }: {
    branch: GitBranchItem;
    context: GitWorktreeContext;
    selected?: boolean;
    rowRole?: "option";
    branchFilter?: string | null;
    sectionId?: string;
    hideFavoriteInSection?: boolean;
    currentLabel?: string;
    occupiedLabelStyle?: "compact" | "verbose";
    compareBaseOid?: string | null;
    compareTargetLabel?: string | null;
    menuLabel: string;
    onSelectRow: () => void;
    onAction: (kind: GitSharedBranchActionKind, branch: GitBranchItem) => void;
  } = $props();

  const isCurrent = $derived(
    branch.kind === "local" && branch.name === context.checkedOutBranch,
  );
  const upstream = $derived(resolveGitBranchUpstream(branch));
  const actions = $derived(
    resolveGitBranchActions({
      item: branch,
      context,
      branchFilter,
      compareBaseOid,
      compareTargetLabel,
    }),
  );
</script>

<GitBranchActionMenu
  {branch}
  {actions}
  {sectionId}
  label={menuLabel}
  {onAction}
>
  <ListButton
    variant="git-row"
    role={rowRole}
    data-branch-item-btn="true"
    data-has-action="true"
    aria-selected={rowRole === "option" ? selected : undefined}
    aria-current={rowRole === "option" ? undefined : selected || undefined}
    onclick={onSelectRow}
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

    {#if branch.favorite && !hideFavoriteInSection}
      <IconStarFilled class="size-3 shrink-0 text-warning" aria-hidden="true" />
    {/if}

    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1">
      <span
        class="min-w-[6ch] max-w-full flex-1 truncate font-medium"
        title={branch.name}
      >
        {branch.name}
      </span>

      {#if isCurrent}
        <Badge variant="info" class="max-w-full">
          <span class="truncate">{currentLabel}</span>
        </Badge>
      {:else if branch.occupiedBy}
        <Badge
          variant="warning"
          class="max-w-full"
          title={`已在另一工作树检出：${branch.occupiedBy.worktreePath}`}
        >
          <span class="truncate">
            {#if occupiedLabelStyle === "verbose"}
              其他工作树已检出{branch.occupiedBy.activeSessions > 0
                ? ` · ${branch.occupiedBy.activeSessions} 会话`
                : ""}
            {:else}
              已占用{branch.occupiedBy.activeSessions > 0
                ? `·${branch.occupiedBy.activeSessions}`
                : ""}
            {/if}
          </span>
        </Badge>
      {/if}

      {#if branch.kind === "local" && upstream.badgeText}
        {#if upstream.kind === "gone"}
          <Badge variant="destructive" class="max-w-full">
            <span class="truncate">{upstream.badgeText}</span>
          </Badge>
        {:else if upstream.kind === "ahead" || upstream.kind === "behind" || upstream.kind === "diverged"}
          <Badge
            variant="outline"
            class="max-w-full"
            title={upstream.detailText}
          >
            <span class="truncate">{upstream.badgeText}</span>
          </Badge>
        {:else if upstream.kind === "none" && occupiedLabelStyle === "verbose"}
          <Badge variant="secondary" class="max-w-full">
            <span class="truncate">{upstream.badgeText}</span>
          </Badge>
        {/if}
      {/if}
    </div>
  </ListButton>
</GitBranchActionMenu>
