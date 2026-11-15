<script lang="ts">
  import type { SessionInfo } from "../../contracts/schemas";
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";

  let {
    ids,
    info,
    active,
    zoomed,
    prefix = false,
    onfocus,
    onzoom,
  }: {
    ids: readonly string[];
    info: Record<string, SessionInfo>;
    active: string;
    zoomed: boolean;
    prefix?: boolean;
    onfocus: (sessionId: string) => void;
    onzoom: () => void;
  } = $props();
</script>

{#if ids.length > 1}
  <nav class="pane-tabs" aria-label={m.s4_terminal_panes_aria()}>
    {#each ids as sessionId, index (sessionId)}
      <Button
        variant="ghost"
        size="sm"
        aria-label={m.s4_switch_to_pane_aria({
          index: String(index + 1),
          title: info[sessionId]?.title || sessionId,
        })}
        aria-current={active === sessionId ? "true" : undefined}
        onclick={() => onfocus(sessionId)}
      >
        <span class="truncate"
          >{index + 1} · {info[sessionId]?.title || sessionId}</span
        >
      </Button>
    {/each}
    <Button variant="ghost" size="sm" aria-pressed={zoomed} onclick={onzoom}
      >{zoomed ? m.s4_restore_split() : m.s4_zoom_pane()}</Button
    >
  </nav>
{/if}
{#if prefix}
  <p class="prefix-notice" role="status">
    {m.s4_prefix_shortcut_bar()}
  </p>
{/if}

<style>
  .pane-tabs {
    display: flex;
    flex-shrink: 0;
    gap: 2px;
    padding: 3px 6px;
    overflow-x: auto;
    border-bottom: 1px solid var(--border);
    background: var(--card);
  }
  .pane-tabs :global(button) {
    flex-shrink: 0;
    max-inline-size: 12rem;
  }
  .prefix-notice {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: space-between;
    position: absolute;
    inset: auto 8px 8px;
    z-index: 10;
    padding: 4px 12px;
    font-size: 12px;
    background: var(--muted);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    pointer-events: none;
  }
</style>
