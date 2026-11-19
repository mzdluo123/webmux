<script lang="ts">
  import IconArrowsMaximize from "~icons/tabler/arrows-maximize";
  import IconArrowsMinimize from "~icons/tabler/arrows-minimize";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconMinus from "~icons/tabler/minus";
  import IconPin from "~icons/tabler/pin";
  import { Button } from "$lib/components/ui/button";
  import * as Menu from "$lib/components/ui/dropdown-menu";
  import { dockEdges, type DockEdge, type DockRail } from "$lib/dock-layout";
  import { m } from "$lib/paraglide/messages.js";
  import type DockWorkspace from "../shared/DockWorkspace.svelte";

  let { dock }: { dock: DockWorkspace | undefined } = $props();
  let target = $state("");
  const pane = $derived(dock?.commandPanel(target));
  const rails = ["left", "right", "bottom"] as const;
  const railLabel = {
    left: m.dock_minimize_left,
    right: m.dock_minimize_right,
    bottom: m.dock_minimize_bottom,
  };
  const edgeLabel = {
    left: m.dock_left,
    right: m.dock_right,
    top: m.dock_top,
    bottom: m.dock_bottom,
    center: m.dock_center,
  };

  function toggle() {
    if (!dock || !pane) return;
    if (pane.peeked) dock.show(target);
    else dock.toggleZoom(target);
  }

  function minimize(rail?: DockRail) {
    if (dock && pane) dock.minimize(target, rail);
  }

  function move(destination: string, edge: DockEdge) {
    if (dock && pane?.targets.some(({ id }) => id === destination))
      dock.move(target, destination, edge);
  }
</script>

<Menu.Root
  onOpenChange={(open) => {
    if (open) target = dock?.commandTarget() ?? "";
  }}
>
  <Menu.Trigger>
    {#snippet child({ props })}
      <Button
        {...props}
        variant="ghost"
        size="sm"
        class="max-sm:hidden"
        data-dock-command-menu
      >
        {m.s4_panes_group_aria()}
        <IconChevronDown aria-hidden="true" />
      </Button>
    {/snippet}
  </Menu.Trigger>
  <Menu.Content align="start" class="w-56">
    {#if pane}
      {@const title = pane.panel.title}
      <Menu.Label variant="title">
        <span class="wrap-anywhere">{title}</span>
      </Menu.Label>
      <Menu.Separator />
      <Menu.Item onSelect={toggle}>
        {#if pane.peeked}
          <IconPin aria-hidden="true" />
          <span class="min-w-0 wrap-anywhere">{m.dock_pin({ title })}</span>
        {:else if pane.zoomed}
          <IconArrowsMinimize aria-hidden="true" />
          <span class="min-w-0 wrap-anywhere">{m.dock_restore({ title })}</span>
        {:else}
          <IconArrowsMaximize aria-hidden="true" />
          <span class="min-w-0 wrap-anywhere">{m.dock_maximize({ title })}</span
          >
        {/if}
      </Menu.Item>
      <Menu.Item onSelect={() => minimize()}>
        <IconMinus aria-hidden="true" />
        <span class="min-w-0 wrap-anywhere">{m.dock_minimize({ title })}</span>
      </Menu.Item>
      {#if !pane.peeked}
        <Menu.Sub>
          <Menu.SubTrigger>{m.dock_minimize_to_rail()}</Menu.SubTrigger>
          <Menu.SubContent collisionPadding={8}>
            {#each rails as rail (rail)}
              <Menu.Item onSelect={() => minimize(rail)}
                >{railLabel[rail]()}</Menu.Item
              >
            {/each}
          </Menu.SubContent>
        </Menu.Sub>
      {/if}
      {#if pane.targets.length}
        <Menu.Sub>
          <Menu.SubTrigger>{m.dock_move_or_merge()}</Menu.SubTrigger>
          <Menu.SubContent collisionPadding={8} class="max-w-64">
            {#each pane.targets as destination (destination.id)}
              <Menu.Sub>
                <Menu.SubTrigger>
                  <span class="min-w-0 wrap-anywhere">{destination.title}</span>
                </Menu.SubTrigger>
                <Menu.SubContent collisionPadding={8}>
                  {#each dockEdges as edge (edge)}
                    <Menu.Item onSelect={() => move(destination.id, edge)}
                      >{edgeLabel[edge]()}</Menu.Item
                    >
                  {/each}
                </Menu.SubContent>
              </Menu.Sub>
            {/each}
          </Menu.SubContent>
        </Menu.Sub>
      {/if}
    {:else}
      <Menu.Item disabled>{m.dock_no_active_panel()}</Menu.Item>
    {/if}
  </Menu.Content>
</Menu.Root>
