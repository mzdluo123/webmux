<script lang="ts">
  import IconRefresh from "~icons/tabler/refresh";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import * as Item from "$lib/components/ui/item";
  import * as m from "$lib/paraglide/messages.js";
  import { restartMethod } from "../../platform/pending-update";
  import type { PendingUpdate } from "../../contracts/schemas";

  // Without ondismiss the notice cannot be closed (the settings page).
  let {
    update,
    ondismiss,
    class: className,
  }: {
    update: PendingUpdate;
    ondismiss?: () => void;
    class?: string;
  } = $props();
  const method = $derived(restartMethod(update.launcher));
</script>

<Item.Root
  variant="outline"
  role="status"
  aria-label={m.s1_pending_update_aria()}
  class={className}
>
  <Item.Media variant="icon"><IconRefresh aria-hidden="true" /></Item.Media>
  <Item.Content>
    <Item.Title
      >{m.s1_pending_update_title({
        version: update.installedVersion,
      })}</Item.Title
    >
    <Item.Description>{m.s1_pending_update_ends_sessions()}</Item.Description>
    <Item.Description>
      {m.s1_pending_update_restart_how()}{#if "command" in method}<code
          >{method.command}</code
        >{:else}{method.text}{/if}
    </Item.Description>
  </Item.Content>
  {#if ondismiss}
    <Item.Actions>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={m.s1_dismiss_update_notice()}
        title={m.s1_close()}
        onclick={ondismiss}><IconX aria-hidden="true" /></Button
      >
    </Item.Actions>
  {/if}
</Item.Root>
