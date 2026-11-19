<script lang="ts">
  import type { Snippet } from "svelte";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";

  let {
    title,
    count,
    open,
    onOpenChange,
    children,
  }: {
    title: string;
    count: number;
    open: boolean;
    onOpenChange: (value: boolean) => void;
    children: Snippet;
  } = $props();
</script>

<Collapsible.MotionRoot {open} {onOpenChange}>
  {#snippet header()}
    <Collapsible.Trigger>
      {#snippet child({ props })}
        <Button {...props} variant="ghost" class="w-full justify-start">
          <IconChevronRight
            class={["motion-safe:transition-transform", open && "rotate-90"]}
            aria-hidden="true"
          />
          {title}
          <span class="font-mono text-xs text-muted-foreground">{count}</span>
        </Button>
      {/snippet}
    </Collapsible.Trigger>
  {/snippet}
  {@render children()}
</Collapsible.MotionRoot>
