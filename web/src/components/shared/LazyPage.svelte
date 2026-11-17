<script lang="ts" generics="T">
  import { untrack, type Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";

  let {
    load,
    label,
    loadingText,
    errorMessage,
    children,
  }: {
    load: () => Promise<T>;
    label: string;
    loadingText?: string;
    errorMessage?: (error: unknown) => string;
    children: Snippet<[T]>;
  } = $props();
  const pending = untrack(() => load());
</script>

{#await pending}
  <p role="status" class="p-6 text-muted-foreground">
    {loadingText ?? m.s1_loading_target({ label })}
  </p>
{:then page}
  {@render children(page)}
{:catch error}
  <p role="alert" class="p-6 text-destructive">
    {errorMessage?.(error) ?? m.s1_load_target_failed_retry({ label })}
  </p>
{/await}
