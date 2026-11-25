<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { locale } from "$lib/locale";
  import type { RelayStatus } from "../../contracts/relay-schemas";

  let { status }: { status: RelayStatus } = $props();
  const managementState = $derived({
    off: m.relay_state_off(),
    connecting: m.s1_connecting_short(),
    ready: m.relay_not_ready(),
    failed: m.relay_state_failed(),
  });
  const probeOutcome = $derived({
    unknown: m.relay_unknown(),
    reachable: m.relay_reach_reachable(),
    unavailable: m.relay_reach_unavailable(),
  });
  const checkedAt = $derived(status.dataCheckedAt);
  const probeDateFormat = $derived(
    new Intl.DateTimeFormat(locale.current, {
      dateStyle: "medium",
      timeStyle: "medium",
    }),
  );
  const probeTime = $derived(
    checkedAt ? probeDateFormat.format(Date.parse(checkedAt)) : "",
  );
</script>

<dl class="grid min-w-0 grid-cols-2 gap-x-4 gap-y-2 text-sm">
  <dt class="text-muted-foreground">{m.relay_control_readiness()}</dt>
  <dd aria-live="polite">
    {status.controlReady
      ? m.relay_state_ready()
      : managementState[status.state]}
  </dd>
  <dt class="text-muted-foreground">{m.relay_origin_probe()}</dt>
  <dd aria-live="polite">
    {probeOutcome[checkedAt ? status.dataReachability : "unknown"]}
  </dd>
  <dt class="text-xs text-muted-foreground">{m.relay_probe_last_checked()}</dt>
  <dd class="text-xs">
    {#if checkedAt}
      <time datetime={checkedAt}>{probeTime}</time>
    {:else}
      {m.relay_probe_no_sample()}
    {/if}
  </dd>
</dl>
