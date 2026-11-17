<script lang="ts">
  import AuthGateFrame from "./AuthGateFrame.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { RelayAuthReason } from "../../contracts/relay-schemas";
  import { relayLoginHref } from "../../hooks/shared/relay-login";

  let {
    reason = "unauthenticated",
    returnTo,
  }: {
    reason?: Exclude<RelayAuthReason, "reconnecting">;
    returnTo: string;
  } = $props();
  const message = $derived(
    reason === "expired"
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
    <Button href={relayLoginHref(returnTo)} class="w-full" wrap>
      {m.relay_auth_login()}
    </Button>
  </div>
</AuthGateFrame>
