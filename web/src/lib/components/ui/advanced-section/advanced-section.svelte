<script lang="ts">
  import { onMount, tick, type Snippet } from "svelte";
  import { animate } from "motion/mini";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import * as Collapsible from "$lib/components/ui/collapsible";

  // Low-frequency form fields folded behind one line. The summary repeats
  // what is set so the folded state still shows it; it is hidden from
  // assistive technology because the fields carry the same values.
  let {
    title,
    summary = "",
    open = $bindable(false),
    children,
  }: {
    title: string;
    summary?: string;
    open?: boolean;
    children: Snippet;
  } = $props();

  let trigger = $state<HTMLButtonElement | null>(null);
  let panel = $state<HTMLDivElement | null>(null);
  let moving = $state(false);
  let previous: boolean | undefined;
  let generation = 0;
  let animation: ReturnType<typeof animate> | undefined;

  function settle() {
    generation++;
    animation?.cancel();
    animation = undefined;
    panel?.style.removeProperty("height");
    moving = false;
  }

  $effect.pre(() => {
    const expanded = open;
    const node = panel;
    const button = trigger;
    if (!node || !button) return;
    const changed = previous !== undefined && previous !== expanded;
    previous = expanded;
    if (!changed) return;
    const from = node.getBoundingClientRect().height;
    if (!expanded && node.contains(document.activeElement)) {
      button.focus({ preventScroll: true });
    }
    settle();
    if (matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    moving = true;
    const version = generation;
    void tick().then(() => {
      if (version !== generation || !node.isConnected) return;
      const to = expanded ? node.scrollHeight : 0;
      animation = animate(
        node,
        { height: [`${from}px`, `${to}px`] },
        { duration: 0.24, ease: [0.22, 1, 0.36, 1] },
      );
      void animation.then(() => {
        if (version === generation) settle();
      });
    });
  });

  onMount(() => {
    const reduced = matchMedia("(prefers-reduced-motion: reduce)");
    reduced.addEventListener("change", settle);
    window.addEventListener("resize", settle);
    return () => {
      reduced.removeEventListener("change", settle);
      window.removeEventListener("resize", settle);
      settle();
    };
  });
</script>

<Collapsible.Root bind:open>
  <Collapsible.Trigger
    bind:ref={trigger}
    class="flex w-full min-w-0 items-center gap-1.5 rounded-md px-1 py-1 text-left text-sm text-muted-foreground hover:bg-muted hover:text-foreground any-pointer-coarse:min-h-11"
  >
    <IconChevronRight
      class={open
        ? "size-3.5 shrink-0 rotate-90 motion-safe:transition-transform"
        : "size-3.5 shrink-0 motion-safe:transition-transform"}
      aria-hidden="true"
    />
    <span class={summary ? "shrink-0" : "min-w-0 wrap-anywhere"}>{title}</span>
    {#if summary}<span
        class="min-w-0 truncate font-mono text-xs"
        aria-hidden="true">{summary}</span
      >{/if}
  </Collapsible.Trigger>
  <div bind:this={panel} inert={!open} class:overflow-clip={moving}>
    <Collapsible.Content forceMount={moving} class="space-y-3 pt-3">
      {#if open || moving}{@render children()}{/if}
    </Collapsible.Content>
  </div>
</Collapsible.Root>
