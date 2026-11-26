<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import type {
    RelayDataProtocol,
    RelayStatus,
  } from "../../contracts/relay-schemas";

  let {
    status,
    blocked,
    testing,
    ontransport,
  }: {
    status: RelayStatus;
    blocked: boolean;
    testing?: RelayDataProtocol;
    ontransport: (value: RelayDataProtocol) => void;
  } = $props();

  const selection = $derived(status.transportSelection);
  const preferred = $derived(selection?.preferred ?? "wss");
  const supported = $derived(selection?.supported ?? false);
  const pending = $derived(selection?.pending ?? testing);
  const attempt = $derived(selection?.lastAttempt);
  const canRecover = $derived(
    preferred === "quic" || selection?.active === "quic",
  );
  const canChoose = $derived(
    !blocked &&
      !pending &&
      status.enabled &&
      status.controlReady &&
      status.storageAvailable &&
      (supported || canRecover),
  );
  let choice = $derived<RelayDataProtocol>(
    pending ?? (supported ? preferred : "wss"),
  );
  const canApply = $derived(
    canChoose &&
      (supported || choice === "wss") &&
      (choice !== preferred || choice !== selection?.active),
  );
  const protocols = { wss: "WSS/TCP", quic: "QUIC/UDP" } as const;
  const stages = $derived({
    prepare: m.relay_transport_prepare(),
    handshake: m.relay_transport_handshake(),
    probe: m.relay_transport_probe(),
    commit: m.relay_transport_commit(),
  });
  const dateFormat = $derived(
    new Intl.DateTimeFormat(locale.current, {
      dateStyle: "medium",
      timeStyle: "medium",
    }),
  );
  const numberFormat = $derived(new Intl.NumberFormat(locale.current));

  function failureReason(code: string) {
    switch (code) {
      case "expired":
        return m.relay_transport_timed_out();
      case "incompatible_version":
        return m.relay_transport_incompatible();
      case "cancelled":
        return m.relay_transport_cancelled();
      case "forbidden":
      case "unauthenticated":
      case "invalid_signature":
      case "wrong_audience":
      case "stale_generation":
        return m.relay_transport_not_authorized();
      case "unavailable":
      case "upstream_failed":
        return m.relay_transport_unavailable();
      default:
        return m.relay_transport_unconfirmed();
    }
  }
</script>

<section class="min-w-0 space-y-3" aria-label={m.relay_transport_title()}>
  <h3 class="text-base font-semibold">{m.relay_transport_title()}</h3>
  {#if !selection}
    <InlineNotice>{m.relay_transport_unsupported_host()}</InlineNotice>
  {:else}
    <p class="text-sm text-muted-foreground">
      {m.relay_transport_description()}
    </p>
    <dl class="grid min-w-0 grid-cols-2 gap-x-4 gap-y-2 text-sm">
      <div>
        <dt class="text-xs text-muted-foreground">
          {m.relay_transport_active()}
        </dt>
        <dd aria-live="polite">
          {selection.active ? protocols[selection.active] : m.relay_none()}
        </dd>
      </div>
      <div>
        <dt class="text-xs text-muted-foreground">
          {m.relay_transport_preferred()}
        </dt>
        <dd aria-live="polite">{protocols[selection.preferred]}</dd>
      </div>
    </dl>
    <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
      <SegmentedControl
        value={choice}
        onValueChange={(value) => {
          if (value === "wss" || value === "quic") choice = value;
        }}
        aria-label={m.relay_transport_choice()}
        size="form"
        disabled={!canChoose}
        wrap
      >
        <SegmentedControlItem value="wss"
          >{m.relay_transport_wss_default()}</SegmentedControlItem
        >
        <SegmentedControlItem value="quic" disabled={!supported}
          >{m.relay_transport_quic_experimental()}</SegmentedControlItem
        >
      </SegmentedControl>
      <Button disabled={!canApply} onclick={() => ontransport(choice)}
        >{m.relay_transport_apply()}</Button
      >
    </div>
    {#if pending}
      <InlineNotice>
        {m.relay_transport_testing({ protocol: protocols[pending] })}
      </InlineNotice>
    {:else if !status.enabled || !status.controlReady}
      <p class="text-sm text-muted-foreground">
        {m.relay_transport_needs_management()}
      </p>
    {:else if !supported}
      <p class="text-sm text-muted-foreground">
        {canRecover
          ? m.relay_transport_wss_recovery()
          : m.relay_transport_unsupported_node()}
      </p>
    {/if}
    {#if attempt}
      <div class="space-y-2">
        <h4 class="text-sm font-medium">{m.relay_transport_last_attempt()}</h4>
        <InlineNotice
          tone={attempt.error ? "danger" : "success"}
          size="compact"
        >
          {attempt.error
            ? m.relay_transport_failed({ reason: failureReason(attempt.error) })
            : m.relay_transport_succeeded()}
        </InlineNotice>
        <dl
          class="grid min-w-0 grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-3"
        >
          <div>
            <dt class="text-xs text-muted-foreground">
              {m.relay_transport_node()}
            </dt>
            <dd class="wrap-anywhere">
              {attempt.nodeId || m.relay_unassigned()}
            </dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">
              {m.relay_transport_tested()}
            </dt>
            <dd>{protocols[attempt.protocol]}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">
              {m.relay_transport_stage()}
            </dt>
            <dd>{stages[attempt.stage]}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">
              {m.relay_transport_duration()}
            </dt>
            <dd>
              {m.relay_transport_duration_ms({
                duration: numberFormat.format(attempt.durationMs),
              })}
            </dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">
              {m.relay_transport_completed_at()}
            </dt>
            <dd>
              <time datetime={attempt.completedAt}
                >{dateFormat.format(Date.parse(attempt.completedAt))}</time
              >
            </dd>
          </div>
        </dl>
      </div>
    {/if}
  {/if}
</section>
