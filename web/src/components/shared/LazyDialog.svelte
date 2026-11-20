<script lang="ts" generics="T">
  import { untrack, type Snippet } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { InlineNotice } from "$lib/components/ui/inline-notice";

  let {
    load,
    label,
    showLoading = true,
    onclose,
    children,
  }: {
    load: () => Promise<T>;
    label: string;
    showLoading?: boolean;
    onclose: () => void;
    children: Snippet<[T]>;
  } = $props();
  const pending = untrack(() => load());
</script>

{#await pending}
  {#if showLoading}
    <InlineNotice surface="floating" class="fixed inset-x-4 bottom-4 z-50">
      {m.s1_loading_target({ label })}
      {#snippet actions()}
        <Button variant="outline" size="sm" onclick={onclose}
          >{m.s1_cancel()}</Button
        >
      {/snippet}
    </InlineNotice>
  {/if}
{:then page}
  {@render children(page)}
{:catch}
  <InlineNotice
    tone="danger"
    surface="floating"
    class="fixed inset-x-4 bottom-4 z-50"
  >
    {m.s1_load_target_failed_retry({ label })}
    {#snippet actions()}
      <Button variant="outline" size="sm" onclick={onclose}
        >{m.s1_close()}</Button
      >
    {/snippet}
  </InlineNotice>
{/await}
