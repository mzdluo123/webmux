<script lang="ts">
  import { untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { threadAPI } from "../../platform/thread-api";
  import { errorMessage } from "../../platform/error";
  import InlineNotice from "../shared/InlineNotice.svelte";
  let { sourceId, snapshotId }: { sourceId: string; snapshotId: string } =
    $props();
  let text = $state("");
  let after = $state("0");
  let offset = $state("0");
  let done = $state(false);
  let busy = $state(false);
  let error = $state("");
  let controller: AbortController | undefined;
  $effect(() => {
    if (!sourceId || !snapshotId) return;
    untrack(() => {
      controller?.abort();
      text = "";
      after = "0";
      offset = "0";
      done = false;
      error = "";
      busy = false;
    });
    return () => controller?.abort();
  });
  async function read() {
    if (busy || done) return;
    busy = true;
    controller = new AbortController();
    const current = controller;
    try {
      const page = await threadAPI.history(
        sourceId,
        snapshotId,
        after,
        offset,
        current.signal,
      );
      if (current.signal.aborted) return;
      text += page.records.map((record) => record.text).join("\n");
      after = page.nextPosition;
      offset = page.nextTextOffset;
      done = !page.more;
    } catch (e) {
      if (!current.signal.aborted) error = errorMessage(e);
    } finally {
      if (!current.signal.aborted) busy = false;
    }
  }
</script>

<div class="min-w-0 space-y-2">
  <Button variant="outline" disabled={busy || done} onclick={() => void read()}
    >{m.handoff_read_original()}</Button
  >
  {#if error}<InlineNotice tone="warning">{error}</InlineNotice>{/if}
  {#if text}<pre
      class="max-h-80 overflow-auto whitespace-pre-wrap break-words text-sm">{text}</pre>{/if}
</div>
