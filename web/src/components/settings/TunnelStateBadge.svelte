<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { Badge, type BadgeVariant } from "$lib/components/ui/badge";
  import type { TunnelInfo } from "../../contracts/schemas";

  let { state }: { state: TunnelInfo["state"] } = $props();

  function stateLabel(value: TunnelInfo["state"]): string {
    switch (value) {
      case "off":
        return m.s2_state_off();
      case "starting":
        return m.s2_state_starting();
      case "ready":
        return m.s2_state_ready();
      case "failed":
        return m.s2_state_failed();
    }
  }

  const variants: Record<TunnelInfo["state"], BadgeVariant> = {
    off: "outline",
    starting: "warning",
    ready: "success",
    failed: "destructive",
  };
</script>

<Badge variant={variants[state]}>{stateLabel(state)}</Badge>
