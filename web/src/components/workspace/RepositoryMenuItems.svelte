<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  // The repository-level entries shared by the sidebar row's "⋯" menu and the
  // home page's: creating a worktree, choosing which ones the home page
  // lists, batch management, settings (home page only) and details. An
  // unavailable repository offers only settings and details, where it can be
  // removed from the list.
  let {
    unavailable = false,
    onnewworktree,
    onhome,
    onmanage,
    onsettings,
    ondetails,
    ongit,
  }: {
    unavailable?: boolean;
    onnewworktree: () => void;
    onhome: () => void;
    onmanage: () => void;
    onsettings?: () => void;
    ondetails: () => void;
    ongit?: () => void;
  } = $props();
</script>

{#if !unavailable}
  {#if ongit}
    <DropdownMenu.Item onSelect={ongit}
      >{m.s8_menu_git_workbench()}</DropdownMenu.Item
    >
  {/if}
  <DropdownMenu.Item onSelect={onnewworktree}
    >{m.s8_menu_new_worktree()}</DropdownMenu.Item
  >
  <DropdownMenu.Item onSelect={onhome}
    >{m.s8_menu_home_display()}</DropdownMenu.Item
  >
  <DropdownMenu.Item onSelect={onmanage}
    >{m.s8_menu_manage_worktrees()}</DropdownMenu.Item
  >
{/if}
{#if onsettings}
  <DropdownMenu.Item onSelect={onsettings}
    >{m.s8_menu_repo_settings()}</DropdownMenu.Item
  >
{/if}
<DropdownMenu.Item onSelect={ondetails}
  >{m.s8_menu_repo_details()}</DropdownMenu.Item
>
