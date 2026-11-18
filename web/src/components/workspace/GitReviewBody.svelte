<script lang="ts">
  import type { Snippet } from "svelte";

  let {
    children,
    resetScrollOn,
    testId,
  }: {
    children: Snippet;
    resetScrollOn?: string;
    testId: string;
  } = $props();

  let bodyEl = $state<HTMLDivElement | null>(null);
  $effect(() => {
    if (!resetScrollOn || !bodyEl) return;
    const viewport = bodyEl.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (viewport) viewport.scrollTop = 0;
  });
</script>

<div bind:this={bodyEl} class="min-w-0 space-y-3" data-testid={testId}>
  {@render children()}
</div>
