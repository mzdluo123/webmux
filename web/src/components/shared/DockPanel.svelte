<script lang="ts">
  import type { Snippet } from "svelte";
  import IconArrowsMaximize from "~icons/tabler/arrows-maximize";
  import IconArrowsMinimize from "~icons/tabler/arrows-minimize";
  import IconDots from "~icons/tabler/dots";
  import IconMinus from "~icons/tabler/minus";
  import { Button } from "$lib/components/ui/button";
  import * as Menu from "$lib/components/ui/dropdown-menu";
  import { m } from "$lib/paraglide/messages.js";
  import { dockEdges, type DockEdge } from "$lib/dock-layout";
  import { getDock } from "./dock-context";

  let {
    id,
    title,
    onactivate,
    children,
  }: {
    id: string;
    title: string;
    onactivate?: () => void;
    children: Snippet<[boolean]>;
  } = $props();

  const dock = getDock();
  const rect = $derived(dock.rect(id));
  const tabs = $derived(dock.tabs(id));
  const targets = $derived(dock.targets(id));
  const compact = $derived(dock.compact());
  const zoomed = $derived(dock.zoomed(id));
  $effect(() =>
    dock.register({
      id,
      get title() {
        return title;
      },
    }),
  );

  function edgeLabel(edge: DockEdge): string {
    switch (edge) {
      case "left":
        return m.dock_left();
      case "right":
        return m.dock_right();
      case "top":
        return m.dock_top();
      case "bottom":
        return m.dock_bottom();
      case "center":
        return m.dock_center();
    }
  }
</script>

<div class="dock-source">
  <section
    use:dock.mount
    class="dock-panel"
    data-dock-panel={id}
    aria-label={title}
    hidden={!rect}
    inert={!rect}
    style:--dock-x={rect ? `${rect.x * 100}%` : undefined}
    style:--dock-y={rect ? `${rect.y * 100}%` : undefined}
    style:--dock-w={rect ? `${rect.width * 100}%` : undefined}
    style:--dock-h={rect ? `${rect.height * 100}%` : undefined}
    ondragover={(event) => dock.dragOver(event, id)}
    ondrop={(event) => dock.drop(event, id)}
  >
    <div class="dock-title">
      {#if !compact && tabs.length > 1 && !zoomed}
        <div
          class="flex min-w-0 max-w-full overflow-x-auto"
          role="group"
          aria-label={m.dock_tabs()}
        >
          {#each tabs as tab (tab.id)}
            <Button
              size="sm"
              variant="ghost"
              aria-pressed={tab.id === id}
              onclick={() => dock.show(tab.id)}
            >
              {tab.title}
            </Button>
          {/each}
        </div>
      {/if}
      <div
        class="dock-title-space"
        role="presentation"
        draggable={!compact && !zoomed}
        ondblclick={() => dock.toggleZoom(id)}
        ondragstart={(event) => {
          if (!event.dataTransfer) return;
          event.dataTransfer.effectAllowed = "move";
          event.dataTransfer.setData("text/plain", title);
          dock.drag(id);
        }}
        ondragend={() => dock.drag(undefined)}
      >
        {#if compact || tabs.length < 2 || zoomed}<span class="truncate" {title}
            >{title}</span
          >{/if}
      </div>
      {#if !compact}
        <Button
          variant="ghost"
          size="icon-sm"
          type="button"
          aria-label={zoomed
            ? m.dock_restore({ title })
            : m.dock_maximize({ title })}
          title={zoomed
            ? m.dock_restore({ title })
            : m.dock_maximize({ title })}
          onclick={() => dock.toggleZoom(id)}
        >
          {#if zoomed}<IconArrowsMinimize
              aria-hidden="true"
            />{:else}<IconArrowsMaximize aria-hidden="true" />{/if}
        </Button>
      {/if}
      <Button
        variant="ghost"
        size="icon-sm"
        type="button"
        aria-label={m.dock_minimize({ title })}
        title={m.dock_minimize({ title })}
        onclick={() => dock.minimize(id)}
      >
        <IconMinus aria-hidden="true" />
      </Button>
      {#if !compact && !zoomed && targets.length}
        <Menu.Root>
          <Menu.Trigger>
            {#snippet child({ props })}
              <Button
                {...props}
                data-dock-actions
                variant="ghost"
                size="icon-sm"
                aria-label={m.dock_move({ title })}
                title={m.dock_move({ title })}
              >
                <IconDots aria-hidden="true" />
              </Button>
            {/snippet}
          </Menu.Trigger>
          <Menu.Content align="end">
            {#each targets as target (target.id)}
              <Menu.Sub>
                <Menu.SubTrigger>{target.title}</Menu.SubTrigger>
                <Menu.SubContent>
                  {#each dockEdges as edge (edge)}
                    <Menu.Item onSelect={() => dock.move(id, target.id, edge)}
                      >{edgeLabel(edge)}</Menu.Item
                    >
                  {/each}
                </Menu.SubContent>
              </Menu.Sub>
            {/each}
          </Menu.Content>
        </Menu.Root>
      {/if}
    </div>
    <div
      data-dock-body
      class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      onpointerdowncapture={onactivate}
      onfocusin={onactivate}
    >
      {@render children(!!rect)}
    </div>
  </section>
</div>

<style>
  .dock-source {
    display: none;
  }

  .dock-panel {
    display: flex;
    isolation: isolate;
    flex-direction: column;
    overflow: hidden;
    min-inline-size: 0;
    min-block-size: 0;
    background: var(--background);
  }

  .dock-panel[hidden] {
    display: none;
  }

  .dock-title {
    display: flex;
    align-items: center;
    flex: none;
    min-inline-size: 0;
    border-block-end: 1px solid var(--border);
  }

  .dock-title-space {
    display: flex;
    align-items: center;
    align-self: stretch;
    flex: 1 1 2rem;
    min-inline-size: 1rem;
    padding-inline: 0.5rem;
    user-select: none;
    font-size: 0.75rem;
  }

  .dock-title-space[draggable="true"] {
    cursor: grab;
  }
</style>
