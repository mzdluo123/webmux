<script lang="ts">
  import type { Snippet } from "svelte";

  let {
    as = "div",
    children,
  }: {
    as?: "div" | "header";
    children: Snippet;
  } = $props();
</script>

<svelte:element
  this={as}
  class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b bg-background px-2.5 py-1.5"
>
  {@render children()}
</svelte:element>
