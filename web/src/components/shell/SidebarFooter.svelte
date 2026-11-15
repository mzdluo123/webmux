<script lang="ts">
  import { onMount, untrack } from "svelte";
  import IconMoon from "~icons/tabler/moon";
  import IconSettings from "~icons/tabler/settings";
  import IconSun from "~icons/tabler/sun";
  import { api } from "../../platform/api";
  import { Button } from "$lib/components/ui/button";
  import * as Popover from "$lib/components/ui/popover";
  import {
    BUSY_LOAD_RATIO,
    formatBytes,
    hostStrain,
    SLOW_LATENCY_MS,
  } from "../../domains/sessions/host-health";
  import { featureContributions } from "../../features/registry.svelte";
  import { HealthSchema, type Health } from "../../contracts/schemas";
  import { toggleTheme } from "../../hooks/settings/theme";
  import type { Connection } from "../../platform/ws";
  import { m } from "$lib/paraglide/messages.js";
  const p2pState = $derived(
    featureContributions.entries.p2p?.p2pState ?? {
      mode: "ws",
      p2pRttMs: null,
      candidateType: "",
      ipFamily: "",
    },
  );
  const forward = $derived(featureContributions.entries.forward);

  // The sidebar's global footer, shared by both modes: the connection and
  // host, port forwarding, settings and the theme. A healthy connection is
  // only the host and a dot; reconnecting says so in words.
  let {
    connection,
    hostname,
    setupSettled,
    rail = false,
    onforward,
  }: {
    connection: Connection;
    hostname: string;
    setupSettled: boolean;
    /** The collapsed rail stacks the buttons and drops the host text. */
    rail?: boolean;
    onforward: () => void;
  } = $props();

  let dark = $state(document.documentElement.classList.contains("dark"));
  let detailsOpen = $state(false);
  let latency = $state<number | null>(null);
  let measuring = $state(false);
  let health = $state<Health | null>(null);
  const effectiveLatency = $derived(
    p2pState.mode === "p2p" && p2pState.p2pRttMs !== null
      ? p2pState.p2pRttMs
      : latency,
  );
  const strain = $derived(
    connection === "connected" ? hostStrain(effectiveLatency, health) : "",
  );
  const statusLabel = $derived(
    connection === "connected"
      ? hostname
        ? strain
          ? m.s1_host_online_strained({ hostname, strain })
          : m.s1_host_online({ hostname })
        : strain
          ? m.s1_online_strained({ strain })
          : m.s1_online()
      : connection === "reconnecting"
        ? m.s1_disconnected_reconnecting()
        : m.s1_connecting(),
  );
  const connectionLabel = $derived(
    connection === "connected"
      ? m.s1_connected()
      : connection === "reconnecting"
        ? m.s1_reconnecting()
        : m.s1_connecting_short(),
  );
  const p2pDetail = $derived(
    p2pState.candidateType
      ? `${p2pState.candidateType}${p2pState.ipFamily ? `/${p2pState.ipFamily}` : ""}`
      : "",
  );
  // A probe that never answers (a stalled path, e.g. over a VPN after a
  // network change) must end, or `measuring` would block every later one.
  const PROBE_TIMEOUT_MS = 10_000;
  async function measureLatency() {
    if (measuring || connection !== "connected") return;
    measuring = true;
    const started = performance.now();
    try {
      const next = await api(
        "/health",
        HealthSchema,
        "GET",
        undefined,
        AbortSignal.timeout(PROBE_TIMEOUT_MS),
      );
      latency = Math.max(0, Math.round(performance.now() - started));
      health = next;
    } catch {
      latency = null;
      health = null;
    } finally {
      measuring = false;
    }
  }
  function detailsChanged(open: boolean) {
    detailsOpen = open;
    if (open) void measureLatency();
  }
  // The dot turns yellow on a slow or loaded host, so it is measured while
  // connected, not only when the details are open.
  $effect(() => {
    if (connection !== "connected") return;
    // measureLatency reads and writes state; only `connection` may re-run
    // this, or each measurement would start the next.
    untrack(() => void measureLatency());
    const timer = setInterval(() => void measureLatency(), 15_000);
    return () => clearInterval(timer);
  });
  onMount(() => {
    const theme = new MutationObserver(() => {
      dark = document.documentElement.classList.contains("dark");
    });
    theme.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });
    return () => theme.disconnect();
  });
</script>

<div
  class={rail
    ? "flex flex-col items-center gap-1 border-t py-2"
    : "flex items-center gap-1 border-t px-3 py-1.5"}
>
  <Popover.Root open={detailsOpen} onOpenChange={detailsChanged}>
    <Popover.Trigger
      type="button"
      aria-label={m.s1_view_host_status({ status: statusLabel })}
      title={statusLabel}
      class={rail
        ? "flex size-8 cursor-pointer items-center justify-center rounded-sm hover:bg-muted/60"
        : "flex h-5 min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-sm text-xs text-muted-foreground hover:bg-muted/60"}
    >
      <!-- min-h-4 is the text line: the hostname arriving later must not move the dot. -->
      <span role="status" class="flex min-h-4 min-w-0 items-center gap-2">
        <span
          class={`status-dot shrink-0 ${connection === "connected" && !strain ? "status-running" : "status-waiting"}`}
        ></span>{#if rail}<span class="sr-only">{statusLabel}</span
          >{:else if connection === "reconnecting"}<span
            class="truncate text-warning"
            >{m.s1_disconnected_reconnecting()}</span
          >{:else if connection === "connected" && setupSettled}<span
            class="truncate">{hostname || m.s1_online()}</span
          >{:else}<span class="sr-only">{statusLabel}</span>{/if}
      </span>
    </Popover.Trigger>
    <Popover.Content
      side="top"
      align="start"
      role="dialog"
      aria-label={m.s1_host_status_details()}
      class="w-[min(18rem,calc(100vw-2rem))] gap-3"
    >
      <Popover.Header>
        <Popover.Title>{m.s1_host_status()}</Popover.Title>
        <Popover.Description
          >{m.s1_host_status_description()}</Popover.Description
        >
      </Popover.Header>
      <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-xs">
        <dt class="text-muted-foreground">{m.s1_hostname()}</dt>
        <dd class="min-w-0 truncate font-medium" title={hostname}>
          {hostname || m.s1_hostname_unavailable()}
        </dd>
        <dt class="text-muted-foreground">{m.s1_connection_status()}</dt>
        <dd
          class={connection === "connected" ? "text-success" : "text-warning"}
        >
          {connectionLabel}
        </dd>
        <dt class="text-muted-foreground">{m.s1_transport_path()}</dt>
        <dd class={p2pState.mode === "p2p" ? "text-success" : ""}>
          {#if p2pState.mode === "p2p"}{p2pDetail
              ? m.s1_transport_p2p_detail({ detail: p2pDetail })
              : m.s1_transport_p2p()}{:else}WebSocket{/if}
        </dd>
        <dt class="text-muted-foreground">{m.s1_network_latency()}</dt>
        <dd
          class={effectiveLatency !== null && effectiveLatency > SLOW_LATENCY_MS
            ? "text-warning"
            : ""}
        >
          {#if connection !== "connected"}{m.s1_not_available()}{:else if measuring && effectiveLatency === null}{m.s1_measuring()}{:else if effectiveLatency !== null}{effectiveLatency}
            ms{:else}{m.s1_not_available()}{/if}
        </dd>
        {#if health?.load}
          <dt class="text-muted-foreground">{m.s1_system_load()}</dt>
          <dd
            class={health.load.one > health.load.cpus * BUSY_LOAD_RATIO
              ? "text-warning"
              : ""}
            title={m.s1_system_load_title({ cpus: health.load.cpus })}
          >
            {m.s1_system_load_value({
              one: health.load.one.toFixed(2),
              five: health.load.five.toFixed(2),
              fifteen: health.load.fifteen.toFixed(2),
              cpus: health.load.cpus,
            })}
          </dd>
        {/if}
        {#if health?.memory}
          <dt class="text-muted-foreground">{m.s1_memory()}</dt>
          <dd>
            {m.s1_memory_value({
              available: formatBytes(health.memory.available),
              total: formatBytes(health.memory.total),
            })}
          </dd>
        {/if}
      </dl>
    </Popover.Content>
  </Popover.Root>
  {#if forward}<forward.Button onclick={onforward} />{/if}
  <Button
    href="/settings"
    variant="ghost"
    size="icon-touch"
    aria-label={m.s1_settings()}
    title={m.s1_settings()}><IconSettings aria-hidden="true" /></Button
  >
  <Button
    variant="ghost"
    size="icon-touch"
    class="shrink-0"
    aria-label={dark ? m.s1_switch_light_theme() : m.s1_switch_dark_theme()}
    title={dark ? m.s1_switch_light_theme() : m.s1_switch_dark_theme()}
    onclick={toggleTheme}
    >{#if dark}<IconSun aria-hidden="true" />{:else}<IconMoon
        aria-hidden="true"
      />{/if}</Button
  >
</div>
