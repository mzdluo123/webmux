<script lang="ts">
  import type { Snippet } from "svelte";
  import IconDots from "~icons/tabler/dots";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";

  // Shared "⋯" trigger + lazy DropdownMenu.Content for sidebar rows.
  let {
    ariaLabel,
    tabindex,
    disabled = false,
    children,
    onmenuopen,
  }: {
    ariaLabel: string;
    tabindex?: number;
    disabled?: boolean;
    children: Snippet;
    onmenuopen?: (open: boolean) => void;
  } = $props();

  let ready = $state(false);

  // The trigger is the row's second tab stop; ArrowLeft steps back to the
  // row item, the way the row's own arrow handling treats it as a sibling.
  // Other keys stay with the primitive's handler (Enter/Space toggle,
  // ArrowDown opens), which an attribute after {...props} would replace.
  function triggerKeydown(
    event: KeyboardEvent,
    onkeydown?: (event: KeyboardEvent) => void,
  ) {
    if (event.key === "ArrowLeft") {
      const row = (event.currentTarget as HTMLElement)
        .closest("li")
        ?.querySelector<HTMLElement>("[data-tree-key]");
      if (row) {
        event.preventDefault();
        row.focus();
        return;
      }
    }
    onkeydown?.(event);
  }
</script>

<DropdownMenu.Root
  onOpenChange={(open) => {
    if (open) {
      ready = true;
      onmenuopen?.(true);
    }
  }}
>
  <DropdownMenu.Trigger {disabled}>
    {#snippet child({ props })}
      <ListButton
        {...props}
        variant="file-tree-action"
        aria-label={ariaLabel}
        title={ariaLabel}
        {tabindex}
        onkeydown={(event) =>
          triggerKeydown(
            event,
            props.onkeydown as ((event: KeyboardEvent) => void) | undefined,
          )}><IconDots aria-hidden="true" /></ListButton
      >
    {/snippet}
  </DropdownMenu.Trigger>
  {#if ready}
    <DropdownMenu.Content
      align="end"
      class="min-w-48"
      onCloseAutoFocus={() => queueMicrotask(() => onmenuopen?.(false))}
    >
      {@render children()}
    </DropdownMenu.Content>
  {/if}
</DropdownMenu.Root>
