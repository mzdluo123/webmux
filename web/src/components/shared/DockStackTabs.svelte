<script lang="ts">
  import IconX from "~icons/tabler/x";
  import { ListButton } from "$lib/components/ui/list-button";
  import { getDock } from "./dock-context";
  import { tick } from "svelte";
  import * as Tabs from "$lib/components/ui/tabs";
  import { m } from "$lib/paraglide/messages.js";
  import type { DockEntry } from "./dock-context";

  let {
    id,
    tabs,
    show,
  }: { id: string; tabs: DockEntry[]; show: (id: string) => void } = $props();
  const dock = getDock();
  let strip = $state<HTMLElement>();
  function reveal(node: HTMLElement) {
    const selected = node.querySelector<HTMLElement>('[aria-selected="true"]');
    if (!selected) return;
    const tab = selected.parentElement!.getBoundingClientRect(),
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
  async function closeTab(index: number) {
    const target = tabs[index].id;
    const next = target === id ? (tabs[index + 1] ?? tabs[index - 1]).id : id;
    if (next !== id) show(next);
    dock.minimize(target);
    await focusTab(next);
  }
  function switchTo(key: string) {
    show(key);
    return focusTab(key);
  }
  async function focusTab(key: string) {
    const surface = strip?.closest("[data-dock-workspace]");
    await tick();
    const panel = surface?.querySelector(
      `[data-dock-panel="${CSS.escape(key)}"]`,
    );
    const target =
      panel?.querySelector<HTMLElement>(
        '[data-dock-stack] [aria-selected="true"]',
      ) ?? panel?.querySelector<HTMLElement>("[data-dock-actions]");
    target?.focus({ preventScroll: true });
  }
</script>

<div class="stack-tabs" bind:this={strip} data-dock-stack>
  <Tabs.Root value={id} onValueChange={switchTo}>
    <Tabs.List variant="line" aria-label={m.dock_tabs()}>
      {#each tabs as tab, index (tab.id)}
        <div class="flex shrink-0 items-center border-r first:border-l">
          <Tabs.Trigger
            value={tab.id}
            title={tab.title}
            aria-controls={`dock-content-${tab.id}`}
            ><span class="truncate">{tab.title}</span></Tabs.Trigger
          >
          <ListButton
            variant="tab-close"
            aria-label={m.s5_close_tab({ path: tab.title })}
            title={m.s5_close_tab({ path: tab.title })}
            onclick={() => closeTab(index)}
            ><IconX aria-hidden="true" /></ListButton
          >
        </div>
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
