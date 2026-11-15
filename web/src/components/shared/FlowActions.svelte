<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";

  // The buttons ending a branch flow: cancel and confirm, then "Done" once
  // the operation ran.
  let {
    done,
    busy,
    ready,
    label,
    destructive = false,
    onconfirm,
    onclose,
  }: {
    done: boolean;
    busy: boolean;
    /** The shown preview can be confirmed. */
    ready: boolean;
    /** The confirm button's words, for the current state. */
    label: string;
    destructive?: boolean;
    onconfirm: () => void;
    onclose: () => void;
  } = $props();
</script>

<div class="flex flex-wrap justify-end gap-2">
  {#if done}
    <Button onclick={onclose}>{m.s1_done()}</Button>
  {:else}
    <Button variant="outline" disabled={busy} onclick={onclose}
      >{m.s1_cancel()}</Button
    >
    <Button
      variant={destructive ? "destructive" : "default"}
      disabled={!ready}
      onclick={onconfirm}>{label}</Button
    >
  {/if}
</div>
