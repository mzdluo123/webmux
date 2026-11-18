<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as Dialog from "$lib/components/ui/dialog";
  import { m } from "$lib/paraglide/messages.js";
  import type { HandoffPreview } from "../../contracts/handoff-schemas";

  let { pages }: { pages: HandoffPreview["pages"] } = $props();
  let open = $state(false);
  let actualSize = $state(false);
  let trigger: HTMLElement | undefined;
  let imageViewport = $state<HTMLDivElement>();
  let enlarged = $state<string | null>(null);
  const selected = $derived(pages.find((page) => page.sha256 === enlarged));
</script>

<div class="grid min-w-0 gap-4 sm:grid-cols-2">
  {#each pages as page (page.sha256)}
    <figure class="min-w-0 space-y-2">
      <img
        src={`data:${page.mimeType};base64,${page.data}`}
        alt={m.handoff_image_page({ page: page.index + 1 })}
        width={page.width}
        height={page.height}
        class="aspect-4/3 h-auto w-full rounded-md bg-muted object-contain"
      />
      <figcaption class="flex flex-wrap items-center gap-2">
        <span class="text-xs text-muted-foreground"
          >{page.width} × {page.height} · {page.bytes} B</span
        >
        <Button
          variant="outline"
          size="sm"
          onclick={(event) => {
            trigger = event.currentTarget;
            enlarged = page.sha256;
            actualSize = false;
            open = true;
          }}
        >
          {m.handoff_image_zoom({ page: page.index + 1 })}
        </Button>
      </figcaption>
    </figure>
  {/each}
</div>
<Dialog.Root open={open && !!selected} onOpenChange={(value) => (open = value)}>
  <Dialog.Content
    class="flex h-[min(48rem,calc(100dvh-2rem))] flex-col overflow-hidden sm:max-w-6xl"
    onCloseAutoFocus={(event) => {
      if (!trigger?.isConnected) return;
      event.preventDefault();
      trigger.focus({ preventScroll: true });
    }}
  >
    {#if selected}
      <Dialog.Header class="shrink-0">
        <Dialog.Title
          >{m.handoff_image_page({ page: selected.index + 1 })}</Dialog.Title
        >
        <Dialog.Description
          >{selected.width} × {selected.height} · {selected.bytes} B</Dialog.Description
        >
      </Dialog.Header>
      <div>
        <Button
          variant="outline"
          size="sm-touch"
          aria-pressed={actualSize}
          onclick={() => {
            actualSize = !actualSize;
            imageViewport?.scrollTo({ top: 0, left: 0 });
          }}>{m.s5_actual_size_1_1()}</Button
        >
      </div>
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (the original image can pan with the keyboard) -->
      <div
        bind:this={imageViewport}
        class="min-h-0 flex-1 overflow-auto overscroll-contain"
        role="region"
        aria-label={m.handoff_image_page({ page: selected.index + 1 })}
        tabindex={actualSize ? 0 : undefined}
      >
        <img
          src={`data:${selected.mimeType};base64,${selected.data}`}
          alt={m.handoff_image_page({ page: selected.index + 1 })}
          width={selected.width}
          height={selected.height}
          class={actualSize ? "max-w-none" : "size-full object-scale-down"}
        />
      </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>
