<script lang="ts">
  import { tick, untrack, type Snippet } from "svelte";
  import type { CreationContributions } from "../../features/context";
  import type { ProjectSelection } from "../../domains/sessions/project-selection";

  let {
    enabled,
    choosing,
    Picker,
    defaultCwd,
    recent,
    onselect,
    children,
  }: {
    enabled: boolean;
    choosing: boolean;
    Picker: CreationContributions["ProjectPicker"];
    defaultCwd: string;
    recent: readonly string[];
    onselect: (selection: ProjectSelection) => void;
    children: Snippet;
  } = $props();
  let mounted = $state(untrack(() => choosing));
  let picker = $state<{ back: () => boolean; showProjects: () => void }>();
  let form = $state<HTMLDivElement>();
  let wasChoosing = untrack(() => enabled && choosing);
  let projectScroll = 0;
  let settingsScroll = 0;
  let settingsFocus: HTMLElement | undefined;

  export function back() {
    return picker?.back() ?? false;
  }
  export async function showProjects() {
    mounted = true;
    await tick();
    picker?.showProjects();
  }

  // Both steps share one dialog viewport. Read the outgoing position before
  // hidden changes its scroll range, and restore the incoming step after layout.
  $effect.pre(() => {
    const node = form;
    const current = enabled && choosing;
    if (!node || current === wasChoosing) return;
    wasChoosing = current;
    const viewport = node.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (current) {
      settingsScroll = viewport?.scrollTop ?? 0;
      const active = document.activeElement;
      settingsFocus =
        active instanceof HTMLElement && node.contains(active)
          ? active
          : undefined;
    } else projectScroll = viewport?.scrollTop ?? 0;

    const frame = requestAnimationFrame(() => {
      if (!node.isConnected) return;
      if (viewport)
        viewport.scrollTop = current ? projectScroll : settingsScroll;
      if (current) return;

      const targets = node.querySelectorAll<HTMLElement>(
        '#session-location, #session-launch, input:not([type="checkbox"]):not(:disabled), button[type="submit"]:not(:disabled)',
      );
      const bounds = viewport?.getBoundingClientRect();
      const target = [settingsFocus, ...targets].find((element) => {
        if (!element || !node.contains(element) || element.matches(":disabled"))
          return false;
        const rect = element.getBoundingClientRect();
        return (
          rect.height > 0 &&
          (!bounds || (rect.top < bounds.bottom && rect.bottom > bounds.top))
        );
      });
      (target ?? node).focus({ preventScroll: true });
    });
    return () => cancelAnimationFrame(frame);
  });
</script>

{#if enabled && Picker && mounted}
  <div hidden={!choosing}>
    <Picker bind:this={picker} {defaultCwd} {recent} {onselect} />
  </div>
{/if}
<div bind:this={form} hidden={enabled && choosing} tabindex="-1">
  {@render children()}
</div>
