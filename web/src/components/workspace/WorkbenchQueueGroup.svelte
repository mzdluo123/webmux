<script lang="ts">
  import { animate } from "motion/mini";
  import { onMount, tick, type Snippet } from "svelte";
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

  let panel: HTMLDivElement;
  let moving = $state(false);
  let generation = 0;
  let expectedOpen: boolean | undefined;
  let animation: ReturnType<typeof animate> | undefined;
  const motionQuery = "(prefers-reduced-motion: reduce)";

  function settle() {
    generation++;
    animation?.cancel();
    animation = undefined;
    panel?.style.removeProperty("height");
    moving = false;
  }

  // The search can open groups without using the trigger.
  $effect.pre(() => {
    if (open !== expectedOpen) settle();
  });

  function toggle(next: boolean) {
    const from = panel.getBoundingClientRect().height;
    settle();
    expectedOpen = next;
    onOpenChange(next);
    if (matchMedia(motionQuery).matches) return;
    moving = true;
    const version = generation;
    void tick().then(() => {
      if (version !== generation || !panel.isConnected) return;
      animation = animate(
        panel,
        { height: [`${from}px`, `${next ? panel.scrollHeight : 0}px`] },
        { duration: 0.24, ease: [0.22, 1, 0.36, 1] },
      );
      void animation.then(() => {
        if (version === generation) settle();
      });
    });
  }

  onMount(() => {
    const media = matchMedia(motionQuery);
    media.addEventListener("change", settle);
    window.addEventListener("resize", settle);
    return () => {
      media.removeEventListener("change", settle);
      window.removeEventListener("resize", settle);
      settle();
    };
  });
</script>

<Collapsible.Root {open} onOpenChange={toggle}>
  <Collapsible.Trigger>
    {#snippet child({ props })}
      <Button {...props} variant="ghost" class="w-full justify-start">
        <IconChevronRight class={open ? "rotate-90" : ""} aria-hidden="true" />
        {title}
        <span class="font-mono text-xs text-muted-foreground">{count}</span>
      </Button>
    {/snippet}
  </Collapsible.Trigger>
  <div bind:this={panel} inert={!open} class:overflow-clip={moving}>
    <Collapsible.Content forceMount={moving}>
      {#if open || moving}{@render children()}{/if}
    </Collapsible.Content>
  </div>
</Collapsible.Root>
