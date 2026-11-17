<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import AccessSummaryCard from "./AccessSummaryCard.svelte";
  import { useMountedRead } from "../../hooks/shared/remote-request.svelte";
  import {
    TailscaleStatusSchema,
    type TailscaleStatus,
  } from "../../contracts/schemas";

  const read = useMountedRead("/tailscale", TailscaleStatusSchema);
  const status = $derived(read.value);

  function modeText(mode: TailscaleStatus["mode"]): string {
    switch (mode) {
      case "off":
        return m.s2_state_off();
      case "serve":
        return m.s2_ts_mode_serve();
      case "funnel":
        return m.s2_ts_mode_funnel();
    }
  }
</script>

<AccessSummaryCard
  id="tailscale"
  title={m.s2_ts_card_title()}
  description={m.s2_ts_card_desc()}
  statusLabel={m.s2_ts_status_label()}
  state={status?.state}
  error={read.error}
  loadingText={m.s2_ts_loading()}
  href="/settings/tailscale"
  manageLabel={m.s2_ts_manage()}
>
  {#snippet summary()}
    {#if status}
      <span class="text-sm">{modeText(status.mode)}</span>
      {#if status.dnsName}
        <span class="text-sm text-muted-foreground">{status.dnsName}</span>
      {/if}
    {/if}
  {/snippet}
  {#snippet detail()}
    {#if status?.publicUrl}
      <p class="text-sm wrap-anywhere">{status.publicUrl}</p>
    {:else if status?.blockedReason || status?.error}
      <p class="text-sm text-muted-foreground wrap-anywhere">
        {status.blockedReason || status.error}
      </p>
    {:else}
      <p class="text-sm text-muted-foreground">
        {m.s2_ts_detail_off()}
      </p>
    {/if}
  {/snippet}
</AccessSummaryCard>
