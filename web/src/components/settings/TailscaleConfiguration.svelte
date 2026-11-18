<script lang="ts">
  import { usePanelMotion } from "../../hooks/shared/dock-motion.svelte";
  import { untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import * as Tabs from "$lib/components/ui/tabs";
  import {
    TailscaleFunnelPorts,
    type TailscaleSettingsUpdate,
    type TailscaleStatus,
  } from "../../contracts/schemas";

  let {
    current,
    disabled,
    onsave,
  }: {
    current: TailscaleStatus;
    disabled: boolean;
    onsave: (draft: TailscaleSettingsUpdate) => void;
  } = $props();
  let mode = $state(untrack(() => current.mode));
  let panels = $state<HTMLDivElement | null>(null);
  usePanelMotion(
    () => panels,
    () => mode,
  );
  let provider = $state(untrack(() => current.configuredProvider));
  // Serve takes any port; empty or 0 selects 443, 8443 or 10000 automatically.
  let servePort = $state(
    untrack(() => (current.port === 0 ? "" : String(current.port))),
  );
  let funnelPort = $state(
    untrack(() =>
      (TailscaleFunnelPorts as readonly number[]).includes(current.port)
        ? String(current.port)
        : "0",
    ),
  );
  let attempted = $state(false);
  const parsedServePort = $derived(
    servePort.trim() === "" ? 0 : Number(servePort),
  );
  const portError = $derived(
    mode === "serve" &&
      !(
        Number.isInteger(parsedServePort) &&
        parsedServePort >= 0 &&
        parsedServePort <= 65535
      )
      ? m.s2_ts_err_port()
      : "",
  );
  const port = $derived(
    mode === "serve" ? parsedServePort : mode === "funnel" ? +funnelPort : 0,
  );

  function submit(event: SubmitEvent) {
    event.preventDefault();
    attempted = true;
    if (disabled || portError) return;
    onsave({ mode, port, provider });
  }
</script>

<form onsubmit={submit} class="space-y-4" aria-label={m.s2_ts_form_aria()}>
  <fieldset {disabled} class="min-w-0 space-y-4">
    <Tabs.Root bind:value={mode}>
      <Tabs.List variant="line" size="form" aria-label={m.s2_ts_mode_aria()}>
        <Tabs.Trigger value="off">{m.s2_launch_attn_mode_off()}</Tabs.Trigger>
        <Tabs.Trigger value="serve">Serve</Tabs.Trigger>
        <Tabs.Trigger value="funnel">Funnel</Tabs.Trigger>
      </Tabs.List>
      <div bind:this={panels}>
        <Tabs.Content value="off" class="pt-3">
          <p class="text-sm text-muted-foreground">
            {m.s2_ts_off_desc()}
          </p>
        </Tabs.Content>
        <Tabs.Content value="serve" class="space-y-4 pt-3">
          <p class="text-sm text-muted-foreground">
            {m.s2_ts_serve_desc()}
          </p>
          <div class="grid gap-2">
            <Label for="tailscale-serve-port">{m.s2_ts_serve_port()}</Label>
            <Input
              id="tailscale-serve-port"
              inputmode="numeric"
              bind:value={servePort}
              placeholder={m.s2_ts_serve_port_placeholder()}
              autocomplete="off"
              aria-invalid={attempted && !!portError}
              aria-describedby="tailscale-serve-port-help"
            />
            <p
              id="tailscale-serve-port-help"
              class="text-xs text-muted-foreground"
            >
              {attempted && portError ? portError : m.s2_ts_serve_port_help()}
            </p>
          </div>
        </Tabs.Content>
        <Tabs.Content value="funnel" class="space-y-4 pt-3">
          <p class="text-sm text-warning">
            {m.s2_ts_funnel_warn()}
          </p>
          <div class="grid gap-2">
            <Label for="tailscale-funnel-port">{m.s2_ts_funnel_port()}</Label>
            <NativeSelect.Root
              id="tailscale-funnel-port"
              bind:value={funnelPort}
            >
              <NativeSelect.Option value="0">{m.s2_auto()}</NativeSelect.Option>
              {#each TailscaleFunnelPorts as option (option)}
                <NativeSelect.Option value={String(option)}
                  >{option}</NativeSelect.Option
                >
              {/each}
            </NativeSelect.Root>
          </div>
        </Tabs.Content>
      </div>
    </Tabs.Root>
    <div class="grid gap-2">
      <Label for="tailscale-provider">Provider</Label>
      <NativeSelect.Root
        id="tailscale-provider"
        bind:value={provider}
        aria-describedby="tailscale-provider-help"
      >
        <NativeSelect.Option value="auto">{m.s2_auto()}</NativeSelect.Option>
        <NativeSelect.Option value="linux"
          >{m.s2_ts_provider_linux()}</NativeSelect.Option
        >
        {#if current.wsl}
          <NativeSelect.Option value="windows"
            >{m.s2_ts_provider_windows()}</NativeSelect.Option
          >
        {/if}
      </NativeSelect.Root>
      <p id="tailscale-provider-help" class="text-xs text-muted-foreground">
        {m.s2_ts_provider_help()}
      </p>
    </div>
    <Button type="submit">{m.s2_save_and_apply()}</Button>
  </fieldset>
</form>
