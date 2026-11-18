<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";

  let {
    formId,
    worktreePath,
    busy,
    canSwitch,
    previewViewport = $bindable(null),
    onsubmit,
    oncancel,
    fields,
    previewBody,
  }: {
    formId?: string;
    worktreePath: string;
    busy: boolean;
    canSwitch: boolean;
    previewViewport?: HTMLElement | null;
    onsubmit: () => void;
    oncancel: () => void;
    fields: Snippet;
    previewBody: Snippet;
  } = $props();
</script>

<form
  id={formId}
  class="space-y-3"
  onsubmit={(event) => {
    event.preventDefault();
    onsubmit();
  }}
>
  <p
    class="rounded-lg border bg-muted/40 p-3 font-mono text-xs break-all text-muted-foreground"
  >
    {worktreePath}
  </p>

  {@render fields()}

  <div
    bind:this={previewViewport}
    tabindex="-1"
    class="space-y-3 border-t pt-3 outline-none"
  >
    {@render previewBody()}
  </div>

  {#if !formId}
    <div class="flex flex-wrap justify-end gap-2 border-t pt-3">
      <Button
        type="button"
        variant="outline"
        disabled={busy}
        onclick={oncancel}
      >
        {m.s8_cancel()}
      </Button>
      <Button type="submit" disabled={!canSwitch}
        >{busy ? m.s8_switching() : m.s8_switch_btn()}</Button
      >
    </div>
  {/if}
</form>
