<script lang="ts">
  import { ListButton } from "$lib/components/ui/list-button";
  import { Separator } from "$lib/components/ui/separator";
  import { m } from "$lib/paraglide/messages.js";
  import type { SplitDirection } from "../../domains/terminal/terminal-layout";

  let {
    busy,
    limit,
    ready,
    zoomed,
    onsplit,
    oncreate,
    onchoose,
    onclose,
    onzoom,
  }: {
    busy: boolean;
    limit: boolean;
    ready: boolean;
    zoomed: boolean;
    onsplit: (direction: SplitDirection) => void;
    oncreate: () => void;
    onchoose: () => void;
    onclose: () => void;
    onzoom?: () => void;
  } = $props();
</script>

<div class="space-y-1" role="group" aria-label={m.s4_panes_group_aria()}>
  <ListButton
    variant="action-sheet"
    disabled={busy || limit || !ready}
    onclick={oncreate}>{m.s4_new_session_ellipsis()}</ListButton
  >
  <ListButton variant="action-sheet" disabled={busy || limit} onclick={onchoose}
    >{m.s4_join_existing_session_ellipsis()}</ListButton
  >
  <Separator class="my-3" />
  <h3 class="px-3 py-2 text-xs text-muted-foreground">
    {m.s4_quick_new_shell()}
  </h3>
  <ListButton
    variant="action-sheet"
    aria-label={m.s4_split_horizontal_new_shell()}
    disabled={busy || limit || !ready}
    onclick={() => onsplit("horizontal")}
  >
    <span class="min-w-0 flex-1">{m.s4_split_horizontal_new_shell()}</span><span
      class="text-xs text-muted-foreground max-sm:hidden"
      aria-hidden="true">Ctrl+B %</span
    >
  </ListButton>
  <ListButton
    variant="action-sheet"
    aria-label={m.s4_split_vertical_new_shell()}
    disabled={busy || limit || !ready}
    onclick={() => onsplit("vertical")}
  >
    <span class="min-w-0 flex-1">{m.s4_split_vertical_new_shell()}</span><span
      class="text-xs text-muted-foreground max-sm:hidden"
      aria-hidden="true">Ctrl+B "</span
    >
  </ListButton>
  {#if limit}<p class="px-3 py-2 text-xs text-muted-foreground">
      {m.s4_pane_limit_reached()}
    </p>{/if}
  {#if onzoom}
    <Separator class="my-3" />
    <ListButton
      variant="action-sheet"
      aria-label={zoomed ? m.s4_restore_split() : m.s4_zoom_pane()}
      onclick={onzoom}
      ><span class="min-w-0 flex-1"
        >{zoomed ? m.s4_restore_split() : m.s4_zoom_pane()}</span
      ><span
        class="text-xs text-muted-foreground max-sm:hidden"
        aria-hidden="true">Ctrl+B z</span
      ></ListButton
    >
  {/if}
  <Separator class="my-3" />
  <ListButton
    variant="action-sheet"
    disabled={busy}
    aria-label={m.s4_close_pane_keep_session()}
    onclick={onclose}
    ><span class="min-w-0 flex-1">{m.s4_close_pane_keep_session()}</span><span
      class="text-xs text-muted-foreground max-sm:hidden"
      aria-hidden="true">Ctrl+B x</span
    ></ListButton
  >
</div>
