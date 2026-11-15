<script lang="ts">
  import RelaySettings from "./RelaySettings.svelte";
  import { useRelaySettings } from "../../hooks/settings/relay-settings.svelte";

  const controller = useRelaySettings();
</script>

<RelaySettings
  view={controller.ui}
  onrefresh={() => void controller.refresh()}
  onpair={(rebind) => void controller.pair(rebind)}
  oncancel={() => void controller.cancelPairing()}
  onenable={() => void controller.setEnabled(true)}
  ondisconnect={() => void controller.disconnect()}
  onreconnect={() => void controller.reconnect()}
  ondelete={() => void controller.deleteBinding()}
/>
