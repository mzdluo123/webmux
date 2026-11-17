<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import * as Card from "$lib/components/ui/card";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import MessagingMessages from "./MessagingMessages.svelte";
  import { featureAvailable } from "../../platform/feature-host.svelte";

  const available = $derived(featureAvailable("messaging.messages"));
</script>

<Card.Root flat id="messaging" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>{m.messaging_title()}</Card.Title>
    <Card.Description>{m.messaging_desc()}</Card.Description>
  </Card.Header>
  <Card.Content class="space-y-4">
    {#if available}
      <MessagingMessages />
    {:else}
      <InlineNotice tone="info">{m.messaging_config_hint()}</InlineNotice>
    {/if}
  </Card.Content>
</Card.Root>
