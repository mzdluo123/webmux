<script lang="ts">
  import { tick, untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Textarea } from "$lib/components/ui/textarea";
  import { m } from "$lib/paraglide/messages.js";
  import type { HandoffEditor } from "../../hooks/sessions/handoff.svelte";
  import { openSession } from "../../hooks/shared/shell.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import HandoffHistoryReader from "./HandoffHistoryReader.svelte";
  import HandoffImages from "./HandoffImages.svelte";
  let { editor: e }: { editor: HandoffEditor } = $props();
  let refreshButton = $state<HTMLButtonElement | null>(null);
  let cancelButton = $state<HTMLButtonElement | null>(null);
  let previewViewport = $state<HTMLDivElement>();

  $effect.pre(() => {
    const preview = e.preview;
    const operation = e.operation;
    const uncertain = e.uncertain;
    const active = document.activeElement;
    const target = untrack(() => {
      if (active && active === cancelButton) return refreshButton;
      if (
        active &&
        (active === refreshButton || previewViewport?.contains(active))
      )
        return previewViewport;
    });
    if (!target) return;

    // Keep focus near the result if the focused page or operation disappears.
    let current = true;
    void tick().then(() => {
      if (
        current &&
        preview === e.preview &&
        operation === e.operation &&
        uncertain === e.uncertain &&
        target.isConnected &&
        !active?.isConnected
      ) {
        target.focus({ preventScroll: true });
      }
    });
    return () => {
      current = false;
    };
  });
</script>

<div
  class="flex h-[clamp(8rem,34dvh,24rem)] min-w-0 flex-col overflow-clip rounded-lg border"
>
  {#if (e.source && e.error) || e.operation || e.uncertain || e.preview?.blockedReason}
    <div class="max-h-1/2 shrink-0 space-y-3 overflow-y-auto border-b p-3">
      {#if e.source && e.error}
        <InlineNotice tone="danger" size="compact">{e.error}</InlineNotice>
      {/if}
      {#if e.preview?.blockedReason}
        <InlineNotice tone="warning" size="compact"
          >{e.preview.blockedReason}</InlineNotice
        >
      {/if}
      {#if e.operation}
        <section class="space-y-3">
          <InlineNotice
            tone={e.operation.state === "succeeded"
              ? "success"
              : e.operation.state === "failed" ||
                  e.operation.state === "unknown"
                ? "warning"
                : "info"}
          >
            <div class="min-w-0 space-y-1">
              <p>
                {e.operation.state === "awaiting_upstream"
                  ? m.worktree_upstream_waiting()
                  : m.handoff_operation_state({ state: e.operation.state })}
              </p>
              {#if e.operation.error}<p class="break-words">
                  {e.operation.error}
                </p>{/if}
              {#if e.operation.worktreePath}<p class="break-all">
                  {e.operation.worktreePath}
                </p>{/if}
            </div>
          </InlineNotice>
        </section>
      {/if}
      {#if e.uncertain}
        <InlineNotice tone="warning">{m.handoff_unknown_result()}</InlineNotice>
      {/if}
      {#if e.operation || e.uncertain}
        <div class="flex flex-wrap gap-2">
          <Button
            variant="outline"
            bind:ref={refreshButton}
            aria-disabled={e.busy || undefined}
            tabindex={e.busy ? -1 : undefined}
            onclick={() => {
              if (e.busy) return;
              void e.refresh();
            }}>{m.handoff_refresh_result()}</Button
          >
          {#if e.active}<Button
              variant="outline"
              bind:ref={cancelButton}
              aria-disabled={e.busy || undefined}
              tabindex={e.busy ? -1 : undefined}
              onclick={() => {
                if (e.busy) return;
                void e.cancel();
              }}>{m.handoff_cancel()}</Button
            >{/if}
          {#if e.operation?.targetSessionId}<Button
              variant="outline"
              onclick={() => openSession({ id: e.operation!.targetSessionId })}
              >{m.s3_qs_enter_session()}</Button
            >{/if}
        </div>
      {/if}
    </div>
  {/if}
  <!-- svelte-ignore a11y_no_noninteractive_tabindex (the preview scrolls with keyboard focus) -->
  <div
    bind:this={previewViewport}
    class="min-h-0 flex-1 overflow-y-auto p-3"
    role="region"
    aria-label={m.s3_preview()}
    tabindex={e.preview ? 0 : -1}
  >
    {#if e.preview}
      <section class="min-w-0 space-y-3">
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
        {#if e.preview.format === "image"}
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
          <p class="text-sm">
            {e.preview.pages.length === 0
              ? m.handoff_archive_text_only()
              : e.preview.capability === "supported"
                ? m.handoff_image_supported()
                : e.preview.capability === "unsupported"
                  ? m.handoff_image_unsupported()
                  : m.handoff_image_pending()}
          </p>
          <p class="text-xs text-muted-foreground">{m.handoff_image_cost()}</p>
          <HandoffImages pages={e.preview.pages} />
        {/if}
        <div class="space-y-2">
          <p class="text-sm">{m.handoff_task_preview()}</p>
          <Textarea
            value={e.preview.userText}
            readonly
            rows={4}
            aria-label={m.handoff_task_preview()}
          />
        </div>
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
      </section>
    {/if}
    {#if !e.preview && !e.operation && !e.uncertain}
      <p class="text-sm text-muted-foreground" role="status">
        {e.busy ? m.s1_flow_loading_preview() : m.s3_preview()}
      </p>
    {/if}
  </div>
</div>
