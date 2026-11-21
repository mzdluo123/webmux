<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Textarea } from "$lib/components/ui/textarea";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import HandoffImages from "./HandoffImages.svelte";
  import HandoffHistoryReader from "./HandoffHistoryReader.svelte";
  import type { HandoffEditor } from "../../hooks/sessions/handoff.svelte";
  let {
    editor: e,
    name,
    onadjust,
  }: { editor: HandoffEditor; name: string; onadjust: () => void } = $props();
</script>

{#if e.operation || e.uncertain}
  <div class="space-y-3" role="status">
    <InlineNotice tone={e.operation?.state === "succeeded" ? "success" : "info"}
      >{e.operation?.state === "succeeded"
        ? m.handoff_sent({ name })
        : e.operation?.error ||
          (e.operation
            ? m.handoff_operation_state({ state: e.operation.state })
            : m.handoff_unknown_result())}</InlineNotice
    >
    {#if e.active || e.uncertain || e.operation?.state === "unknown"}
      {#if e.operation?.targetSessionId}<Button
          variant="outline"
          disabled={e.busy}
          onclick={() => e.refresh()}>{m.handoff_refresh_result()}</Button
        >{/if}
      {#if e.active}<Button
          variant="outline"
          disabled={e.busy}
          onclick={() => e.cancel()}>{m.handoff_cancel()}</Button
        >{/if}
    {/if}
  </div>
{:else if e.preview}
  <section class="space-y-5" aria-label={m.s3_preview()}>
    {#if e.preview.blockedReason}<InlineNotice tone="warning"
        >{e.preview.blockedReason}</InlineNotice
      >{/if}
    <h3 class="font-medium">
      {m.handoff_history_pages({ count: e.preview.pages.length })}
    </h3>
    {#if e.preview.omittedCount || e.preview.gapCount}<InlineNotice
        tone="warning"
        >{m.handoff_preview_stats({
          selected: e.preview.selectedCount,
          omitted: e.preview.omittedCount,
          gaps: e.preview.gapCount,
        })}
        <Button variant="outline" size="sm" onclick={onadjust}
          >{m.handoff_adjust()}</Button
        ></InlineNotice
      >{/if}
    <HandoffImages pages={e.preview.pages} />
    <div class="space-y-2">
      <h3 class="font-medium">{m.handoff_next_task()}</h3>
      <p class="whitespace-pre-wrap break-words text-sm">
        {e.preview.userText || m.handoff_no_task()}
      </p>
    </div>
    <details>
      <summary class="cursor-pointer text-sm hover:underline"
        >{m.handoff_details()}</summary
      >
      <div class="mt-3 space-y-3">
        <p class="text-sm">
          {m.handoff_preview_stats({
            selected: e.preview.selectedCount,
            omitted: e.preview.omittedCount,
            gaps: e.preview.gapCount,
          })}
        </p>
        <p class="text-xs text-muted-foreground">
          {m.handoff_history_budget({
            used: e.preview.budget.used,
            total: e.preview.budget.history,
            expires: e.preview.expiresAt,
          })}
        </p>
        <p class="text-xs text-muted-foreground">
          {m.handoff_image_budget(e.preview.imageBudget)}
        </p>
        <p class="text-xs text-muted-foreground">
          {m.handoff_archive_stats(e.preview.imageBudget)}
        </p>
        {#if e.preview.pages.length > 0}
          <p class="text-xs text-muted-foreground">
            {m.handoff_archive_tokens(e.preview.imageBudget)}
            {#if !e.preview.imageBudget.tokenEstimateKnown}
              {m.handoff_archive_unknown()}
            {/if}
          </p>
        {/if}
        {#if e.preview.pages.length > 0}
          <p class="text-sm">
            {e.preview.capability === "supported"
              ? m.handoff_image_supported()
              : e.preview.capability === "unsupported"
                ? m.handoff_image_unsupported()
                : m.handoff_image_pending()}
          </p>
        {/if}
        <p class="text-xs text-muted-foreground">{m.handoff_image_cost()}</p>
        <details>
          <summary class="cursor-pointer text-sm hover:underline"
            >{m.handoff_prompt_preview()}</summary
          >
          <Textarea
            value={e.preview.text}
            readonly
            rows={8}
            aria-label={m.handoff_prompt_preview()}
          />
        </details>
        <details>
          <summary class="cursor-pointer text-sm hover:underline"
            >{m.handoff_text_auxiliary()}</summary
          >
          <Textarea
            value={e.preview.historyText}
            readonly
            rows={12}
            aria-label={m.handoff_text_auxiliary()}
          />
        </details>
        <HandoffHistoryReader
          sourceId={e.preview.sourceSessionId}
          snapshotId={e.preview.snapshotId}
        />
      </div>
    </details>
  </section>{/if}
