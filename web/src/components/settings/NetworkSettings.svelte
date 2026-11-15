<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import * as Card from "$lib/components/ui/card";
  import { Label } from "$lib/components/ui/label";
  import { Separator } from "$lib/components/ui/separator";
  import { Switch } from "$lib/components/ui/switch";
  import { getPluginContributions } from "../../plugins/context";
  import { setWsCompression, wsCompression } from "../../platform/ws";

  let compression = $state(wsCompression());
  const providers = getPluginContributions().network;
</script>

<Card.Root flat id="network" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.s2_network_title()}</Card.Title>
    <Card.Description>{m.s2_network_desc()}</Card.Description>
  </Card.Header>
  <Card.Content class="space-y-6">
    <div class="space-y-2">
      <div class="flex items-center justify-between gap-4">
        <Label for="ws-compression">{m.s2_ws_compression()}</Label>
        <Switch
          id="ws-compression"
          checked={compression}
          aria-describedby="ws-compression-help"
          onCheckedChange={(checked) => {
            setWsCompression(checked);
            compression = wsCompression();
          }}
        />
      </div>
      <p id="ws-compression-help" class="text-xs text-muted-foreground">
        {m.s2_ws_compression_help()}
      </p>
    </div>
    {#if providers.ForwardSettings}<providers.ForwardSettings />{/if}
    {#if providers.P2PSettings}
      <Separator />
      <providers.P2PSettings />
    {/if}
  </Card.Content>
</Card.Root>
