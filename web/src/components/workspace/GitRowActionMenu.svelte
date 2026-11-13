<script lang="ts">
  import type { Snippet } from "svelte";
  import IconDots from "~icons/tabler/dots";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";

  let {
    label,
    open = $bindable(false),
    children,
  }: {
    label: string;
    open?: boolean;
    children: Snippet;
  } = $props();
</script>

<DropdownMenu.Root bind:open>
  <DropdownMenu.Trigger>
    {#snippet child({ props })}
      <ListButton
        {...props}
        variant="file-tree-action"
        aria-label={label}
        title={label}
      >
        <IconDots aria-hidden="true" />
      </ListButton>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end">
    {@render children()}
  </DropdownMenu.Content>
</DropdownMenu.Root>
