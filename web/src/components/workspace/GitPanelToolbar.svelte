<script lang="ts">
  import type { Snippet } from "svelte";
  import { cn } from "$lib/utils.js";

  let {
    as = "div",
    edge = "top",
    children,
  }: {
    as?: "div" | "header";
    edge?: "top" | "bottom";
    children: Snippet;
  } = $props();
</script>

<svelte:element
  this={as}
  class={cn(
    "flex min-w-0 flex-wrap items-center justify-between gap-2 bg-background px-2.5 py-1.5",
    edge === "bottom" ? "border-t" : "border-b",
  )}
>
  {@render children()}
</svelte:element>
