<script lang="ts">
  import IconLayout from "~icons/tabler/layout";
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
    canMove,
  }: {
    id: string;
    tabs: DockEntry[];
    canMove: boolean;
    show: (id: string) => void;
  } = $props();
  const dock = getDock();
  let activationMode = $state<"manual" | "automatic">("automatic");
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
  async function closeTab(target: string, index: number) {
    const next = target === id ? (tabs[index + 1] ?? tabs[index - 1])?.id : id;
    const entry = tabs[index];
    if (!entry) return;
    if (next && next !== id) show(next);
    if (entry.onclose) entry.onclose();
    else dock.minimize(target);
    if (next) await focusTab(next);
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
      ) ??
      panel?.querySelector<HTMLElement>("[data-dock-actions]") ??
      surface
        ?.closest(".terminal-page")
        ?.querySelector<HTMLElement>("[data-dock-command-menu]");
    target?.focus({ preventScroll: true });
  }
</script>

<div
  class="stack-tabs"
  bind:this={strip}
  data-dock-stack
  role="presentation"
  onpointerdowncapture={() => (activationMode = "manual")}
  onkeydowncapture={() => (activationMode = "automatic")}
>
  <Tabs.Root {activationMode} value={id} onValueChange={switchTo}>
    <Tabs.List
      variant="line"
      aria-label={tabs.every((entry) => entry.onclose)
        ? m.s5_file_tabs()
        : m.dock_tabs()}
    >
      {#each tabs as tab, index (tab.id)}
        {@const Icon = tab.icon ?? IconLayout}
        <div
          class="stack-tab relative flex shrink-0 items-center self-stretch border-r first:border-l"
        >
          <Tabs.Trigger
            value={tab.id}
            title={tab.tooltip ?? tab.title}
            aria-label={tab.tabLabel ?? tab.title}
            draggable={canMove}
            ondragstart={(event) => {
              if (!canMove || !event.dataTransfer) return;
              event.stopPropagation();
              event.dataTransfer.effectAllowed = "move";
              event.dataTransfer.setData("text/plain", tab.title);
              dock.drag(tab.id);
            }}
            aria-controls={`dock-content-${tab.id}`}
            ><Icon aria-hidden="true" /><span class="truncate">{tab.title}</span
            ></Tabs.Trigger
          >
          <ListButton
            variant="tab-close"
            aria-label={m.s5_close_tab({ path: tab.title })}
            title={m.s5_close_tab({ path: tab.title })}
            onclick={() => closeTab(tab.id, index)}
            ><IconX aria-hidden="true" /></ListButton
          >
        </div>
      {/each}
    </Tabs.List>
  </Tabs.Root>
</div>

<style>
  .stack-tabs {
    align-self: stretch;
    min-inline-size: 0;
    overflow-x: auto;
    max-inline-size: 100%;
    scrollbar-width: thin;
  }
  .stack-tabs :global([data-slot="tabs"]),
  .stack-tabs :global([data-slot="tabs-list"]) {
    block-size: 100%;
  }
  .stack-tabs :global([data-slot="tabs-list"]) {
    padding-block: 0;
  }
  .stack-tabs :global([role="tab"]) {
    max-inline-size: 12rem;
    block-size: auto;
  }
  .stack-tab :global([role="tab"])::after {
    content: none;
  }
  .stack-tab:has(> :global([aria-selected="true"]))::after {
    position: absolute;
    inset-inline: 0.25rem;
    inset-block-end: 0;
    block-size: 2px;
    border-radius: var(--radius-sm);
    background: var(--primary);
    pointer-events: none;
    content: "";
  }
</style>
