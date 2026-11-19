<script lang="ts">
  import { ListButton } from "$lib/components/ui/list-button";
  import { Separator } from "$lib/components/ui/separator";
  import { m } from "$lib/paraglide/messages.js";
  import type { SplitDirection } from "../../domains/terminal/terminal-layout";

  let {
    busy,
    limit,
    ready,
    compact,
    oncommand,
    onsplit,
    oncreate,
    onchoose,
    onclose,
    hideLabel,
    onhide,
  }: {
    busy: boolean;
    limit: boolean;
    ready: boolean;
    compact: boolean;
    oncommand: () => void;
    onsplit: (direction: SplitDirection) => void;
    oncreate: () => void;
    onchoose?: () => void;
    onclose?: () => void;
    hideLabel?: string;
    onhide?: () => void;
  } = $props();
  const blocked = $derived(busy || limit || !ready);

  function run(action: (() => void) | undefined) {
    if (!action) return;
    oncommand();
    action();
  }
</script>

<div class="space-y-1" role="group" aria-label={m.s4_panes_group_aria()}>
  {#if compact}<Separator class="my-3" />{/if}
  <ListButton
    variant="action-sheet"
    disabled={blocked}
    onclick={() => run(oncreate)}>{m.s4_new_session_ellipsis()}</ListButton
  >
  <ListButton
    variant="action-sheet"
    disabled={busy || limit || !onchoose}
    onclick={() => run(onchoose)}
    >{m.s4_join_existing_session_ellipsis()}</ListButton
  >
  <Separator class="my-3" />
  {#if compact}
    <ListButton
      variant="action-sheet"
      disabled={blocked}
      onclick={() => run(() => onsplit("horizontal"))}
      >{m.s4_quick_new_shell()}</ListButton
    >
  {:else}
    <h3 class="px-3 py-2 text-xs text-muted-foreground">
      {m.s4_quick_new_shell()}
    </h3>
    <ListButton
      variant="action-sheet"
      aria-label={m.s4_split_horizontal_new_shell()}
      disabled={blocked}
      onclick={() => run(() => onsplit("horizontal"))}
    >
      <span class="min-w-0 flex-1">{m.s4_split_horizontal_new_shell()}</span>
      <span class="text-xs text-muted-foreground" aria-hidden="true"
        >Ctrl+B %</span
      >
    </ListButton>
    <ListButton
      variant="action-sheet"
      aria-label={m.s4_split_vertical_new_shell()}
      disabled={blocked}
      onclick={() => run(() => onsplit("vertical"))}
    >
      <span class="min-w-0 flex-1">{m.s4_split_vertical_new_shell()}</span>
      <span class="text-xs text-muted-foreground" aria-hidden="true"
        >Ctrl+B "</span
      >
    </ListButton>
  {/if}
  {#if limit}<p class="px-3 py-2 text-xs text-muted-foreground">
      {m.s4_pane_limit_reached()}
    </p>{/if}
  {#if (onhide && hideLabel) || onclose}<Separator class="my-3" />{/if}
  {#if onhide && hideLabel}
    <ListButton variant="action-sheet" onclick={onhide}>{hideLabel}</ListButton>
  {/if}
  {#if onclose}
    <ListButton
      variant="action-sheet"
      disabled={busy}
      aria-label={m.s4_close_pane_keep_session()}
      onclick={() => run(onclose)}
    >
      <span class="min-w-0 flex-1">{m.s4_close_pane_keep_session()}</span>
      {#if !compact}<span
          class="text-xs text-muted-foreground"
          aria-hidden="true">Ctrl+B x</span
        >{/if}
    </ListButton>
  {/if}
</div>
