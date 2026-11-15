<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import SettingsPageHeader from "../components/settings/SettingsPageHeader.svelte";
  import TunnelManager from "../components/settings/TunnelManager.svelte";
  import { setBrowserChrome } from "../platform/browser-chrome";
  $effect(() => {
    setBrowserChrome(m.s1_chrome_public_access(), false);
  });
</script>

<div class="safe-area-page min-h-dvh">
  <SettingsPageHeader />
  <main class="mx-auto max-w-3xl space-y-6 px-4 py-8 sm:px-8 sm:py-10">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">
        {m.s1_public_access()}
      </h1>
      <p class="mt-2 text-sm text-muted-foreground">
        {m.s1_public_access_desc()}
      </p>
    </div>
    <TunnelManager />
  </main>
</div>
