<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as Card from "$lib/components/ui/card";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { detectSupport } from "../../platform/push";
  import { featureEnabled } from "../../platform/feature-host.svelte";
  import {
    adviseInstall,
    installation,
    promptInstall,
    type InstallPlatform,
  } from "../../platform/install.svelte";

  // Injectable so component tests need no real install prompt.
  let {
    platform = detectSupport(),
    status = installation,
    prompt = promptInstall,
  }: {
    platform?: InstallPlatform;
    status?: { promptable: boolean; installed: boolean };
    prompt?: () => Promise<"accepted" | "dismissed" | "unavailable">;
  } = $props();

  const advice = $derived(adviseInstall(platform, status));
  let busy = $state(false);
  let dismissed = $state(false);
  let installError = $state("");

  async function install() {
    busy = true;
    dismissed = false;
    installError = "";
    try {
      dismissed = (await prompt()) !== "accepted";
    } catch {
      installError = m.s2_install_err_prompt();
    } finally {
      busy = false;
    }
  }
</script>

<Card.Root flat id="install" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.s2_install_title()}</Card.Title>
    <Card.Description>{m.s2_install_desc()}</Card.Description>
  </Card.Header>
  <Card.Content class="space-y-4">
    <div class="flex flex-wrap items-center gap-2 text-sm" role="status">
      {#if advice === "standalone"}
        <Badge>{m.s2_install_badge_standalone()}</Badge>
      {:else if advice === "installed"}
        <Badge>{m.s2_install_badge_installed()}</Badge>
        <span class="text-muted-foreground"
          >{m.s2_install_installed_hint()}</span
        >
      {:else if advice === "prompt"}
        <Badge variant="secondary">{m.s2_install_badge_not_installed()}</Badge>
      {:else}
        <!-- Without a prompt the browser does not say whether the app is
             installed: a later tab of an installed app gets none either. -->
        <Badge variant="secondary">{m.s2_install_badge_browser_tab()}</Badge>
      {/if}
    </div>

    {#if !platform.secure}
      <InlineNotice tone="warning">
        <p class="font-medium">{m.s2_install_insecure_title()}</p>
        <p>{m.s2_install_insecure_desc()}</p>
        {#if featureEnabled("cloudflare")}
          <a class="underline underline-offset-2" href="#access"
            >{m.s2_view_public_access_config()}</a
          >
        {:else if featureEnabled("tailscale")}
          <a class="underline underline-offset-2" href="#tailscale"
            >{m.s2_view_tailscale_config()}</a
          >
        {/if}
      </InlineNotice>
    {/if}

    {#if advice === "prompt"}
      <Button disabled={busy} onclick={install}>{m.s2_install_btn()}</Button>
    {:else if advice === "ios"}
      <div class="space-y-1 text-sm">
        <p class="font-medium">{m.s2_install_ios_title()}</p>
        <ol class="list-decimal space-y-1 pl-5 text-muted-foreground">
          <li>{m.s2_ios_step_share()}</li>
          <li>{m.s2_install_ios_step_open()}</li>
        </ol>
        {#if !platform.secure}
          <p class="text-xs text-muted-foreground">
            {m.s2_install_ios_https_note()}
          </p>
        {/if}
      </div>
    {:else if advice === "browser-menu" && platform.secure && !dismissed}
      <p class="text-sm text-muted-foreground">
        {m.s2_install_browser_menu_hint()}
      </p>
    {/if}
    {#if installError}
      <InlineNotice tone="warning">{installError}</InlineNotice>
    {/if}
    {#if dismissed && advice !== "installed"}
      <p class="text-xs text-muted-foreground">
        {m.s2_install_dismissed_note()}
      </p>
    {/if}
  </Card.Content>
</Card.Root>
