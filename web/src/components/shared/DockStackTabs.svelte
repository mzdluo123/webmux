<script lang="ts">
  import { tick } from "svelte";
  import * as Tabs from "$lib/components/ui/tabs";
  import { m } from "$lib/paraglide/messages.js";
  import type { DockEntry } from "./dock-context";

  let {
    id,
    tabs,
    show,
  }: { id: string; tabs: DockEntry[]; show: (id: string) => void } = $props();
  let strip = $state<HTMLElement>();
  function reveal(node: HTMLElement) {
    const selected = node.querySelector<HTMLElement>('[aria-selected="true"]');
    if (!selected) return;
    const tab = selected.getBoundingClientRect(),
      bounds = node.getBoundingClientRect();
    if (tab.left < bounds.left) node.scrollLeft += tab.left - bounds.left;
    else if (tab.right > bounds.right)
      node.scrollLeft += tab.right - bounds.right;
  }
  $effect(() => {
    const node = strip;
    if (!node) return;
    const observer = new ResizeObserver(() => reveal(node));
    observer.observe(node);
    return () => observer.disconnect();
  });
  async function switchTo(key: string) {
    const surface = strip?.closest("[data-dock-workspace]");
    show(key);
    await tick();
    surface
      ?.querySelector<HTMLElement>(
        `[data-dock-panel="${CSS.escape(key)}"] [data-dock-stack] [aria-selected="true"]`,
      )
      ?.focus({ preventScroll: true });
  }
</script>

<div class="stack-tabs" bind:this={strip} data-dock-stack>
  <Tabs.Root value={id} onValueChange={switchTo}>
    <Tabs.List variant="line" aria-label={m.dock_tabs()}>
      {#each tabs as tab (tab.id)}
        <Tabs.Trigger
          value={tab.id}
          title={tab.title}
          aria-controls={`dock-content-${tab.id}`}
          ><span class="truncate">{tab.title}</span></Tabs.Trigger
        >
      {/each}
    </Tabs.List>
  </Tabs.Root>
</div>

<style>
  .stack-tabs {
    min-inline-size: 0;
    overflow-x: auto;
    max-inline-size: 100%;
    scrollbar-width: thin;
  }
  .stack-tabs :global([role="tab"]) {
    max-inline-size: 12rem;
  }
  .stack-tabs :global([role="tab"][aria-selected="true"]) {
    color: var(--foreground);
  }
  .stack-tabs :global([role="tab"][aria-selected="true"])::after {
    bottom: 0;
    background: var(--primary);
  }
</style>
