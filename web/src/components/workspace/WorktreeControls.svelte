<script lang="ts">
  import IconDots from "~icons/tabler/dots";
  import IconInfoCircle from "~icons/tabler/info-circle";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import {
    branchActions,
    type WorktreeMenuActionProps,
  } from "../../domains/workspace/branch-actions";
  import { canCommitWorktree } from "../../domains/workspace/worktree-sync";
  import WorktreeMenuItems from "./WorktreeMenuItems.svelte";

  // The details button and "⋯" menu at the end of a group header.
  let {
    title,
    kind,
    ondetails,
    hidden = false,
    pinned,
    ontogglepin,
    topPinned,
    topPinDisabled,
    ontoggletoppin,
    canMoveTopPinUp,
    canMoveTopPinDown,
    onmovetoppinup,
    onmovetoppindown,
    onswitch,
    onremove,
    worktree,
    onoperation,
    ongit,
    oncommit,
  }: WorktreeMenuActionProps & {
    title: string;
    /** Names the target in labels: a worktree or a plain directory. */
    kind: "worktree" | "directory";
    ondetails: () => void;
    hidden?: boolean;
  } = $props();
  const detailsLabel = $derived(
    kind === "worktree"
      ? m.s8_worktree_details_for({ title })
      : m.s8_dir_details_for({ title }),
  );
  const menuLabel = $derived(m.s8_worktree_menu_for({ title }));
  const syncable = $derived(
    !!worktree && !!onoperation && branchActions(worktree).actions.length > 0,
  );
  const committable = $derived(
    Boolean(oncommit && canCommitWorktree(worktree)),
  );
  let menuOpen = $state(false);
  $effect(() => {
    if (hidden) menuOpen = false;
  });
</script>

<Button
  size="icon-sm"
  variant="ghost"
  aria-label={detailsLabel}
  title={detailsLabel}
  aria-haspopup="dialog"
  onclick={ondetails}><IconInfoCircle aria-hidden="true" /></Button
>
{#if onremove || ontogglepin || ontoggletoppin || onswitch || syncable || ongit || committable}
  <DropdownMenu.Root bind:open={menuOpen}>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          size="icon-sm"
          variant="ghost"
          title={menuLabel}
          aria-label={menuLabel}
          ><IconDots class="size-4" aria-hidden="true" /></Button
        >
      {/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content align="end">
      <WorktreeMenuItems
        {onswitch}
        {pinned}
        {ontogglepin}
        {topPinned}
        {topPinDisabled}
        {ontoggletoppin}
        {canMoveTopPinUp}
        {canMoveTopPinDown}
        {onmovetoppinup}
        {onmovetoppindown}
        {onremove}
        {worktree}
        {onoperation}
        {ongit}
        {oncommit}
      />
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{/if}
