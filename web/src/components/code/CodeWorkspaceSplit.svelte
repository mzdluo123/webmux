<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import {
    navMin,
    navMax,
    useNavigationResize,
  } from "../../hooks/code/code-navigation-resize.svelte";
  let {
    width = $bindable(),
    open,
    navigation,
    children,
  }: {
    width: number;
    open: boolean;
    navigation?: Snippet;
    children: Snippet;
  } = $props();
  let inner = $state<HTMLDivElement>();
  let navDivider = $state<HTMLDivElement>();
  const { resizeStart, resizeKey } = useNavigationResize(
    () => inner,
    () => navDivider,
    { get: () => width, set: (value) => (width = value) },
  );
</script>

<div
  class="code-inner flex min-h-0 flex-1 overflow-hidden"
  style:--code-nav-width={`${width}px`}
  bind:this={inner}
>
  {#if navigation}
    <div class="contents" class:hidden={!open}>
      {@render navigation()}
      <!-- A focusable separator with aria-valuenow is an interactive widget (WAI-ARIA window splitter). -->
      <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
      <div
        bind:this={navDivider}
        class="code-nav-divider"
        role="separator"
        aria-orientation="vertical"
        aria-label={m.s5_resize_nav_width()}
        aria-valuemin={navMin}
        aria-valuemax={navMax}
        aria-valuenow={width}
        tabindex="0"
        onpointerdown={resizeStart}
        onkeydown={resizeKey}
      ></div>
    </div>
  {/if}
  {@render children()}
</div>
