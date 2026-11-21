<script lang="ts" generics="T">
  import { untrack, type Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";

  const LOADING_DELAY_MS = 150;
  let {
    load,
    label,
    loadingText,
    loadingDelayMs = LOADING_DELAY_MS,
    errorMessage,
    children,
  }: {
    load: () => Promise<T>;
    label: string;
    loadingText?: string;
    loadingDelayMs?: number;
    errorMessage?: (error: unknown) => string;
    children: Snippet<[T]>;
  } = $props();
  const pending = untrack(() => load());
  let showLoading = $state(false);

  $effect(() => {
    if (loadingDelayMs <= 0) {
      showLoading = true;
      return;
    }
    showLoading = false;
    let active = true;
    const stop = () => {
      active = false;
      clearTimeout(timer);
    };
    const timer = setTimeout(() => {
      if (active) showLoading = true;
    }, loadingDelayMs);
    void pending.then(stop, stop);
    return stop;
  });
</script>

{#await pending}
  {#if showLoading}
    <p role="status" class="p-6 text-muted-foreground">
      {loadingText ?? m.s1_loading_target({ label })}
    </p>
  {/if}
{:then page}
  {@render children(page)}
{:catch error}
  <p role="alert" class="p-6 text-destructive">
    {errorMessage?.(error) ?? m.s1_load_target_failed_retry({ label })}
  </p>
{/await}
