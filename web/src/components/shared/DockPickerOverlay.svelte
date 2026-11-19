<script lang="ts">
  import type { Snippet } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import * as Dialog from "$lib/components/ui/dialog";
  import * as Drawer from "$lib/components/ui/drawer";
  import { m } from "$lib/paraglide/messages.js";

  let {
    open = $bindable(false),
    dialog,
    onreturnfocus,
    children,
  }: {
    open: boolean;
    dialog: boolean;
    onreturnfocus: (event: Event) => void;
    children: Snippet;
  } = $props();
</script>

{#snippet cancel()}
  <Button variant="outline" onclick={() => (open = false)}
    >{m.s1_cancel()}</Button
  >
{/snippet}

{#if dialog}
  <Dialog.Root bind:open>
    <Dialog.Content onCloseAutoFocus={onreturnfocus}>
      <Dialog.Header
        ><Dialog.Title>{m.s4_open_panel()}</Dialog.Title></Dialog.Header
      >
      {@render children()}
      <Dialog.Footer>{@render cancel()}</Dialog.Footer>
    </Dialog.Content>
  </Dialog.Root>
{:else}
  <Drawer.Root bind:open>
    <Drawer.Content onCloseAutoFocus={onreturnfocus}>
      <Drawer.Header
        ><Drawer.Title>{m.s4_open_panel()}</Drawer.Title></Drawer.Header
      >
      {@render children()}
      <Drawer.Footer>{@render cancel()}</Drawer.Footer>
    </Drawer.Content>
  </Drawer.Root>
{/if}
