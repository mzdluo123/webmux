<script lang="ts">
  import type { Snippet } from "svelte";
  import * as Resizable from "$lib/components/ui/resizable";
  import { m } from "$lib/paraglide/messages.js";
  import { terminalFindShortcut } from "../../hooks/terminal/terminal-find-shortcut";
  let {
    draftOpen,
    terminalPane,
    draftPane,
    onfind,
  }: {
    draftOpen: boolean;
    terminalPane: Snippet;
    draftPane: Snippet;
    onfind?: () => void;
  } = $props();
  let composerSize = $state(25);
  let terminalHeight = $state(0);
</script>

<div
  class="terminal-column"
  bind:clientHeight={terminalHeight}
  use:terminalFindShortcut={onfind}
>
  <Resizable.PaneGroup direction="vertical">
    <Resizable.Pane defaultSize={100 - composerSize} minSize={20} order={0}>
      {@render terminalPane()}
    </Resizable.Pane>
    {#if draftOpen}
      <Resizable.Handle
        withHandle
        class="draft-divider"
        aria-label={m.s4_resize_draft_height()}
      />
      <Resizable.Pane
        order={1}
        defaultSize={composerSize}
        minSize={Math.min(
          80,
          (160 / Math.max(1, (terminalHeight || 500) - 8)) * 100,
        )}
        onResize={(size) => (composerSize = size)}
        class="draft-pane"
        data-bottom-dock
      >
        {@render draftPane()}
      </Resizable.Pane>
    {/if}
  </Resizable.PaneGroup>
</div>
