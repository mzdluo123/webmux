<script lang="ts">
  import { untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconEye from "~icons/tabler/eye";
  import IconEyeOff from "~icons/tabler/eye-off";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Tabs from "$lib/components/ui/tabs";
  import type {
    TunnelSettings,
    TunnelSettingsUpdate,
  } from "../../contracts/schemas";

  let {
    current,
    disabled,
    onsave,
  }: {
    current: TunnelSettings;
    disabled: boolean;
    onsave: (draft: TunnelSettingsUpdate) => void;
  } = $props();
  let mode = $state(untrack(() => current.mode));
  let hostname = $state(untrack(() => current.hostname));
  let token = $state("");
  let revealed = $state(false);
  let attempted = $state(false);
  const normalizedHost = $derived(hostname.toLowerCase().replace(/\.+$/, ""));
  const hostnameValid = $derived(
    normalizedHost.length > 0 &&
      normalizedHost.length <= 253 &&
      normalizedHost
        .split(".")
        .every(
          (label) =>
            label.length <= 63 &&
            /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(label),
        ),
  );
  const hostnameError = $derived(
    mode === "named" && !hostnameValid ? m.s2_tunnel_err_hostname() : "",
  );
  const tokenError = $derived(
    mode === "named" && !token && !current.tokenConfigured
      ? m.s2_tunnel_err_token()
      : "",
  );

  function submit(event: SubmitEvent) {
    event.preventDefault();
    attempted = true;
    if (disabled || hostnameError || tokenError) return;
    onsave({
      mode,
      hostname: mode === "named" ? normalizedHost : "",
      ...(mode === "named" && token ? { token } : {}),
    });
  }
</script>

<form
  onsubmit={submit}
  class="space-y-4"
  aria-label={m.s2_tunnel_config_title()}
>
  <fieldset {disabled} class="min-w-0 space-y-4">
    <Tabs.Root bind:value={mode}>
      <Tabs.List variant="line" size="form">
        <Tabs.Trigger value="quick">Quick Tunnel</Tabs.Trigger>
        <Tabs.Trigger value="named">Named Tunnel</Tabs.Trigger>
      </Tabs.List>
      <Tabs.Content value="quick" class="space-y-2 pt-3">
        <p class="text-sm text-muted-foreground">
          {m.s2_tunnel_quick_desc()}
        </p>
        {#if current.tokenConfigured}<p class="text-sm text-warning">
            {m.s2_tunnel_quick_warn_clear()}
          </p>{/if}
      </Tabs.Content>
      <Tabs.Content value="named" class="space-y-4 pt-3">
        <p class="text-sm text-muted-foreground">
          {m.s2_tunnel_named_desc()}
        </p>
        <div class="grid gap-2">
          <Label for="tunnel-hostname">{m.s2_tunnel_hostname_label()}</Label>
          <Input
            id="tunnel-hostname"
            bind:value={hostname}
            placeholder="term.example.com"
            autocomplete="off"
            autocapitalize="off"
            spellcheck={false}
            aria-invalid={!!hostnameError && (attempted || !!hostname)}
            aria-describedby="tunnel-hostname-help"
          />
          <p id="tunnel-hostname-help" class="text-xs text-muted-foreground">
            {(attempted || hostname) && hostnameError
              ? hostnameError
              : m.s2_tunnel_hostname_help()}
          </p>
        </div>
        <div class="grid gap-2">
          <Label for="tunnel-token">{m.s2_tunnel_token_label()}</Label>
          <div class="flex items-center gap-2">
            <Input
              id="tunnel-token"
              type={revealed ? "text" : "password"}
              bind:value={token}
              autocomplete="new-password"
              autocapitalize="off"
              spellcheck={false}
              placeholder={current.tokenConfigured
                ? m.s2_tunnel_token_ph_configured()
                : m.s2_tunnel_token_ph_empty()}
              aria-invalid={attempted && !!tokenError}
              aria-describedby="tunnel-token-help"
            />
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label={revealed
                ? m.s2_tunnel_token_hide()
                : m.s2_tunnel_token_show()}
              title={revealed
                ? m.s2_tunnel_token_hide()
                : m.s2_tunnel_token_show()}
              onclick={() => (revealed = !revealed)}
            >
              {#if revealed}<IconEyeOff aria-hidden="true" />{:else}<IconEye
                  aria-hidden="true"
                />{/if}
            </Button>
          </div>
          <p id="tunnel-token-help" class="text-xs text-muted-foreground">
            {attempted && tokenError ? tokenError : m.s2_tunnel_token_help()}
          </p>
        </div>
      </Tabs.Content>
    </Tabs.Root>
    <Button type="submit">{m.s2_save_and_apply()}</Button>
  </fieldset>
</form>
