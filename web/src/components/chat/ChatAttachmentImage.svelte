<script lang="ts">
  import { untrack } from "svelte";
  import { apiBlob } from "../../platform/api";
  import type { ChatAttachment } from "../../contracts/schemas";
  import IconPhoto from "~icons/tabler/photo";

  // Small thumbnail for a chat image attachment, falling back to the photo
  // icon while loading or when the image cannot be read.
  let {
    attachment,
    sessionId,
    preview,
    size = "bubble",
  }: {
    attachment: ChatAttachment;
    /** Session whose upload endpoint serves the attachment across reloads. */
    sessionId?: string;
    /** Immediate object or data URL (e.g. from the current upload or story). */
    preview?: string;
    size?: "chip" | "bubble";
  } = $props();

  let fetchedUrl = $state("");
  let failed = $state(false);
  const src = $derived(failed ? "" : preview || fetchedUrl);

  $effect(() => {
    const path = attachment.path;
    const id = sessionId;
    const local = preview;
    untrack(() => {
      fetchedUrl = "";
      failed = false;
    });
    if (local || !id || !path) return;
    const controller = new AbortController();
    let objectUrl = "";
    const query = new URLSearchParams({ path });
    void apiBlob(`/sessions/${id}/upload?${query}`, controller.signal)
      .then((blob) => {
        if (controller.signal.aborted) return;
        const mime = attachment.mimeType;
        objectUrl = URL.createObjectURL(
          mime && blob.type !== mime ? blob.slice(0, blob.size, mime) : blob,
        );
        fetchedUrl = objectUrl;
      })
      .catch(() => {
        if (!controller.signal.aborted) failed = true;
      });
    return () => {
      controller.abort();
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  });
</script>

{#snippet thumbnail(className: string)}
  <img
    {src}
    alt={attachment.name ?? attachment.path}
    class={className}
    onerror={() => (failed = true)}
  />
{/snippet}

{#if size === "bubble"}
  <span
    class="relative grid h-24 w-36 min-w-0 place-items-center overflow-hidden rounded-sm border bg-background"
  >
    {#if src}
      {@render thumbnail("absolute inset-0 size-full object-contain")}
    {:else}
      <IconPhoto class="size-3.5 text-muted-foreground" aria-hidden="true" />
    {/if}
  </span>
{:else if src}
  {@render thumbnail(
    "size-8 shrink-0 rounded-sm border bg-background object-cover",
  )}
{:else}
  <IconPhoto class="size-3.5 shrink-0" aria-hidden="true" />
{/if}
