<script lang="ts">
  import type { AuthStatus } from "../../contracts/auth-schemas";
  import { Button } from "$lib/components/ui/button";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import {
    isWebAuthnSupported,
    localhostRedirectUrl,
  } from "../../platform/webauthn";
  import { m } from "$lib/paraglide/messages.js";

  let {
    authStatus,
  }: {
    authStatus?: AuthStatus | null;
  } = $props();

  const localhostUrl = $derived(
    typeof location !== "undefined" ? localhostRedirectUrl(location) : null,
  );
  const webAuthnOk = $derived(isWebAuthnSupported());
  const rpIdInvalid = $derived(
    Boolean(authStatus && !authStatus.rpIdValid && !localhostUrl),
  );
</script>

{#if localhostUrl}
  <InlineNotice tone="warning">
    <div class="space-y-3">
      <div class="space-y-1">
        <p class="font-medium">{m.auth_localhost_required_title()}</p>
        <p class="text-xs leading-relaxed">
          {m.auth_localhost_required_desc()}
        </p>
      </div>
      <Button
        type="button"
        size="sm"
        onclick={() => {
          location.assign(localhostUrl);
        }}
      >
        {m.auth_localhost_switch_button()}
      </Button>
    </div>
  </InlineNotice>
{:else if !webAuthnOk || rpIdInvalid}
  <InlineNotice tone="warning">
    <div class="space-y-1">
      <p class="font-medium">{m.auth_insecure_context_title()}</p>
      <p class="text-xs leading-relaxed">
        {m.auth_insecure_context_desc()}
      </p>
    </div>
  </InlineNotice>
{/if}
