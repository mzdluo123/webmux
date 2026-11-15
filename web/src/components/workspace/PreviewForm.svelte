<script lang="ts">
  import type { Snippet } from "svelte";
  import * as m from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";

  // The form of a dialog that previews a worktree operation (removal,
  // release, rebuild) before running it: what it targets, a busy notice, the
  // preview, the last error, then cancel beside the operation's actions.
  let {
    busy,
    error,
    onsubmit,
    onclose,
    confirm,
    summary,
    children,
    actions,
  }: {
    busy: boolean;
    error: string;
    onsubmit: () => void;
    onclose: () => void;
    confirm?: {
      ready: boolean;
      disabled: boolean;
      label: string;
      variant?: "default" | "destructive";
      onreload: () => void;
    };
    summary: Snippet;
    children: Snippet;
    actions?: Snippet;
  } = $props();
</script>

<form
  class="space-y-4"
  onsubmit={(event) => {
    event.preventDefault();
    onsubmit();
  }}
>
  {@render summary()}
  {#if busy}<p role="status" class="text-sm">
      {m.s8_preview_form_busy()}
    </p>{/if}
  {@render children()}
  {#if error}<p role="alert" class="break-all text-sm text-destructive">
      {error}
    </p>{/if}
  <div class="flex flex-wrap justify-end gap-2">
    <Button type="button" variant="outline" disabled={busy} onclick={onclose}
      >{m.s8_cancel()}</Button
    >
    {#if confirm}
      {#if confirm.ready}<Button
          type="submit"
          variant={confirm.variant}
          disabled={busy || confirm.disabled}>{confirm.label}</Button
        >{:else}<Button type="button" disabled={busy} onclick={confirm.onreload}
          >{m.s8_reload_preview_btn()}</Button
        >{/if}
    {:else if actions}
      {@render actions()}
    {/if}
  </div>
</form>
