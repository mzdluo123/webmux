<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { relayLoginHref } from "../../hooks/shared/relay-login";
  import type { RelayReconnection } from "../../platform/relay-auth.svelte";

  let { checking, returnTo, onretry }: RelayReconnection = $props();

  const detail = $derived(
    checking ? m.relay_auth_reconnecting() : m.relay_auth_retry_stopped(),
  );
</script>

<p role="status" class="leading-relaxed">{detail}</p>
<p class="text-xs text-muted-foreground">{m.relay_auth_drafts_kept()}</p>
<div class="flex flex-wrap justify-end gap-2">
  <Button variant="outline" size="sm" disabled={checking} onclick={onretry}
    >{m.relay_auth_check_again()}</Button
  >
  <Button size="sm" href={relayLoginHref(returnTo)}
    >{m.relay_auth_login()}</Button
  >
</div>
