<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";

  let {
    worktreePath,
    busy,
    canSwitch,
    previewViewport = $bindable(null),
    onsubmit,
    oncancel,
    fields,
    previewBody,
  }: {
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
  class="flex h-[min(36rem,calc(95dvh-8rem))] min-h-36 flex-col gap-3 sm:h-[min(36rem,calc(90dvh-9rem))]"
  onsubmit={(event) => {
    event.preventDefault();
    onsubmit();
  }}
>
  <div class="switch-body flex min-h-0 flex-1 flex-col gap-3">
    <div
      class="switch-fields max-h-[clamp(6rem,calc(75%-4rem),22rem)] min-h-0 shrink-0"
    >
      <ScrollArea class="h-full" viewportClasses="overscroll-contain">
        <div class="space-y-3 p-1">
          <p
            class="rounded-lg border bg-muted/40 p-3 font-mono text-xs break-all text-muted-foreground"
          >
            {worktreePath}
          </p>

          {@render fields()}
        </div>
      </ScrollArea>
    </div>
    <div class="switch-preview min-h-0 flex-1 border-t">
      <ScrollArea
        class="h-full"
        viewportClasses="overscroll-contain"
        bind:viewportRef={previewViewport}
        viewportProps={{
          tabindex: 0,
          "aria-label": m.s8_switch_preview_aria(),
        }}
      >
        <div class="space-y-3 p-1 pt-3">
          {@render previewBody()}
        </div>
      </ScrollArea>
    </div>
  </div>

  <div class="flex shrink-0 flex-wrap justify-end gap-2 border-t pt-3">
    <Button type="button" variant="outline" disabled={busy} onclick={oncancel}>
      {m.s8_cancel()}
    </Button>
    <Button type="submit" disabled={!canSwitch}
      >{busy ? m.s8_switching() : m.s8_switch_btn()}</Button
    >
  </div>
</form>

<style>
  @media (min-width: 640px) and (max-height: 480px) {
    .switch-body {
      flex-direction: row;
    }

    .switch-fields,
    .switch-preview {
      flex: 1;
      min-width: 0;
    }

    .switch-fields {
      max-height: none;
    }

    .switch-preview {
      border-top: 0;
      border-inline-start: 1px solid var(--border);
      padding-inline-start: 0.5rem;
    }
  }
</style>
