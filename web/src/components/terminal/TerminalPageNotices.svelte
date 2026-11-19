<script lang="ts">
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";

  let {
    featureErrors,
    error,
    reloadPage,
    dismissError,
  }: {
    featureErrors?: string[];
    error: string;
    reloadPage: () => void;
    dismissError: () => void;
  } = $props();
</script>

{#if featureErrors?.length}
  <InlineNotice tone="warning">
    {m.s4_feature_load_partial_warning({
      errors: featureErrors.join(m.s4_list_semicolon_sep()),
    })}
    {#snippet actions()}
      <Button variant="outline" size="sm" onclick={reloadPage}
        >{m.s4_reload_page()}</Button
      >
    {/snippet}
  </InlineNotice>
{/if}
{#if error}
  <InlineNotice tone="danger">
    {error}
    {#snippet actions()}
      <Button variant="ghost" size="sm" onclick={dismissError}
        >{m.s4_close()}</Button
      >
    {/snippet}
  </InlineNotice>
{/if}
