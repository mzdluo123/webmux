<script lang="ts" module>
  export type PreviewConfirmation = {
    ready: boolean;
    disabled: boolean;
    label: string;
    variant?: "default" | "destructive";
    onreload: () => void;
  };
</script>

<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";

  let {
    formId,
    busy,
    confirm,
    onclose,
    actions,
  }: {
    formId?: string;
    busy: boolean;
    confirm?: PreviewConfirmation;
    onclose: () => void;
    actions?: Snippet;
  } = $props();
</script>

{#snippet previewActions()}
  {#if confirm}
    {#if confirm.ready}
      <Button
        type="submit"
        form={formId}
        variant={confirm.variant}
        disabled={busy || confirm.disabled}>{confirm.label}</Button
      >
    {:else}
      <Button type="button" disabled={busy} onclick={confirm.onreload}
        >{m.s8_reload_preview_btn()}</Button
      >
    {/if}
  {:else if actions}
    {@render actions()}
  {/if}
{/snippet}

<div
  class={formId
    ? "grid grid-cols-[auto_minmax(0,1fr)] items-end gap-2"
    : "flex flex-wrap justify-end gap-2"}
>
  <Button type="button" variant="outline" disabled={busy} onclick={onclose}
    >{m.s8_cancel()}</Button
  >
  {#if formId}
    <div class="flex min-w-0 flex-wrap justify-end gap-2">
      {@render previewActions()}
    </div>
  {:else}
    {@render previewActions()}
  {/if}
</div>
