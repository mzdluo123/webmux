<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import type { WorktreeMenuActionProps } from "../../domains/workspace/branch-actions";
  import { canCommitWorktree } from "../../domains/workspace/worktree-sync";
  import BranchMenuItems from "./BranchMenuItems.svelte";

  // The worktree-level entries of a "⋯" menu, the same on the home page, in
  // the sidebar and in the repository panel: details, switching its branch,
  // home-page pinning and removal. Each place passes the actions it offers;
  // switching is left out for bare, missing and prunable worktrees. With
  // `worktree` and `onoperation`, its branch's sync actions follow the
  // switch (by `branchActions`).
  let {
    ondetails,
    onswitch,
    pinned,
    ontogglepin,
    onremove,
    worktree,
    onoperation,
    ongit,
    oncommit,
  }: WorktreeMenuActionProps & {
    ondetails?: () => void;
  } = $props();

  const canCommit = $derived(Boolean(oncommit && canCommitWorktree(worktree)));
</script>

{#if ongit}<DropdownMenu.Item onSelect={ongit}
    >{m.s8_menu_open_in_git_workbench()}</DropdownMenu.Item
  >{/if}
{#if canCommit && oncommit}<DropdownMenu.Item onSelect={oncommit}
    >{m.s8_menu_commit_changes()}</DropdownMenu.Item
  >{/if}
{#if ondetails}<DropdownMenu.Item onSelect={ondetails}
    >{m.s8_menu_worktree_details()}</DropdownMenu.Item
  >{/if}
{#if onswitch}<DropdownMenu.Item onSelect={onswitch}
    >{m.s8_menu_switch_branch()}</DropdownMenu.Item
  >{/if}
{#if worktree && onoperation}<BranchMenuItems
    target={worktree}
    {onoperation}
  />{/if}
{#if ontogglepin}<DropdownMenu.Item onSelect={ontogglepin}
    >{pinned
      ? m.s8_menu_remove_from_home()
      : m.s8_menu_add_to_home()}</DropdownMenu.Item
  >{/if}
{#if onremove}<DropdownMenu.Item variant="destructive" onSelect={onremove}
    >{m.s8_menu_remove_worktree()}</DropdownMenu.Item
  >{/if}
