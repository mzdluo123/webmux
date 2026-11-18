<script lang="ts">
  import type { Snippet } from "svelte";
  import { useManualDisclosureMotion } from "./manual-motion.svelte";
  import * as Collapsible from "$lib/components/ui/collapsible";

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

<Collapsible.Root
  {open}
  onOpenChange={(value) => motion.toggle(value, onOpenChange)}
>
  {@render header()}
  <div bind:this={panel} inert={!open} class:overflow-clip={motion.moving}>
    <Collapsible.Content forceMount={motion.moving}>
      {#if open || motion.moving}{@render children()}{/if}
    </Collapsible.Content>
  </div>
</Collapsible.Root>
