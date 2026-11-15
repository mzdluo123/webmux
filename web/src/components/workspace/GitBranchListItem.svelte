<script lang="ts">
  import IconCloud from "~icons/tabler/cloud";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconStarFilled from "~icons/tabler/star-filled";
  import IconTag from "~icons/tabler/tag";
  import { Badge } from "$lib/components/ui/badge";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as m from "$lib/paraglide/messages.js";
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
    compareTargetLabel: rawCompareTargetLabel,
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

  const compareTargetLabel = $derived(
    rawCompareTargetLabel === undefined
      ? m.s6_compare_with_head()
      : rawCompareTargetLabel,
  );
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
    variant="compact-row"
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

      {#if branch.kind === "tag"}
        {#if branch.tagKind}
          <Badge variant="outline">
            {branch.tagKind === "annotated"
              ? m.s6_tag_annotated_short()
              : m.s6_tag_lightweight_short()}
          </Badge>
        {/if}
        {#if branch.tagSigned}
          <Badge variant="info">{m.s6_tag_signed()}</Badge>
        {/if}
      {:else if isCurrent}
        <Badge variant="info" class="max-w-full">
          <span class="truncate">{currentLabel}</span>
        </Badge>
      {:else if branch.occupiedBy}
        <Badge
          variant="warning"
          class="max-w-full"
          title={m.s6_branch_occupied_tooltip({
            path: branch.occupiedBy.worktreePath,
          })}
        >
          <span class="truncate">
            {#if occupiedLabelStyle === "verbose"}
              {branch.occupiedBy.activeSessions > 0
                ? m.s6_branch_occupied_verbose_sessions({
                    count: branch.occupiedBy.activeSessions,
                  })
                : m.s6_branch_occupied_verbose()}
            {:else}
              {branch.occupiedBy.activeSessions > 0
                ? m.s6_branch_occupied_compact_sessions({
                    count: branch.occupiedBy.activeSessions,
                  })
                : m.s6_branch_occupied_compact()}
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
