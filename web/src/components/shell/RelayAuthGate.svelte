<script lang="ts">
  import AuthGateFrame from "./AuthGateFrame.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { RelayAuthReason } from "../../contracts/relay-schemas";
  import { relayLoginHref } from "../../hooks/shared/relay-login";

  let {
    reason = "unauthenticated",
    checking = false,
    returnTo,
    onretry,
  }: {
    reason?: RelayAuthReason;
    checking?: boolean;
    returnTo: string;
    onretry: () => void;
  } = $props();
  const reconnecting = $derived(reason === "reconnecting");
  const message = $derived(
    reconnecting
      ? checking
        ? m.relay_auth_reconnecting()
        : m.relay_auth_retry_stopped()
      : reason === "expired"
        ? m.relay_auth_expired()
        : reason === "revoked"
          ? m.relay_auth_revoked()
          : m.relay_auth_unauthenticated(),
  );
</script>

<AuthGateFrame
  title={m.relay_auth_title()}
  subtitle={m.relay_auth_description()}
>
  <div class="space-y-4">
    <InlineNotice tone="warning">{message}</InlineNotice>
    {#if reconnecting}
      <p class="text-sm text-muted-foreground">
        {m.relay_auth_drafts_kept()}
      </p>
      <Button
        variant="outline"
        class="w-full"
        disabled={checking}
        onclick={onretry}>{m.relay_auth_check_again()}</Button
      >
    {/if}
    <Button href={relayLoginHref(returnTo)} class="w-full" wrap>
      {m.relay_auth_login()}
    </Button>
  </div>
</AuthGateFrame>
