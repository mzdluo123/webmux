<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import * as Command from "$lib/components/ui/command";

  // The searchable list of a repository's local and optional remote-tracking
  // branches, shared by the new-worktree form and quick start's base branch.
  // Rows are keyed by name (several branches can share a commit); a row with
  // a `reason` stays listed, disabled, with the reason under it.
  type BranchItem = { name: string; reason?: string };

  let {
    label,
    branches,
    remoteBranches = [],
    selected,
    loading = false,
    failed = false,
    disabled = false,
    search = $bindable(""),
    onselect,
  }: {
    /** The list's accessible name. */
    label: string;
    branches: readonly BranchItem[];
    remoteBranches?: readonly BranchItem[];
    selected: string;
    loading?: boolean;
    /** The branch list could not be read. */
    failed?: boolean;
    disabled?: boolean;
    search?: string;
    onselect: (name: string) => void;
  } = $props();
</script>

{#snippet branchOption(item: BranchItem)}
  <Command.Item
    value={item.name}
    disabled={disabled || !!item.reason}
    data-checked={selected === item.name}
    onSelect={() => onselect(item.name)}
  >
    <span class="min-w-0 flex-1 break-all">
      {item.name}
      {#if item.reason}
        <span class="block text-muted-foreground">{item.reason}</span>
      {/if}
    </span>
  </Command.Item>
{/snippet}

<Command.Root
  {label}
  class="h-auto min-w-0"
  filter={(value, query) =>
    value.toLowerCase().includes(query.trim().toLowerCase()) ? 1 : 0}
  disableInitialScroll
>
  <Command.Input
    bind:value={search}
    placeholder={remoteBranches.length
      ? m.s8_search_branches_placeholder()
      : m.s8_search_local_branches_placeholder()}
    aria-label={remoteBranches.length
      ? m.s8_search_branches_label()
      : m.s8_search_local_branches_label()}
    autocomplete="off"
    autocapitalize="off"
    autocorrect="off"
    spellcheck={false}
  />
  <Command.List
    aria-label={m.s8_local_branch_list_label()}
    class="max-h-[min(14rem,50dvh)] p-1"
  >
    <Command.Empty>
      {loading
        ? m.s8_loading_local_branches()
        : failed
          ? m.s8_failed_local_branches()
          : m.s8_no_matching_local_branches()}
    </Command.Empty>
    {#if remoteBranches.length}
      {#if branches.length}
        <Command.Group heading={m.s8_local_branches()}>
          {#each branches as item (item.name)}
            {@render branchOption(item)}
          {/each}
        </Command.Group>
      {/if}
      <Command.Group heading={m.s8_remote_branches()}>
        {#each remoteBranches as item (item.name)}
          {@render branchOption(item)}
        {/each}
      </Command.Group>
    {:else}
      {#each branches as item (item.name)}
        {@render branchOption(item)}
      {/each}
    {/if}
  </Command.List>
</Command.Root>
