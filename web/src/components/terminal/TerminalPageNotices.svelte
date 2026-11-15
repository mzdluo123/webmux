<script lang="ts">
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";

  let {
    pluginErrors,
    error,
    reloadPage,
    dismissError,
  }: {
    pluginErrors?: string[];
    error: string;
    reloadPage: () => void;
    dismissError: () => void;
  } = $props();
</script>

{#if pluginErrors?.length}
  <InlineNotice tone="warning">
    <div class="flex items-center justify-between gap-2">
      <span
        >{m.s4_plugin_load_partial_warning({
          errors: pluginErrors.join(m.s4_list_semicolon_sep()),
        })}</span
      >
      <Button variant="outline" size="sm" onclick={reloadPage}
        >{m.s4_reload_page()}</Button
      >
    </div>
  </InlineNotice>
{/if}
{#if error}
  <InlineNotice tone="danger">
    <div class="flex items-center justify-between gap-2">
      <span>{error}</span>
      <Button variant="ghost" size="sm" onclick={dismissError}
        >{m.s4_close()}</Button
      >
    </div>
  </InlineNotice>
{/if}
