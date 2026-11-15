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

  export function back() {
    return picker?.back() ?? false;
  }
  export async function showProjects() {
    mounted = true;
    await tick();
    picker?.showProjects();
  }

  // Both steps stay mounted for their drafts, so focus must explicitly leave
  // the hidden Command search when selection reveals the session settings.
  $effect(() => {
    const current = enabled && choosing;
    const enteringSettings = wasChoosing && !current;
    wasChoosing = current;
    let canceled = false;
    let frame = 0;
    if (enteringSettings)
      void tick().then(() => {
        if (canceled) return;
        frame = requestAnimationFrame(() => {
          const targets = form?.querySelectorAll<HTMLElement>(
            '#session-location, #session-launch, input:not([type="checkbox"]):not(:disabled), button[type="submit"]:not(:disabled)',
          );
          for (const target of targets ?? []) {
            if (!target.getClientRects().length || target.closest("[hidden]"))
              continue;
            target.focus();
            break;
          }
        });
      });
    return () => {
      canceled = true;
      cancelAnimationFrame(frame);
    };
  });
</script>

{#if enabled && Picker && mounted}
  <div hidden={!choosing}>
    <Picker bind:this={picker} {defaultCwd} {recent} {onselect} />
  </div>
{/if}
<div bind:this={form} hidden={enabled && choosing}>
  {@render children()}
</div>
