<script lang="ts">
  import type { Snippet } from "svelte";
  import IconArrowsMaximize from "~icons/tabler/arrows-maximize";
  import IconArrowsMinimize from "~icons/tabler/arrows-minimize";
  import IconBack from "~icons/tabler/arrow-left";
  import IconDots from "~icons/tabler/dots";
  import IconMinus from "~icons/tabler/minus";
  import IconPin from "~icons/tabler/pin";
  import { Button } from "$lib/components/ui/button";
  import * as Menu from "$lib/components/ui/dropdown-menu";
  import { m } from "$lib/paraglide/messages.js";
  import { dockEdges, type DockEdge } from "$lib/dock-layout";
  import DockStackTabs from "./DockStackTabs.svelte";
  import { getDock } from "./dock-context";

  let {
    id,
    title,
    onactivate,
    children,
    header,
    hostProps,
  }: {
    id: string;
    title: string;
    onactivate?: () => void;
    children: Snippet<[boolean]>;
    header?: Snippet;
    hostProps?: Record<string, unknown>;
  } = $props();

  const dock = getDock();
  const rect = $derived(dock.rect(id));
  const tabs = $derived(dock.tabs(id));
  const compact = $derived(dock.compact());
  const zoomed = $derived(dock.zoomed(id));
  const edgeRails = $derived(dock.edgeRails());
  const peeked = $derived(dock.peeked(id));
  const canMove = $derived(!compact && !zoomed && !peeked);
  const targets = $derived(canMove ? dock.targets(id) : undefined);
  const showTabs = $derived(!compact && !peeked && tabs.length > 1);
  const zoomLabel = $derived(
    peeked
      ? m.dock_pin({ title })
      : zoomed
        ? m.dock_restore({ title })
        : m.dock_maximize({ title }),
  );
  const menuLabel = $derived(
    edgeRails ? m.dock_actions({ title }) : m.dock_move({ title }),
  );
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
    {...hostProps}
    use:dock.mount
    class="dock-panel"
    data-dock-panel={id}
    data-dock-active={dock.active(id) || undefined}
    data-dock-peek={peeked || undefined}
    aria-label={title}
    hidden={!rect}
    inert={!rect}
    style:--dock-x={rect ? `${rect.x * 100}%` : undefined}
    style:--dock-y={rect ? `${rect.y * 100}%` : undefined}
    style:--dock-w={rect ? `${rect.width * 100}%` : undefined}
    style:--dock-h={rect ? `${rect.height * 100}%` : undefined}
    onpointerdowncapture={() => {
      dock.select(id);
      onactivate?.();
    }}
    onfocusin={() => {
      dock.select(id);
      onactivate?.();
    }}
  >
    <div
      class="dock-title"
      role="presentation"
      draggable={canMove}
      ondblclick={(event) => {
        if (
          !peeked &&
          event.target instanceof Element &&
          !event.target.closest("button, input, a, [role=tab]")
        )
          dock.toggleZoom(id);
      }}
      ondragstart={(event) => {
        if (!canMove || !event.dataTransfer) return;
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData("text/plain", title);
        dock.drag(id);
      }}
      ondragend={() => dock.drag(undefined)}
    >
      {#if compact && dock.canGoBack()}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label={m.s8_back()}
          title={m.s8_back()}
          onclick={dock.back}><IconBack aria-hidden="true" /></Button
        >
      {/if}
      {#if showTabs}
        <DockStackTabs {id} {tabs} show={dock.show} />
      {/if}
      {#if header}{@render header()}{/if}
      <div class="dock-title-space" class:with-header={!!header}>
        {#if !header && !showTabs}<span class="truncate" {title}>{title}</span
          >{/if}
      </div>
      {#if !dock.centralCommands()}
        {#if !compact}
          <Button
            variant="ghost"
            size="icon-sm"
            type="button"
            aria-label={zoomLabel}
            title={zoomLabel}
            onclick={() => (peeked ? dock.show(id) : dock.toggleZoom(id))}
          >
            {#if peeked}
              <IconPin aria-hidden="true" />
            {:else if zoomed}
              <IconArrowsMinimize aria-hidden="true" />
            {:else}
              <IconArrowsMaximize aria-hidden="true" />
            {/if}
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
        {#if !compact && !peeked && (edgeRails || targets?.length)}
          <Menu.Root>
            <Menu.Trigger>
              {#snippet child({ props })}
                <Button
                  {...props}
                  data-dock-actions
                  variant="ghost"
                  size="icon-sm"
                  aria-label={menuLabel}
                  title={menuLabel}
                >
                  <IconDots aria-hidden="true" />
                </Button>
              {/snippet}
            </Menu.Trigger>
            <Menu.Content align="end">
              {#if edgeRails}
                <Menu.Item onSelect={() => dock.minimize(id, "left")}
                  >{m.dock_minimize_left()}</Menu.Item
                >
                <Menu.Item onSelect={() => dock.minimize(id, "right")}
                  >{m.dock_minimize_right()}</Menu.Item
                >
                <Menu.Item onSelect={() => dock.minimize(id, "bottom")}
                  >{m.dock_minimize_bottom()}</Menu.Item
                >
                {#if targets?.length}<Menu.Separator />{/if}
              {/if}
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
      {/if}
    </div>
    <div
      id={`dock-content-${id}`}
      role={showTabs ? "tabpanel" : undefined}
      aria-label={showTabs ? title : undefined}
      data-dock-body
      class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
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

  .dock-panel[data-dock-active]::after {
    content: "";
    position: absolute;
    inset: 0;
    z-index: 10;
    pointer-events: none;
    border: 2px solid var(--primary);
  }

  .dock-panel[data-dock-peek] {
    z-index: 30;
  }

  .dock-panel[hidden] {
    display: none;
  }

  .dock-title {
    display: flex;
    align-items: center;
    block-size: var(--control-default);
    flex: none;
    min-inline-size: 0;
    border-block-end: 1px solid var(--border);
  }

  @media (any-pointer: coarse), (max-width: 639px) {
    .dock-title {
      block-size: var(--control-touch);
    }
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

  .dock-title-space.with-header {
    flex: 0 0 1rem;
    padding: 0;
  }
  .dock-title[draggable="true"] {
    user-select: none;
    cursor: grab;
  }
</style>
