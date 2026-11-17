<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Textarea } from "$lib/components/ui/textarea";
  import { m } from "$lib/paraglide/messages.js";
  import type { HandoffEditor } from "../../hooks/sessions/handoff.svelte";
  import { openSession } from "../../hooks/shared/shell.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import HandoffHistoryReader from "./HandoffHistoryReader.svelte";
  let { editor: e }: { editor: HandoffEditor } = $props();
</script>

{#if e.preview}
  <section class="min-w-0 space-y-3">
    <p class="text-sm text-muted-foreground">{m.handoff_source_dependency()}</p>
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
    <Textarea
      value={e.preview.text}
      readonly
      rows={12}
      aria-label={m.s3_handoff_generate_preview()}
    />
    <HandoffHistoryReader
      sourceId={e.preview.sourceSessionId}
      snapshotId={e.preview.snapshotId}
    />
  </section>
{/if}
{#if e.operation}
  <section class="space-y-3">
    <InlineNotice
      tone={e.operation.state === "succeeded"
        ? "success"
        : e.operation.state === "failed" || e.operation.state === "unknown"
          ? "warning"
          : "info"}
    >
      <div class="min-w-0 space-y-1">
        <p>{m.handoff_operation_state({ state: e.operation.state })}</p>
        {#if e.operation.error}<p class="break-words">
            {e.operation.error}
          </p>{/if}
        {#if e.operation.worktreePath}<p class="break-all">
            {e.operation.worktreePath}
          </p>{/if}
      </div>
    </InlineNotice>
    <div class="flex flex-wrap gap-2">
      <Button
        variant="outline"
        disabled={e.busy}
        onclick={() => void e.refresh()}>{m.handoff_refresh_result()}</Button
      >
      {#if e.active}<Button
          variant="outline"
          disabled={e.busy}
          onclick={() => void e.cancel()}>{m.handoff_cancel()}</Button
        >{/if}
      {#if e.operation.targetSessionId}<Button
          variant="outline"
          onclick={() => openSession({ id: e.operation!.targetSessionId })}
          >{m.s3_qs_enter_session()}</Button
        >{/if}
    </div>
  </section>
{/if}

{#if e.uncertain}
  <InlineNotice tone="warning">{m.handoff_unknown_result()}</InlineNotice>
  <Button variant="outline" disabled={e.busy} onclick={() => void e.refresh()}
    >{m.handoff_refresh_result()}</Button
  >
{/if}
