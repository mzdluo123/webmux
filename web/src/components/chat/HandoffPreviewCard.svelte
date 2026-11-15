<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Textarea } from "$lib/components/ui/textarea";
  import { locale } from "$lib/locale";
  import { m } from "$lib/paraglide/messages.js";
  import HandoffHistoryReader from "./HandoffHistoryReader.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import type { HandoffEditor } from "../../hooks/sessions/handoff.svelte";
  import { openSession } from "../../hooks/shared/shell.svelte";
  let { editor: e }: { editor: HandoffEditor } = $props();
  const state = $derived(
    e.delivery?.status === "unknown"
      ? "unknown"
      : e.delivery?.status === "pending"
        ? "delivering"
        : e.delivery?.status === "failed"
          ? "failed"
          : e.sealed
            ? "delivered"
            : (e.preview?.state ?? "ready"),
  );
  function stateLabel(value: string): string {
    switch (value) {
      case "ready":
        return m.s3_preview_state_ready();
      case "stale":
        return m.s3_preview_state_stale();
      case "over_budget":
        return m.s3_preview_state_over_budget();
      case "delivering":
        return m.s3_preview_state_delivering();
      case "delivered":
        return m.s3_preview_state_delivered();
      case "unknown":
        return m.s3_preview_state_unknown();
      case "failed":
        return m.s3_preview_state_failed();
      case "preparing":
        return m.s3_preview_state_preparing();
      case "superseded":
        return m.s3_preview_state_superseded();
      default:
        return value;
    }
  }
</script>

{#if e.preview}
  {@const p = e.preview}
  <section
    class="min-w-0 space-y-3 rounded-lg border p-4"
    aria-label={m.s3_preview_handoff_history()}
  >
    <div class="flex flex-wrap items-center gap-2">
      <h2 class="text-base font-medium">
        {e.sealed
          ? m.s3_preview_last_handoff()
          : m.s3_preview_handoff_history()}
      </h2>
      <Badge
        variant={state === "delivered"
          ? "success"
          : ["failed", "over_budget"].includes(state)
            ? "destructive"
            : ["unknown", "stale"].includes(state)
              ? "warning"
              : "outline"}>{stateLabel(state)}</Badge
      >
      <Badge variant="outline"
        >{p.manual
          ? m.s3_preview_strategy_manual()
          : p.strategy === "delta_since_target_last_seen"
            ? m.s3_preview_strategy_delta()
            : m.s3_preview_strategy_full()}</Badge
      >
    </div>
    <dl
      class="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1 text-xs"
    >
      <dt class="text-muted-foreground">{m.s3_preview_target_thread()}</dt>
      <dd class="break-all">{p.targetProviderThreadId}</dd>
      <dt class="text-muted-foreground">{m.s3_preview_frozen_point()}</dt>
      <dd>{p.sourcePosition} · revision {p.revision}</dd>
      <dt class="text-muted-foreground">{m.s3_preview_history_budget()}</dt>
      <dd>
        {p.budget.used.toLocaleString(locale.current)} / {p.budget.history.toLocaleString(
          locale.current,
        )} · {p.budget.source === "provider"
          ? m.s3_preview_budget_provider()
          : m.s3_preview_budget_estimate()}
      </dd>
      <dt class="text-muted-foreground">{m.s3_context_window()}</dt>
      <dd>
        {m.s3_preview_window_detail({
          window: p.budget.window.toLocaleString(locale.current),
          used: p.budget.nativeUsed.toLocaleString(locale.current),
          reserve: p.budget.reserve.toLocaleString(locale.current),
        })}
      </dd>
      <dt class="text-muted-foreground">{m.s3_preview_coverage()}</dt>
      <dd>
        {m.s3_preview_coverage_detail({
          selected: String(p.selectedCount),
          omitted: String(p.omittedCount),
        })}
      </dd>
      <dt class="text-muted-foreground">{m.s3_preview_recovery_tool()}</dt>
      <dd>
        {p.historyToolAvailable
          ? m.s3_preview_tool_available()
          : m.s3_preview_tool_fallback()}
      </dd>
      <dt class="text-muted-foreground">{m.s3_preview_transport()}</dt>
      <dd>
        {p.nativeAvailable
          ? m.s3_preview_transport_native()
          : m.s3_preview_transport_inline()}
      </dd>
    </dl>
    {#if p.omittedCount}{#key `${p.id}:${p.revision}`}<HandoffHistoryReader
          handoffId={p.id}
          revision={p.revision}
          sourceId={p.sourceConversationId}
          point={p.sourcePosition}
          omittedCount={p.omittedCount}
        />{/key}{/if}
    {#if p.reconcileRequired || p.historyOrigin !== "complete"}
      <InlineNotice tone="warning"
        >{m.s3_preview_history_gap_notice()}</InlineNotice
      >
    {/if}
    {#if p.stale && !e.sealed && !e.locked}
      <InlineNotice tone="warning">{m.s3_preview_stale_notice()}</InlineNotice>
    {/if}
    {#if p.budget.overBudget}
      <InlineNotice tone="danger"
        >{m.s3_preview_over_budget_notice()}</InlineNotice
      >
    {/if}
    <div class="space-y-2">
      <Label for="handoff-history">{m.s3_preview_history_body_label()}</Label>
      <Textarea
        id="handoff-history"
        class="max-h-72 min-h-32"
        rows={6}
        value={e.context}
        oninput={(event) => (e.context = event.currentTarget.value)}
        disabled={e.busy ||
          e.locked ||
          e.sealed ||
          (e.unpreviewed && e.conflict)}
      />
      <p class="text-xs text-muted-foreground">
        {m.s3_preview_history_body_hint({
          status:
            e.unpreviewed && e.conflict
              ? m.s3_preview_status_other_device()
              : e.saving
                ? m.s3_preview_status_saving()
                : e.dirty
                  ? m.s3_preview_status_unsaved()
                  : m.s3_preview_status_saved(),
        })}
      </p>
    </div>
    {#if e.delivery}
      {@const d = e.delivery}
      {#if d.status === "unknown"}
        <InlineNotice tone="warning"
          >{m.s3_preview_delivery_unknown({
            history: d.historyStatus,
            prompt: d.promptStatus,
          })}</InlineNotice
        >
      {:else if d.status === "failed"}
        <InlineNotice tone="danger"
          >{m.s3_preview_delivery_failed({
            step: d.step,
            errorClass: d.errorClass,
            history: d.historyStatus,
            prompt: d.promptStatus,
          })}</InlineNotice
        >
      {:else if d.status === "pending"}
        <InlineNotice
          >{d.step === "worktree"
            ? m.s3_preview_delivery_pending_worktree()
            : d.step === "prompt"
              ? m.s3_preview_delivery_pending_prompt()
              : m.s3_preview_delivery_pending_session()}</InlineNotice
        >
      {:else}
        <InlineNotice tone="success"
          >{m.s3_preview_delivery_done()}</InlineNotice
        >
      {/if}
      <div class="flex flex-wrap gap-2">
        {#if d.sessionId}<Button
            variant="outline"
            onclick={() => openSession({ id: d.sessionId })}
            >{m.s3_preview_open_target_chat()}</Button
          >{/if}
        {#if d.replacementHandoffId}<Button
            variant="outline"
            disabled={e.busy}
            onclick={() => void e.replacement()}
            >{m.s3_preview_check_replacement()}</Button
          >{/if}
        {#if d.status === "unknown"}<Button
            variant="outline"
            onclick={() => void e.refresh()}
            >{m.s3_preview_refresh_reconcile()}</Button
          >{/if}
      </div>
    {/if}
  </section>
{/if}
