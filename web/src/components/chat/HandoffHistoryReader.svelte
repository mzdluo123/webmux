<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as m from "$lib/paraglide/messages.js";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { threadAPI } from "../../platform/thread-api";
  import { errorMessage } from "../../platform/error";
  import type { ConversationActivitySchema } from "../../contracts/schemas";
  import type { z } from "zod";
  let {
    handoffId,
    revision,
    sourceId,
    point,
    omittedCount,
  }: {
    handoffId: string;
    revision: number;
    sourceId: string;
    point: string;
    omittedCount: number;
  } = $props();
  let open = $state(false);
  let item = $state<z.infer<typeof ConversationActivitySchema> | null>(null);
  let offset = $state("0");
  let loading = $state(false);
  let error = $state("");
  let request: AbortController | null = null;
  let disposed = false;
  const itemIds: string[] = [];
  let selectionPosition = "0";
  let selectionDone = false;
  const index = $derived(
    item ? Math.max(0, itemIds.indexOf(item.activityId)) : 0,
  );
  async function load(next: number, start = "0") {
    if (next < 0 || next >= omittedCount) return;
    request?.abort();
    const active = new AbortController();
    request = active;
    loading = true;
    error = "";
    try {
      while (next >= itemIds.length && !selectionDone) {
        const page = await threadAPI.selection(
          handoffId,
          selectionPosition,
          active.signal,
        );
        if (disposed || request !== active) return;
        if (
          page.sourceConversationId !== sourceId ||
          page.sourcePosition !== point ||
          page.revision !== revision
        )
          throw new Error(m.s3_reader_err_preview_changed());
        if (
          !page.done &&
          BigInt(page.nextPosition) <= BigInt(selectionPosition)
        )
          throw new Error(m.s3_reader_err_cursor_stalled());
        for (const entry of page.items) {
          if (!entry.selected) itemIds.push(entry.activityId);
        }
        selectionPosition = page.nextPosition;
        selectionDone = page.done;
      }
      const id = itemIds[next];
      if (!id) throw new Error(m.s3_reader_err_omitted_mismatch());
      const result = await threadAPI.activity(
        sourceId,
        id,
        start,
        active.signal,
      );
      if (
        result.conversationId !== sourceId ||
        BigInt(result.position) > BigInt(point)
      )
        throw new Error(m.s3_reader_err_outside_frozen_source());
      if (!disposed && request === active) {
        item = result;
        offset = start;
      }
    } catch (cause) {
      if (!disposed && request === active && !active.signal.aborted)
        error = errorMessage(cause);
    } finally {
      if (!disposed && request === active) loading = false;
    }
  }
  $effect(() => () => {
    disposed = true;
    request?.abort();
  });
</script>

<div class="space-y-2">
  <Button
    variant="outline"
    size="sm"
    aria-expanded={open}
    onclick={() => {
      open = !open;
      if (open && !item) void load(0);
    }}>{m.s3_reader_view_omitted()}</Button
  >
  {#if open}
    <div class="min-w-0 space-y-2" aria-label={m.s3_reader_region_label()}>
      <p class="text-xs text-muted-foreground">
        {m.s3_reader_reference_hint()}
      </p>
      {#if error}<InlineNotice tone="danger">{error}</InlineNotice>{/if}
      {#if loading}<p role="status" class="text-sm">
          {m.s3_reader_loading()}
        </p>{/if}
      {#if item}
        <p class="break-all text-xs text-muted-foreground">
          {m.s3_reader_item_meta({
            current: String(index + 1),
            total: String(omittedCount),
            role: item.role,
            run: String(item.runOrdinal),
            position: item.position,
            offset,
          })}<br />{item.activityId}
        </p>
        {#if item.exportable}<Textarea
            readonly
            value={item.text}
            rows={5}
            class="max-h-64"
            aria-label={m.s3_reader_snippet_label()}
          />{:else}<InlineNotice
            >{m.s3_reader_not_exportable({
              reason: String(item.exclusionReason ?? ""),
            })}</InlineNotice
          >{/if}
        {#if item.attachments.length}<p class="text-xs text-muted-foreground">
            {m.s3_reader_attachment_refs({
              names: item.attachments
                .map((attachment) => attachment.name)
                .join(m.s3_list_separator()),
            })}
          </p>{/if}
        <div class="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={loading || index === 0}
            onclick={() => void load(index - 1)}
            >{m.s3_reader_prev_item()}</Button
          >
          <Button
            variant="outline"
            size="sm"
            disabled={loading || index >= omittedCount - 1}
            onclick={() => void load(index + 1)}
            >{m.s3_reader_next_item()}</Button
          >
          {#if item.truncated}<Button
              variant="outline"
              size="sm"
              disabled={loading}
              onclick={() => void load(index, item?.nextTextOffset)}
              >{m.s3_reader_next_chunk()}</Button
            >{/if}
          {#if offset !== "0"}<Button
              variant="outline"
              size="sm"
              disabled={loading}
              onclick={() => void load(index)}
              >{m.s3_reader_read_from_start()}</Button
            >{/if}
        </div>
      {/if}
    </div>
  {/if}
</div>
