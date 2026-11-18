<script lang="ts">
  import type { Snippet } from "svelte";
  import { ScrollArea } from "$lib/components/ui/scroll-area";

  let {
    children,
    controls,
    footer,
    resetScrollOn,
    testId,
    fullScreen = false,
  }: {
    children: Snippet;
    controls?: Snippet;
    footer: Snippet;
    resetScrollOn?: string;
    testId: string;
    fullScreen?: boolean;
  } = $props();

  let viewport = $state<HTMLElement | null>(null);
  $effect(() => {
    if (resetScrollOn && viewport) viewport.scrollTop = 0;
  });
</script>

<div
  class={[
    "flex min-h-min min-w-0 flex-col gap-3 sm:h-[min(76dvh,calc(90dvh-7rem),36rem)]",
    fullScreen
      ? "max-sm:h-[calc(100dvh-10rem-env(safe-area-inset-bottom))]"
      : "max-sm:h-[min(76dvh,calc(95dvh-10rem-env(safe-area-inset-bottom)),36rem)]",
  ]}
  data-testid={testId}
>
  {@render controls?.()}
  <ScrollArea
    class="min-h-24 flex-1"
    bind:viewportRef={viewport}
    viewportClasses="overscroll-contain"
    viewportProps={{ tabindex: 0 }}
  >
    <div class="min-w-0 space-y-3">{@render children()}</div>
  </ScrollArea>
  <div class="shrink-0">
    {@render footer()}
  </div>
</div>
