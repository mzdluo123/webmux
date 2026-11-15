<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import InlineNotice from "../components/shared/InlineNotice.svelte";
  import type { PluginID } from "../contracts/plugin-schemas";

  let { errors }: { errors: Partial<Record<PluginID, string>> } = $props();
  let dismissed = $state<Partial<Record<PluginID, string>>>({});
  const visibleErrors = $derived(
    Object.entries(errors).filter(
      ([id, message]) => dismissed[id as PluginID] !== message,
    ),
  );
</script>

{#if visibleErrors.length}
  <div
    class="fixed inset-x-2 bottom-2 z-50 max-h-[50dvh] space-y-2 overflow-y-auto"
  >
    {#each visibleErrors as [id, message] (id)}
      <InlineNotice tone="warning">
        <div class="min-w-0 space-y-2">
          <p class="break-words">
            {m.s1_plugin_load_notice({ id, message: message ?? "" })}
          </p>
          <div class="flex flex-wrap justify-end gap-2">
            <Button
              variant="ghost"
              size="sm"
              aria-label={m.s1_dismiss_plugin_notice_aria({ id })}
              onclick={() => (dismissed = { ...dismissed, [id]: message })}
              >{m.s1_dismiss_notice()}</Button
            >
            <Button
              variant="outline"
              size="sm"
              onclick={() => location.reload()}>{m.s1_reload_page()}</Button
            >
          </div>
        </div>
      </InlineNotice>
    {/each}
  </div>
{/if}
