<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { featureContributions } from "../../features/registry.svelte";
  import { relayAuth } from "../../platform/relay-auth.svelte";
  import { relayDiagnostic } from "../../domains/settings/relay-diagnostics";
  import InlineNotice from "../shared/InlineNotice.svelte";

  const optimizer = $derived(featureContributions.entries.p2p?.p2pState);
  const labels = $derived({
    terminal: m.s1_page_terminal(),
    chat: m.s1_page_chat(),
    list: m.relay_stream_list(),
    other: m.relay_stream_other(),
  });
  const pathLabels = $derived({
    ws: "WS",
    wss: "WSS",
    "webrtc-direct": m.relay_path_direct(),
    "webrtc-turn": "WebRTC TURN",
  });
</script>

<section class="space-y-3" aria-label={m.relay_paths_aria()}>
  <h2 class="text-base font-semibold">{m.relay_paths_title()}</h2>
  <p class="text-sm text-muted-foreground">
    {m.relay_paths_description()}
  </p>
  {#if !optimizer}
    <InlineNotice>{m.relay_optimizer_unavailable()}</InlineNotice>
  {:else}
    <p class="text-sm" role="status">
      {m.relay_baseline_reason({
        reason: relayDiagnostic(optimizer.reason) || m.relay_awaiting_samples(),
      })}
    </p>
    {#if !relayAuth.authenticated}
      <p class="text-xs text-muted-foreground">
        {m.relay_owner_required()}
      </p>
    {/if}
    {#if optimizer.inputUncertain}
      <InlineNotice tone="warning">{m.relay_input_uncertain()}</InlineNotice>
    {/if}
    <ul class="space-y-2 text-sm">
      {#each optimizer.streams as stream (stream.id)}
        <li>
          {labels[stream.kind]} #{stream.id} · {pathLabels[stream.path]} · {stream.state ===
          "connected"
            ? m.s1_connected()
            : stream.state === "reconnecting"
              ? m.relay_reattaching()
              : m.s1_connecting_short()}
        </li>
      {:else}
        <li>{m.relay_no_streams()}</li>
      {/each}
    </ul>
  {/if}
</section>
