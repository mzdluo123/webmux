<script lang="ts" generics="T">
  import { untrack, type Snippet } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import InlineNotice from "./InlineNotice.svelte";

  let {
    load,
    label,
    onclose,
    children,
  }: {
    load: () => Promise<T>;
    label: string;
    onclose: () => void;
    children: Snippet<[T]>;
  } = $props();
  const pending = untrack(() => load());
</script>

{#await pending}
  <InlineNotice class="fixed inset-x-4 bottom-4 z-50">
    <div class="flex items-center justify-between gap-2">
      <span>{m.s1_loading_target({ label })}</span>
      <Button variant="outline" size="sm" onclick={onclose}
        >{m.s1_cancel()}</Button
      >
    </div>
  </InlineNotice>
{:then page}
  {@render children(page)}
{:catch}
  <InlineNotice tone="danger" class="fixed inset-x-4 bottom-4 z-50">
    <div class="flex items-center justify-between gap-2">
      <span>{m.s1_load_target_failed_retry({ label })}</span>
      <Button variant="outline" size="sm" onclick={onclose}
        >{m.s1_close()}</Button
      >
    </div>
  </InlineNotice>
{/await}
