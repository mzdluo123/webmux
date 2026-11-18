<script lang="ts">
  import RelayAuthGate from "./RelayAuthGate.svelte";
  import RelayReconnectIndicator from "./RelayReconnectIndicator.svelte";
  import {
    relayAuth,
    relayGate,
    relayReconnecting,
  } from "../../platform/relay-auth.svelte";

  let {
    checking,
    returnTo,
    hasShell,
    onretry,
  }: {
    checking: boolean;
    returnTo: string;
    /** The shell mounts its own indicator beside the sidebar. */
    hasShell: boolean;
    onretry: () => void;
  } = $props();
</script>

{#if relayGate()}
  <div class="fixed inset-0 z-[100] overflow-y-auto bg-background">
    <RelayAuthGate
      reason={relayAuth.reason === "reconnecting"
        ? undefined
        : relayAuth.reason}
      {returnTo}
    />
  </div>
{:else if relayReconnecting() && !hasShell}
  <RelayReconnectIndicator {checking} {returnTo} {onretry} />
{/if}
