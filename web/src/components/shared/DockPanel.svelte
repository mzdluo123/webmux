<script lang="ts">
  import type { Component, Snippet } from "svelte";
  import IconArrowsMaximize from "~icons/tabler/arrows-maximize";
  import IconArrowsMinimize from "~icons/tabler/arrows-minimize";
  import IconBack from "~icons/tabler/arrow-left";
  import IconDots from "~icons/tabler/dots-vertical";
  import IconMinus from "~icons/tabler/minus";
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
    titleContent,
    commands,
    icon,
    tabLabel,
    tooltip,
    onclose,
  }: {
    id: string;
    title: string;
    onactivate?: () => void;
    children: Snippet<[boolean]>;
    header?: Snippet;
    // Decorate the desktop title inside its flexible slot, not beside it.
    titleContent?: Snippet<[Snippet]>;
    commands?: Snippet;
    icon?: Component;
    tabLabel?: string;
    tooltip?: string;
    onclose?: () => void;
  } = $props();

  const dock = getDock();
  const rect = $derived(dock.rect(id));
  const tabs = $derived(dock.tabs(id));
  const compact = $derived(dock.compact());
  const localHeader = $derived(
    compact && commands && !onclose ? commands : header,
  );
  const showTitle = $derived(!onclose && (commands ? !compact : !header));
  const zoomed = $derived(dock.zoomed(id));
  const edgeRails = $derived(dock.edgeRails());
  const canMove = $derived(!compact && !zoomed);
  const targets = $derived(canMove ? dock.targets(id) : undefined);
  const showTabs = $derived(
    (!compact && tabs.length > 1) || (!!onclose && !!tabs.length),
  );
  // Compact mode exposes no zoom, edge rails or move targets; omit the empty
  // dock actions menu (and in edge-rail workspaces, the duplicate minimize
  // button already owned by the panel picker drawer).
  const showDockActions = $derived(!compact);
  const showMinimize = $derived(!(compact && edgeRails));
  const showTitleSpace = $derived(!compact || showTitle || !localHeader);
  // Desktop single-session views already lift their title, status and pane menu
  // into SessionWorkspaceHeader; omit the duplicate pane title bar until splits,
  // tabs or zoom need per-pane chrome.
  const hideSingleTitle = $derived(
    !compact &&
      !!commands &&
      !onclose &&
      !showTabs &&
      !zoomed &&
      !dock.hasMultipleVisiblePanels(),
  );
  const zoomLabel = $derived(
    zoomed ? m.dock_restore({ title }) : m.dock_maximize({ title }),
  );
  const menuLabel = $derived(m.dock_actions({ title }));
  $effect(() =>
    dock.register({
      id,
      commands,
      onactivate,
      onclose,
      get icon() {
        return icon;
      },
      get tabLabel() {
        return tabLabel;
      },
      get tooltip() {
        return tooltip;
      },
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

{#snippet titleLabel()}
  {#if showTitle && !showTabs}<span class="truncate" {title}>{title}</span>{/if}
{/snippet}

<div class="dock-source">
  <section
    use:dock.mount
    class="dock-panel"
    data-dock-panel={id}
    data-dock-active={dock.active(id) || undefined}
    data-dock-multiple={dock.hasMultipleVisiblePanels() || undefined}
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
    {#if !hideSingleTitle}
      <div
        class="dock-title"
        data-compact-dock-title={compact || undefined}
        data-session-dock-title={(compact && commands && !onclose) || undefined}
        role="presentation"
        draggable={canMove}
        ondblclick={(event) => {
          if (
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
            size="icon-touch-sm"
            variant="ghost"
            aria-label={m.s8_back()}
            title={m.s8_back()}
            onclick={dock.back}><IconBack aria-hidden="true" /></Button
          >
        {/if}
        {#if showTabs}
          <DockStackTabs {id} {tabs} {canMove} show={dock.show} />
        {/if}
        {@render localHeader?.()}
        {#if showTitleSpace}
          <div
            class="dock-title-space"
            class:with-header={!!localHeader && !showTitle}
          >
            {#if !compact && titleContent}
              {@render titleContent(titleLabel)}
            {:else}
              {@render titleLabel()}
            {/if}
          </div>
        {/if}
        {#if showDockActions}
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
            <Menu.Content align="end" class="w-56">
              <Menu.Label>{title}</Menu.Label>
              <Menu.Item onSelect={() => dock.toggleZoom(id)}>
                {#if zoomed}<IconArrowsMinimize aria-hidden="true" />
                {:else}<IconArrowsMaximize aria-hidden="true" />{/if}
                {zoomLabel}
              </Menu.Item>
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
        {#if showMinimize}
          <Button
            variant="ghost"
            size="icon-touch-sm"
            aria-label={m.dock_minimize({ title })}
            title={m.dock_minimize({ title })}
            onclick={() => dock.minimize(id)}
            ><IconMinus aria-hidden="true" /></Button
          >
        {/if}
      </div>
    {/if}
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
    --dock-touch-height: var(--control-touch);
    display: flex;
    isolation: isolate;
    flex-direction: column;
    overflow: hidden;
    min-inline-size: 0;
    min-block-size: 0;
    background: var(--background);
  }

  .dock-panel[data-dock-active][data-dock-multiple]::after {
    content: "";
    position: absolute;
    inset: 0;
    z-index: 10;
    pointer-events: none;
    border: 2px solid var(--primary);
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
      /* Include the 1px bottom border so 44px touch buttons do not clip by 0.5px. */
      block-size: calc(var(--dock-touch-height) + 1px);
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
  .dock-title[data-compact-dock-title] {
    padding-inline-end: 0.25rem;
  }
  .dock-title[data-session-dock-title] {
    /* Keep icon targets usable when text zoom doubles rem lengths. */
    --control-sm: min(2rem, 32px);
    --control-touch: min(2.75rem, 44px);
    --control-touch-compact: min(2.25rem, 36px);
    padding-inline: 0.25rem;
  }
  .dock-title[data-session-dock-title]:not(:has(> button:first-child)) {
    padding-inline-start: 0.75rem;
  }
  .dock-title[draggable="true"] {
    user-select: none;
    cursor: grab;
  }
</style>
