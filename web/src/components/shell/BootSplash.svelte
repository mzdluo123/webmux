<script lang="ts">
  import Brand from "./Brand.svelte";
  import { m } from "$lib/paraglide/messages.js";

  const DEFAULT_DELAY_MS = 180;
  let { delayMs = DEFAULT_DELAY_MS }: { delayMs?: number } = $props();
  let visible = $state(false);

  $effect(() => {
    if (delayMs <= 0) {
      visible = true;
      return;
    }
    visible = false;
    const timer = setTimeout(() => {
      visible = true;
    }, delayMs);
    return () => clearTimeout(timer);
  });
</script>

{#if visible}
  <main class="flex min-h-dvh flex-col items-center justify-center gap-3 p-6">
    <p class="brand"><Brand /></p>
    <p role="status" class="text-sm text-muted-foreground">
      {m.s1_connecting_ellipsis()}
    </p>
  </main>
{/if}
