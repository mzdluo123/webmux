<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import type { DockRail } from "$lib/dock-layout";
  import type { DockEntry } from "./dock-context";
  let {
    side,
    entries,
    restore,
  }: {
    side: DockRail;
    entries: DockEntry[];
    restore: (id: string) => void;
  } = $props();
</script>

{#if entries.length}
  <div data-dock-rail={side}>
    {#each entries as entry (entry.id)}
      <Button
        size="sm"
        variant="ghost"
        data-dock-tab={entry.id}
        title={entry.title}
        onclick={() => restore(entry.id)}
      >
        <span class="truncate">{entry.title}</span>
      </Button>
    {/each}
  </div>
{/if}

<style>
  [data-dock-rail] {
    display: flex;
    flex: none;
    background: var(--card);
    align-items: center;
    gap: 0.25rem;
    padding: 0.125rem;
  }
  [data-dock-rail="bottom"] {
    border-block-start: 1px solid var(--border);
    overflow-x: auto;
  }
  [data-dock-rail]:not([data-dock-rail="bottom"]) {
    flex-direction: column;
    overflow-y: auto;
    width: 2rem;
  }
  [data-dock-rail="left"] {
    border-inline-end: 1px solid var(--border);
  }
  [data-dock-rail="right"] {
    border-inline-start: 1px solid var(--border);
  }
  [data-dock-rail]:not([data-dock-rail="bottom"]) :global([data-dock-tab]) {
    writing-mode: vertical-rl;
    block-size: 100%;
    inline-size: auto;
    max-inline-size: 12rem;
    padding: 0.5rem 0.125rem;
    flex: none;
  }
</style>
