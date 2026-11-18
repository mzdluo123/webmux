<script lang="ts">
  import type { Snippet } from "svelte";
  import { ScrollArea as ScrollAreaPrimitive } from "bits-ui";
  import { cn, type WithoutChild } from "$lib/utils.js";
  import { Scrollbar } from "./index.js";

  let {
    ref = $bindable(null),
    viewportRef = $bindable(null),
    class: className,
    orientation = "vertical",
    scrollbarXClasses = "",
    scrollbarYClasses = "",
    viewportClasses = "",
    viewportProps = {},
    onscroll,
    children,
    footer,
    ...restProps
  }: WithoutChild<ScrollAreaPrimitive.RootProps> & {
    orientation?: "vertical" | "horizontal" | "both" | undefined;
    scrollbarXClasses?: string | undefined;
    scrollbarYClasses?: string | undefined;
    viewportClasses?: string | undefined;
    viewportProps?: WithoutChild<ScrollAreaPrimitive.ViewportProps> | undefined;
    viewportRef?: HTMLElement | null;
    footer?: Snippet;
    onscroll?: ScrollAreaPrimitive.ViewportProps["onscroll"];
  } = $props();
</script>

{#snippet area()}
  <ScrollAreaPrimitive.Root
    bind:ref
    data-slot="scroll-area"
    class={cn(
      "relative flex min-h-0 min-w-0 flex-col",
      footer ? "flex-1" : className,
    )}
    {...restProps}
  >
    <ScrollAreaPrimitive.Viewport
      {...viewportProps}
      bind:ref={viewportRef}
      data-slot="scroll-area-viewport"
      {onscroll}
      class={cn(
        "size-full min-h-0 min-w-0 rounded-[inherit] transition-[color,box-shadow] outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1",
        viewportClasses,
      )}
    >
      {@render children?.()}
    </ScrollAreaPrimitive.Viewport>
    {#if orientation === "vertical" || orientation === "both"}
      <Scrollbar orientation="vertical" class={scrollbarYClasses} />
    {/if}
    {#if orientation === "horizontal" || orientation === "both"}
      <Scrollbar orientation="horizontal" class={scrollbarXClasses} />
    {/if}
    <ScrollAreaPrimitive.Corner />
  </ScrollAreaPrimitive.Root>
{/snippet}

{#if footer}
  <div class={cn("flex min-h-0 min-w-0 flex-col", className)}>
    {@render area()}
    {@render footer()}
  </div>
{:else}
  {@render area()}
{/if}
