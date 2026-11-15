<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconExternalLink from "~icons/tabler/external-link";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import * as Card from "$lib/components/ui/card";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import TunnelStateBadge from "./TunnelStateBadge.svelte";
  import TunnelConfiguration from "./TunnelConfiguration.svelte";
  import { useCopied } from "../../hooks/shared/copied.svelte";
  import { useTunnelSettings } from "../../hooks/settings/tunnel-settings.svelte";
  import type { TunnelSettingsUpdate } from "../../contracts/schemas";

  const tunnel = useTunnelSettings();
  let confirmOpen = $state(false);
  let action = $state<"enable" | "disable" | "reset" | "save">();
  let pending = $state<TunnelSettingsUpdate>();
  const link = useCopied();
  let copyFallback = $state("");
  const publicPage = $derived(
    !!tunnel.view &&
      (tunnel.view.publicUrl === location.origin ||
        tunnel.view.hostname === location.hostname),
  );
  const running = $derived(
    tunnel.view?.state === "starting" || tunnel.view?.state === "ready",
  );
  function confirmTitle(next: typeof action): string {
    switch (next) {
      case "enable":
        return m.s2_tunnel_confirm_enable();
      case "disable":
        return m.s2_tunnel_confirm_disable();
      case "reset":
        return m.s2_tunnel_confirm_reset();
      case "save":
        return m.s2_tunnel_confirm_save();
      default:
        return m.s2_confirm_action();
    }
  }
  $effect(() => {
    if (!confirmOpen) {
      pending = undefined;
      action = undefined;
    }
  });

  function confirm(next: typeof action) {
    action = next;
    confirmOpen = true;
  }
  function save(draft: TunnelSettingsUpdate) {
    if (publicPage && running) {
      pending = draft;
      confirm("save");
    } else void tunnel.save(draft);
  }
  async function apply() {
    const selected = action;
    const draft = pending;
    confirmOpen = false;
    if (selected === "save" && draft) await tunnel.save(draft);
    else if (selected === "reset") await tunnel.reset();
    else if (selected) await tunnel.setEnabled(selected === "enable");
  }
  async function copyLink() {
    const url = tunnel.view?.publicUrl;
    if (!url) return;
    copyFallback = (await link.copy(url)) ? "" : url;
  }
</script>

<div class="space-y-6">
  {#if tunnel.loading}
    <p role="status">{m.s2_tunnel_mgr_loading()}</p>
  {/if}
  {#if tunnel.error}<InlineNotice tone="danger">{tunnel.error}</InlineNotice
    >{/if}
  {#if tunnel.notice}<p role="status" class="text-sm text-muted-foreground">
      {tunnel.notice}
    </p>{/if}
  {#if tunnel.view}
    {@const current = tunnel.view}
    <Card.Root>
      <Card.Header>
        <Card.Title>{m.s2_tunnel_status_title()}</Card.Title>
        <Card.Description>{m.s2_tunnel_status_desc()}</Card.Description>
      </Card.Header>
      <Card.Content class="space-y-4">
        <div class="flex flex-wrap items-center gap-2" role="status">
          <TunnelStateBadge state={current.state} />
          <span class="text-sm"
            >{current.mode === "named" ? "Named Tunnel" : "Quick Tunnel"}</span
          >
          <span class="text-sm text-muted-foreground"
            >{current.enabled
              ? m.s2_tunnel_desired_on()
              : m.s2_tunnel_desired_off()}</span
          >
        </div>
        <p class="text-sm text-muted-foreground">
          {current.source === "web"
            ? m.s2_tunnel_source_web()
            : m.s2_tunnel_source_config()}
        </p>
        {#if current.hostname}<p class="text-sm wrap-anywhere">
            {m.s2_tunnel_hostname_prefix()}<code>{current.hostname}</code>
          </p>{/if}
        {#if current.publicUrl}
          <p class="text-sm wrap-anywhere">
            {m.s2_tunnel_public_url_prefix()}<code>{current.publicUrl}</code>
          </p>
          <Button variant="outline" onclick={copyLink}
            >{link.copied ? m.s2_copied() : m.s2_tunnel_copy_link()}</Button
          >
          {#if copyFallback}
            <div class="grid gap-2">
              <Label for="tunnel-copy">{m.s2_tunnel_copy_fallback()}</Label>
              <Input
                id="tunnel-copy"
                readonly
                value={copyFallback}
                onfocus={(event) => event.currentTarget.select()}
              />
            </div>
          {/if}
        {/if}
        {#if current.error}<InlineNotice tone="danger"
            >{current.error}</InlineNotice
          >{/if}
        {#if !current.storageAvailable}
          <InlineNotice tone="warning"
            >{m.s2_tunnel_storage_warn({
              error: current.storageError ?? "",
            })}</InlineNotice
          >
        {/if}
        {#if !current.canEnable && current.enableBlockedReason}
          <InlineNotice tone="warning"
            >{current.enableBlockedReason}</InlineNotice
          >
        {/if}
        <div class="flex flex-wrap gap-2">
          {#if current.enabled || running}
            {#if current.state === "failed"}
              <Button
                disabled={tunnel.busy ||
                  !current.canEnable ||
                  !current.storageAvailable}
                onclick={() => confirm("enable")}
                >{m.s2_tunnel_retry_enable()}</Button
              >
            {/if}
            <Button
              variant="destructive"
              disabled={tunnel.busy}
              onclick={() => confirm("disable")}
              >{m.s2_tunnel_disable_btn()}</Button
            >
          {:else}
            <Button
              disabled={tunnel.busy ||
                !current.canEnable ||
                !current.storageAvailable}
              onclick={() => confirm("enable")}
              >{m.s2_tunnel_enable_btn()}</Button
            >
          {/if}
          {#if tunnel.shutdownUnpersisted}
            <Button
              variant="outline"
              disabled={tunnel.busy}
              onclick={() => void tunnel.setEnabled(false)}
              >{m.s2_tunnel_resave_off()}</Button
            >
          {/if}
          <Button
            variant="outline"
            disabled={tunnel.busy}
            onclick={() => void tunnel.refresh()}
            >{m.s2_tunnel_refresh()}</Button
          >
        </div>
      </Card.Content>
    </Card.Root>
    <Card.Root>
      <Card.Header>
        <Card.Title>{m.s2_tunnel_config_title()}</Card.Title>
        <Card.Description>{m.s2_tunnel_config_desc()}</Card.Description>
      </Card.Header>
      <Card.Content class="space-y-4">
        {#key tunnel.editVersion}
          <TunnelConfiguration
            {current}
            disabled={tunnel.busy || !current.storageAvailable}
            onsave={save}
          />
        {/key}
        <p class="text-sm text-muted-foreground">
          {m.s2_tunnel_hostname_push_note()}
        </p>
        <p class="text-sm text-muted-foreground">
          {m.s2_tunnel_auth_token_note()}
        </p>
        <Button
          variant="outline"
          href="https://github.com/mzdluo123/webmux/blob/main/docs/cloudflare-tunnel.md"
          target="_blank"
          rel="noreferrer"
          >{m.s2_tunnel_open_guide()}<IconExternalLink
            aria-hidden="true"
          /></Button
        >
      </Card.Content>
    </Card.Root>
    <Card.Root>
      <Card.Header>
        <Card.Title>{m.s2_tunnel_reset_title()}</Card.Title>
        <Card.Description>{m.s2_tunnel_reset_desc()}</Card.Description>
      </Card.Header>
      <Card.Content>
        <Button
          variant="outline"
          disabled={tunnel.busy ||
            !current.storageAvailable ||
            current.source !== "web"}
          onclick={() => confirm("reset")}>{m.s2_tunnel_reset_title()}</Button
        >
      </Card.Content>
    </Card.Root>
  {:else if !tunnel.loading}
    <Button variant="outline" onclick={() => void tunnel.refresh()}
      >{m.s2_tunnel_reload_btn()}</Button
    >
  {/if}
  <InlineNotice tone="warning">{m.s2_tunnel_security_warn()}</InlineNotice>
</div>

<ConfirmDialog
  bind:open={confirmOpen}
  title={confirmTitle(action)}
  onconfirm={apply}
>
  {#if action === "enable"}
    {m.s2_tunnel_confirm_enable_body()}
  {:else if action === "reset"}
    {m.s2_tunnel_confirm_reset_body()}
  {:else if action === "save"}
    {m.s2_tunnel_confirm_save_body()}
  {:else}
    {m.s2_tunnel_confirm_disable_body()}
  {/if}
  {#if publicPage && action !== "enable"}{m.s2_tunnel_confirm_disconnect_warn()}{/if}
</ConfirmDialog>
