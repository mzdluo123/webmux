<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import AccessSummaryCard from "./AccessSummaryCard.svelte";
  import { useMountedRead } from "../../hooks/shared/remote-request.svelte";
  import { ConfigSchema } from "../../contracts/schemas";

  const read = useMountedRead("/config", ConfigSchema);
  const tunnel = $derived(read.value?.tunnel);
</script>

<AccessSummaryCard
  id="access"
  title={m.s2_tunnel_card_title()}
  description={m.s2_tunnel_card_desc()}
  statusLabel={m.s2_tunnel_status_label()}
  state={tunnel?.state}
  error={read.error}
  loadingText={m.s2_tunnel_loading()}
  href="/settings/tunnel"
  manageLabel={m.s2_tunnel_manage()}
>
  {#snippet summary()}
    <span class="text-sm"
      >{tunnel?.named ? "Named Tunnel" : "Quick Tunnel"}</span
    >
  {/snippet}
  {#snippet detail()}
    {#if tunnel?.publicUrl || tunnel?.hostname}
      <p class="text-sm wrap-anywhere">
        {tunnel.publicUrl || tunnel.hostname}
      </p>
    {/if}
  {/snippet}
</AccessSummaryCard>
