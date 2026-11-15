<script lang="ts">
  import IconDots from "~icons/tabler/dots";
  import IconInfoCircle from "~icons/tabler/info-circle";
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
  const label = $derived(kind === "worktree" ? "工作树" : "目录");
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
  aria-label={`${label}详情 ${title}`}
  title={`${label}详情 ${title}`}
  aria-haspopup="dialog"
  onclick={ondetails}><IconInfoCircle aria-hidden="true" /></Button
>
{#if onremove || ontogglepin || onswitch || syncable || ongit || committable}
  <DropdownMenu.Root bind:open={menuOpen}>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          size="icon-sm"
          variant="ghost"
          title={`工作树菜单 ${title}`}
          aria-label={`工作树菜单 ${title}`}
          ><IconDots class="size-4" aria-hidden="true" /></Button
        >
      {/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content align="end">
      <WorktreeMenuItems
        {onswitch}
        {pinned}
        {ontogglepin}
        {onremove}
        {worktree}
        {onoperation}
        {ongit}
        {oncommit}
      />
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{/if}
