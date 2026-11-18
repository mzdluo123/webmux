<script lang="ts">
  import { onMount } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconCopy from "~icons/tabler/copy";
  import IconRefresh from "~icons/tabler/refresh";
  import ConfirmDialog from "../components/shared/ConfirmDialog.svelte";
  import { Button } from "$lib/components/ui/button";
  import * as Card from "$lib/components/ui/card";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import SettingsPageHeader from "../components/settings/SettingsPageHeader.svelte";
  import TailscaleConfiguration from "../components/settings/TailscaleConfiguration.svelte";
  import TunnelStateBadge from "../components/settings/TunnelStateBadge.svelte";
  import { stored } from "../platform/api";
  import { setBrowserChrome } from "../platform/browser-chrome";
  import { useCopied } from "../hooks/shared/copied.svelte";
  import { useTailscaleSettings } from "../hooks/settings/tailscale-settings.svelte";
  import type {
    TailscaleSettingsUpdate,
    TailscaleStatus,
  } from "../contracts/schemas";

  const tailscale = useTailscaleSettings();
  const ui = tailscale.state;
  const status = $derived(ui.view);
  const canReset = $derived(
    !ui.busy && !!status?.storageAvailable && status.source === "web",
  );
  const copyFeedback = useCopied({ duration: 2500, expireFailure: true });
  const copyNotice = $derived(
    copyFeedback.copied
      ? m.s1_copied()
      : copyFeedback.failed
        ? m.s1_copy_failed_clipboard()
        : "",
  );
  let confirmOpen = $state(false);
  let action = $state<"funnel" | "save" | "reset">();
  let pending = $state<TailscaleSettingsUpdate>();
  $effect(() => {
    if (!confirmOpen) {
      pending = undefined;
      action = undefined;
    }
  });

  const providers = $derived({
    linux: "Linux / macOS",
    "wsl-native": m.s1_ts_provider_wsl_native(),
    windows: m.s1_ts_provider_windows(),
    none: m.s1_ts_provider_none(),
  });
  const backend = $derived({
    running: m.s1_ts_backend_running(),
    stopped: m.s1_ts_backend_stopped(),
    "needs-login": m.s1_ts_backend_needs_login(),
    unavailable: m.s1_ts_backend_unavailable(),
    unknown: m.s1_ts_backend_unknown(),
  });
  const funnel = $derived({
    available: m.s1_ts_funnel_available(),
    unavailable: m.s1_ts_funnel_unavailable(),
    unknown: m.s1_git_status_unknown(),
  });
  const modeText = $derived({
    off: m.s1_ts_mode_off(),
    serve: m.s1_ts_mode_serve(),
    funnel: m.s1_ts_mode_funnel(),
  });
  const titles = $derived({
    funnel: m.s1_ts_confirm_funnel_title(),
    save: m.s1_ts_confirm_replace_title(),
    reset: m.s1_ts_confirm_reset_title(),
  });
  const entryAuthority = (entry: TailscaleStatus["entries"][number]) =>
    entry.port === 443 ? entry.host : `${entry.host}:${entry.port}`;

  async function copyURL() {
    if (status?.publicUrl) await copyFeedback.copy(status.publicUrl);
  }
  async function copyTokenLink() {
    if (!status?.publicUrl) return;
    const token = stored("token");
    await copyFeedback.copy(
      token
        ? status.publicUrl + "#token=" + encodeURIComponent(token)
        : status.publicUrl,
    );
  }
  function confirm(next: typeof action, draft?: TailscaleSettingsUpdate) {
    action = next;
    pending = draft;
    confirmOpen = true;
  }
  // Funnel needs an explicit risk confirmation. Changing the entry that
  // serves this very page would cut the connection, so confirm that too.
  function save(draft: TailscaleSettingsUpdate) {
    if (draft.mode === "funnel" && status?.mode !== "funnel")
      confirm("funnel", draft);
    else if (status?.publicUrl === location.origin) confirm("save", draft);
    else void tailscale.save(draft);
  }
  async function apply() {
    const selected = action;
    const draft = pending;
    confirmOpen = false;
    if (selected === "reset") await tailscale.reset();
    else if (draft)
      await tailscale.save({ ...draft, confirmFunnel: selected === "funnel" });
  }
  onMount(() => setBrowserChrome("WebMux · Tailscale", false));
</script>

<div class="safe-area-page min-h-dvh">
  <SettingsPageHeader />
  <main class="mx-auto max-w-3xl space-y-6 px-4 py-8 sm:px-8 sm:py-10">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight sm:text-3xl">
        Tailscale
      </h1>
      <p class="mt-2 text-sm text-muted-foreground">
        {m.s1_ts_intro()}
      </p>
    </div>

    <div class="flex flex-wrap items-center gap-3">
      <Button
        variant="outline"
        disabled={ui.loading || ui.busy}
        onclick={() => tailscale.load(true)}
        ><IconRefresh aria-hidden="true" />{m.s1_ts_refresh_check()}</Button
      >
      {#if ui.loading}<span role="status" class="text-sm text-muted-foreground"
          >{m.s1_ts_loading()}</span
        >{/if}
      {#if ui.notice}<span role="status" class="text-sm text-muted-foreground"
          >{ui.notice}</span
        >{/if}
    </div>
    {#if ui.error}
      <InlineNotice tone="danger"
        >{m.s1_ts_error_isolated({ error: ui.error })}</InlineNotice
      >
    {/if}

    {#if status}
      {#if status.mode === "funnel"}
        <InlineNotice tone="warning">
          {m.s1_ts_funnel_warning_before()}<strong
            >{m.s1_ts_funnel_warning_strong()}</strong
          >{m.s1_ts_funnel_warning_after()}
        </InlineNotice>
      {/if}
      {#if status.blockedReason || status.error}
        <InlineNotice tone={status.state === "failed" ? "danger" : "warning"}>
          {status.blockedReason || status.error}
        </InlineNotice>
      {/if}

      <Card.Root>
        <Card.Header>
          <Card.Title>{m.s1_ts_detection_title()}</Card.Title>
          <Card.Description>{m.s1_ts_detection_desc()}</Card.Description>
        </Card.Header>
        <Card.Content class="space-y-4">
          <div class="flex flex-wrap items-center gap-2" role="status">
            <TunnelStateBadge state={status.state} />
            <span class="text-sm">{providers[status.provider]}</span>
            {#if status.dnsName}<span
                class="font-mono text-xs text-muted-foreground"
                >{status.dnsName}</span
              >{/if}
          </div>
          <dl class="grid gap-3 text-sm sm:grid-cols-2">
            <div>
              <dt class="text-muted-foreground">{m.s1_ts_backend_state()}</dt>
              <dd>{backend[status.backendState]}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{m.s1_ts_https_cert()}</dt>
              <dd>
                {status.httpsReady ? m.s1_ts_ready() : m.s1_ts_not_ready()}
              </dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{m.s1_ts_funnel_attr()}</dt>
              <dd>{funnel[status.funnelAvailable]}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">
                {m.s1_ts_configured_provider()}
              </dt>
              <dd>{status.configuredProvider}</dd>
            </div>
          </dl>
          {#if !status.httpsReady}
            <p class="text-sm text-muted-foreground">
              {m.s1_ts_enable_https_hint()}
            </p>
          {/if}
        </Card.Content>
      </Card.Root>

      <Card.Root>
        <Card.Header>
          <Card.Title>{m.s1_ts_managed_entry_title()}</Card.Title>
          <Card.Description>{m.s1_ts_managed_entry_desc()}</Card.Description>
        </Card.Header>
        <Card.Content class="space-y-4">
          <dl class="grid gap-3 text-sm sm:grid-cols-2">
            <div>
              <dt class="text-muted-foreground">{m.s1_mode()}</dt>
              <dd>{modeText[status.mode]}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{m.s1_port()}</dt>
              <dd>{status.port === 0 ? m.s1_auto() : status.port}</dd>
            </div>
          </dl>
          {#if status.publicUrl}
            <div class="space-y-2">
              <p class="font-mono text-sm wrap-anywhere">{status.publicUrl}</p>
              <div class="flex flex-wrap gap-2">
                <Button
                  variant="outline"
                  onclick={() => copyURL()}
                  aria-label={m.s1_ts_copy_url_aria()}
                  ><IconCopy aria-hidden="true" />{m.s1_ts_copy_url()}</Button
                >
                <Button
                  variant="outline"
                  onclick={() => copyTokenLink()}
                  aria-label={m.s1_ts_copy_token_link_aria()}
                  ><IconCopy
                    aria-hidden="true"
                  />{m.s1_ts_copy_token_link()}</Button
                >
              </div>
              {#if copyNotice}<span
                  role="status"
                  class="text-xs text-muted-foreground">{copyNotice}</span
                >{/if}
            </div>
          {:else}
            <p class="text-sm text-muted-foreground">
              {status.mode === "off"
                ? m.s1_ts_no_ready_url_off()
                : m.s1_ts_no_ready_url_fix()}
            </p>
          {/if}
          <p class="text-sm text-muted-foreground">
            {status.source === "web"
              ? m.s1_ts_source_web()
              : m.s1_ts_source_config()}
          </p>
          {#if !status.storageAvailable}
            <InlineNotice tone="danger"
              >{m.s1_ts_web_storage_unavailable({
                error: status.storageError ?? "",
              })}</InlineNotice
            >
          {:else if !status.canEnable}
            <InlineNotice tone="warning"
              >{m.s1_ts_cannot_enable({
                reason: status.enableBlockedReason ?? "",
              })}</InlineNotice
            >
          {/if}
          {#key ui.editVersion}
            <TailscaleConfiguration
              current={status}
              disabled={ui.busy || !status.storageAvailable}
              onsave={save}
            />
          {/key}
          <Button
            variant="outline"
            disabled={!canReset}
            onclick={() => confirm("reset")}
            >{m.s1_ts_restore_config_settings()}</Button
          >
        </Card.Content>
      </Card.Root>

      {#if status.wsl}
        <Card.Root>
          <Card.Header>
            <Card.Title>{m.s1_ts_wsl_title()}</Card.Title>
            <Card.Description>{m.s1_ts_wsl_desc()}</Card.Description>
          </Card.Header>
          <Card.Content class="space-y-3">
            <p>
              {m.s1_ts_wsl_net_mode()}<strong
                >{status.wsl.networkingMode}</strong
              >{m.s1_ts_wsl_win_tailscale({
                state: status.wsl.windowsDetected
                  ? status.wsl.windowsRunning
                    ? m.s1_ts_wsl_win_running()
                    : m.s1_ts_wsl_win_stopped()
                  : m.s1_ts_wsl_win_missing(),
              })}{#if status.wsl.windowsDnsName}{m.s1_ts_wsl_dns_suffix({
                  dns: status.wsl.windowsDnsName,
                })}{/if}
            </p>
            {#if status.configuredProvider === "windows" && status.mode !== "off" && status.wsl.networkingMode !== "mirrored"}
              <InlineNotice tone="warning">
                {m.s1_ts_wsl_mirrored_warning()}
              </InlineNotice>
            {/if}
            {#if status.configuredProvider !== "windows" && status.wsl.windowsDetected}
              <p class="text-muted-foreground">
                {m.s1_ts_wsl_select_windows_hint()}
              </p>
            {/if}
            {#if status.configuredProvider === "windows"}
              <p class="text-muted-foreground">
                {m.s1_ts_wsl_operator_hint()}
              </p>
            {/if}
          </Card.Content>
        </Card.Root>
      {/if}

      <Card.Root>
        <Card.Header>
          <Card.Title>{m.s1_ts_existing_entries_title()}</Card.Title>
          <Card.Description>{m.s1_ts_existing_entries_desc()}</Card.Description>
        </Card.Header>
        <Card.Content>
          {#if status.entries.length === 0}
            <p class="text-sm text-muted-foreground">
              {m.s1_ts_no_existing_entries()}
            </p>
          {:else}
            <ul
              class="space-y-2 text-sm"
              aria-label={m.s1_ts_existing_entries_aria()}
            >
              {#each status.entries as entry (entryAuthority(entry))}
                <li
                  class="flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2"
                >
                  <span class="font-mono wrap-anywhere"
                    >{entryAuthority(entry)}</span
                  >
                  <span class="text-muted-foreground"
                    >{entry.public
                      ? m.s1_ts_entry_public()
                      : m.s1_ts_entry_tailnet()}</span
                  >
                </li>
              {/each}
            </ul>
          {/if}
        </Card.Content>
      </Card.Root>
    {/if}
  </main>
</div>

<ConfirmDialog
  bind:open={confirmOpen}
  title={action ? titles[action] : m.s1_confirm_action()}
  onconfirm={apply}
>
  {#if action === "funnel"}
    {m.s1_ts_confirm_funnel_body()}
  {:else if action === "reset"}
    {m.s1_ts_confirm_reset_body()}
  {:else}
    {m.s1_ts_confirm_replace_body()}
  {/if}
</ConfirmDialog>
