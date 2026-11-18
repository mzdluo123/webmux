<script lang="ts">
  import type { Snippet } from "svelte";
  import Content from "./collapsible-content.svelte";
  import Root from "./collapsible.svelte";
  import { useManualDisclosureMotion } from "./manual-motion.svelte";

  let {
    header,
    open,
    onOpenChange,
    children,
  }: {
    header: Snippet;
    open: boolean;
    onOpenChange: (value: boolean) => void;
    children: Snippet;
  } = $props();

  let panel: HTMLDivElement | null = null;
  const motion = useManualDisclosureMotion(
    () => panel,
    () => open,
  );
</script>

<Root {open} onOpenChange={(value) => motion.toggle(value, onOpenChange)}>
  {@render header()}
  <div bind:this={panel} inert={!open} class:overflow-clip={motion.moving}>
    <Content forceMount={motion.moving}>
      {#if open || motion.moving}{@render children()}{/if}
    </Content>
  </div>
</Root>
