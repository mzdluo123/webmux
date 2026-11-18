<script lang="ts">
  import type { Snippet } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import { useDockMotion } from "../../hooks/shared/dock-motion.svelte";

  let { title, children }: { title: string; children: Snippet } = $props();
  let open = $state(false);
  let panel = $state<HTMLDivElement | null>(null);
  useDockMotion(
    () => panel,
    () => open,
  );
</script>

<Collapsible.Root bind:open>
  <Collapsible.Trigger>
    {#snippet child({ props })}
      <Button {...props} variant="outline">{title}</Button>
    {/snippet}
  </Collapsible.Trigger>
  <Collapsible.Content forceMount bind:ref={panel} hidden={!open} inert={!open}>
    <div class="pt-3">{@render children()}</div>
  </Collapsible.Content>
</Collapsible.Root>
