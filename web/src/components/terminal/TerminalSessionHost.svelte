<script lang="ts">
  import type { ComponentProps } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import type { PaneRect } from "../../domains/terminal/terminal-layout";
  import SessionTerminal from "./SessionTerminal.svelte";

  let {
    pane,
    paneIndex,
    full,
    multiple,
    onactivate,
    ...session
  }: ComponentProps<typeof SessionTerminal> & {
    pane?: PaneRect;
    paneIndex: number;
    full: boolean;
    multiple: boolean;
    onactivate: () => void;
  } = $props();
  let terminal = $state<SessionTerminal>();

  export function focus() {
    terminal?.focus();
  }
  export function send(input: string) {
    terminal?.send(input);
  }
  export function revealChat() {
    terminal?.revealChat();
  }
</script>

<section
  class="session-owner"
  class:split-terminal={!!pane}
  class:active={!!pane && multiple && session.active}
  aria-label={m.s4_terminal_pane_index({ index: String(paneIndex + 1) })}
  data-session-id={session.id}
  data-active={!!pane && session.active}
  data-visible={session.visible}
  style:--pane-left={`${full ? 0 : (pane?.x ?? 0)}px`}
  style:--pane-top={`${full ? 0 : (pane?.y ?? 0)}px`}
  style:--pane-width={full ? "100%" : `${pane?.width ?? 0}px`}
  style:--pane-height={full ? "100%" : `${pane?.height ?? 0}px`}
  onpointerdowncapture={onactivate}
  onfocusin={() => {
    if (!session.active) onactivate();
  }}
>
  <SessionTerminal bind:this={terminal} {...session} />
</section>

<style>
  .session-owner {
    position: absolute;
    left: var(--pane-left);
    top: var(--pane-top);
    width: var(--pane-width);
    height: var(--pane-height);
    min-inline-size: 0;
    min-block-size: 0;
    container: terminal-pane / inline-size;
  }
  .session-owner[data-visible="false"] {
    display: none;
  }
  .session-owner.active {
    outline: 1px solid var(--ring);
    outline-offset: -1px;
  }
</style>
