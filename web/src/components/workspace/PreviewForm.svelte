<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import PreviewFormActions, {
    type PreviewConfirmation,
  } from "./PreviewFormActions.svelte";

  // The form of a dialog that previews a worktree operation (removal,
  // release, rebuild) before running it: what it targets, a busy notice, the
  // preview, the last error, then cancel beside the operation's actions.
  let {
    formId,
    busy,
    error,
    onsubmit,
    onclose,
    confirm,
    summary,
    children,
    actions,
  }: {
    formId?: string;
    busy: boolean;
    error: string;
    onsubmit: () => void;
    onclose: () => void;
    confirm?: PreviewConfirmation;
    summary: Snippet;
    children: Snippet;
    actions?: Snippet;
  } = $props();
</script>

<form
  id={formId}
  class="space-y-4"
  onsubmit={(event) => {
    event.preventDefault();
    onsubmit();
  }}
>
  {#if formId}
    <div
      class="sticky top-0 z-10 bg-popover pb-2 [@media(height<480px)]:static"
    >
      {@render summary()}
    </div>
  {:else}
    {@render summary()}
  {/if}
  {#if busy}<p role="status" class="text-sm">
      {m.s8_preview_form_busy()}
    </p>{/if}
  {@render children()}
  {#if error}<p role="alert" class="break-all text-sm text-destructive">
      {error}
    </p>{/if}
  {#if !formId}
    <PreviewFormActions {busy} {confirm} {onclose} {actions} />
  {/if}
</form>
