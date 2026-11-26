<script lang="ts">
  import { onDestroy, type ComponentProps } from "svelte";
  import RelaySettings from "../src/components/settings/RelaySettings.svelte";
  import type {
    RelayDataProtocol,
    RelayStatus,
  } from "../src/contracts/relay-schemas";

  let { view, ...actions }: ComponentProps<typeof RelaySettings> = $props();
  let result = $state<RelayStatus>();
  let testing = $state<RelayDataProtocol>();
  let timer: ReturnType<typeof setTimeout> | undefined;
  const current = $derived(result ?? view.status);

  function test(protocol: RelayDataProtocol) {
    const status = current;
    const selection = status?.transportSelection;
    if (!status || !selection || testing) return;
    testing = protocol;
    const started = performance.now();
    timer = setTimeout(() => {
      const completedAt = new Date().toISOString();
      result = {
        ...status,
        state: "ready",
        dataReady: true,
        dataReachability: "reachable",
        dataCheckedAt: completedAt,
        publicUrl: status.boundPublicUrl,
        transportSelection: {
          ...selection,
          preferred: protocol,
          active: protocol,
          lastAttempt: {
            nodeId: status.nodeId,
            protocol,
            stage: "commit",
            error: "",
            durationMs: Math.round(performance.now() - started),
            completedAt,
          },
        },
      };
      testing = undefined;
    }, 1800);
  }

  onDestroy(() => clearTimeout(timer));
</script>

<RelaySettings
  {...actions}
  view={{
    ...view,
    status: current,
    busy: Boolean(testing),
    testingTransport: testing,
  }}
  ontransport={test}
/>
