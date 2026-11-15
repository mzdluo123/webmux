<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import SettingsPageHeader from "../components/settings/SettingsPageHeader.svelte";
  import RelayManager from "../components/settings/RelayManager.svelte";
  import { setBrowserChrome } from "../platform/browser-chrome";
  import { api } from "../platform/api";
  import { NoContentSchema } from "../contracts/schemas";
  import { clearRelayAuth } from "../platform/relay-auth";
  import { relayAuth } from "../platform/relay-auth.svelte";
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "../components/shared/InlineNotice.svelte";
  import RelayPathStatus from "../components/settings/RelayPathStatus.svelte";

  let logoutFailed = $state(false);
  let loggingOut = $state(false);
  async function logout() {
    if (loggingOut) return;
    loggingOut = true;
    logoutFailed = false;
    window.dispatchEvent(new Event("webmux:relay-revoked"));
    try {
      await api("/relay/auth/logout", NoContentSchema, "POST");
      clearRelayAuth();
      window.dispatchEvent(new Event("webmux:auth"));
    } catch {
      logoutFailed = true;
    } finally {
      loggingOut = false;
    }
  }
  $effect(() => {
    setBrowserChrome(m.relay_chrome_title(), false);
  });
</script>

<div class="safe-area-page min-h-dvh settings-layout">
  <div class="col-span-full"><SettingsPageHeader /></div>
  <main class="mx-auto w-full max-w-3xl space-y-6 px-4 py-8 sm:px-8 sm:py-10">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">
        {m.relay_title()}
      </h1>
      <p class="mt-2 text-sm text-muted-foreground">
        {m.relay_page_description()}
      </p>
    </div>
    <RelayManager />
    <RelayPathStatus />
    {#if relayAuth.authenticated}
      <section class="space-y-3" aria-label={m.relay_login_aria()}>
        <h2 class="text-base font-semibold">{m.relay_login_title()}</h2>
        <p class="text-sm text-muted-foreground">
          {m.relay_logout_description()}
        </p>
        <Button
          variant="outline"
          disabled={loggingOut}
          onclick={() => void logout()}>{m.relay_logout()}</Button
        >
        {#if logoutFailed}<InlineNotice tone="danger"
            >{m.relay_logout_error()}</InlineNotice
          >{/if}
      </section>
    {/if}
  </main>
</div>
