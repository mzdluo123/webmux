<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as Dialog from "$lib/components/ui/dialog";
  import { m } from "$lib/paraglide/messages.js";
  import type { HandoffPreview } from "../../contracts/handoff-schemas";

  let { pages }: { pages: HandoffPreview["pages"] } = $props();
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
        class="h-auto w-full"
      />
      <figcaption class="flex flex-wrap items-center gap-2">
        <span class="text-xs text-muted-foreground"
          >{page.width} × {page.height} · {page.bytes} B</span
        >
        <Button
          variant="outline"
          size="sm"
          onclick={() => (enlarged = page.sha256)}
        >
          {m.handoff_image_zoom({ page: page.index + 1 })}
        </Button>
      </figcaption>
    </figure>
  {/each}
</div>
<Dialog.Root
  open={!!selected}
  onOpenChange={(open) => {
    if (!open) enlarged = null;
  }}
>
  <Dialog.Content class="sm:max-w-6xl">
    {#if selected}
      <Dialog.Header>
        <Dialog.Title
          >{m.handoff_image_page({ page: selected.index + 1 })}</Dialog.Title
        >
        <Dialog.Description
          >{selected.width} × {selected.height} · {selected.bytes} B</Dialog.Description
        >
      </Dialog.Header>
      <div class="min-w-0 overflow-auto">
        <img
          src={`data:${selected.mimeType};base64,${selected.data}`}
          alt={m.handoff_image_page({ page: selected.index + 1 })}
          width={selected.width}
          height={selected.height}
          class="max-w-none"
        />
      </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>
