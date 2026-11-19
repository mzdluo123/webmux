<script lang="ts">
  import IconFolder from "~icons/tabler/folder";
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import DockPanel from "../shared/DockPanel.svelte";
  let {
    id,
    inert,
    navigationHost = $bindable(),
    children,
  }: {
    id: string;
    inert: boolean;
    navigationHost?: HTMLDivElement;
    children: Snippet;
  } = $props();
</script>

<DockPanel {id} icon={IconFolder} title={m.s5_file_tree()}>
  <div
    class="dock-code-navigation flex min-h-0 flex-1"
    {inert}
    bind:this={navigationHost}
  >
    {@render children()}
  </div>
</DockPanel>

<style>
  .dock-code-navigation :global(.code-navigation) {
    position: static;
    width: 100%;
    max-width: 100%;
    border-inline-end: 0;
  }
</style>
