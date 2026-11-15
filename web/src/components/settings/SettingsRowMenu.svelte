<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconDots from "~icons/tabler/dots";
  import IconTrash from "~icons/tabler/trash";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

  // The one action entry at the end of a settings list row: a "⋯" button that
  // opens the row's menu, so a row never carries a column of icon buttons.
  let {
    label,
    disabled = false,
    deleteAriaLabel,
    ondelete,
    children,
  }: {
    /** The row's name; it completes the accessible name. */
    label: string;
    disabled?: boolean;
    deleteAriaLabel?: string;
    ondelete?: () => void;
    children?: Snippet;
  } = $props();
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger {disabled}>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="ghost"
        size="icon"
        aria-label={m.s2_row_actions_aria({ label })}
        title={m.s2_row_actions_title()}><IconDots aria-hidden="true" /></Button
      >
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end" class="min-w-40">
    {@render children?.()}
    {#if ondelete}
      <DropdownMenu.Item
        variant="destructive"
        aria-label={deleteAriaLabel ?? m.s2_delete_item({ name: label })}
        onSelect={ondelete}
        ><IconTrash aria-hidden="true" />{m.s2_delete()}</DropdownMenu.Item
      >
    {/if}
  </DropdownMenu.Content>
</DropdownMenu.Root>
