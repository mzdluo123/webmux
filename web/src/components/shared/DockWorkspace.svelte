<script lang="ts">
  import { tick, untrack, type Snippet } from "svelte";
  import { SvelteMap } from "svelte/reactivity";
  import IconCheck from "~icons/tabler/check";
  import IconLayout from "~icons/tabler/layout-columns";
  import { Button } from "$lib/components/ui/button";
  import * as Dialog from "$lib/components/ui/dialog";
  import * as Drawer from "$lib/components/ui/drawer";
  import { ListButton } from "$lib/components/ui/list-button";
  import { m } from "$lib/paraglide/messages.js";
  import {
    dockActivate,
    dockFill,
    dockGeometry,
    dockIds,
    dockMove,
    dockRemove,
    dockResize,
    restoreDock,
    type DockEdge,
    type DockNode,
    type DockRect,
  } from "$lib/dock-layout";
  import { store, stored } from "../../platform/storage";
  import { mountDockHost, setDock, type DockEntry } from "./dock-context";
  import DockDragLayer from "./DockDragLayer.svelte";
  import DockEdgeRail from "./DockEdgeRail.svelte";
  import type { DockRail } from "$lib/dock-layout";
  import DockSplitter from "./DockDivider.svelte";

  let {
    initial,
    storageKey,
    migrate,
    edgeRails = false,
    compact = false,
    flow = false,
    mobilePanel = $bindable(""),
    showPicker = $bindable(false),
    pickerTrigger = true,
    pickerActions,
    onSelectPanel,
    children,
  }: {
    initial: DockNode;
    storageKey: string;
    edgeRails?: boolean;
    migrate?: () => ReturnType<typeof restoreDock>;
    compact?: boolean;
    flow?: boolean;
    mobilePanel?: string;
    showPicker?: boolean;
    pickerTrigger?: boolean;
    pickerActions?: Snippet<
      [() => void, DockEntry | undefined, (() => void) | undefined]
    >;
    onSelectPanel?: (id: string) => void;
    children: Snippet;
  } = $props();

  const key = untrack(() => `dock.${storageKey}`);
  const saved = untrack(() => {
    const raw = stored(key);
    return raw ? restoreDock(raw) : migrate?.();
  });
  let node = $state.raw<DockNode | undefined>(saved?.node);
  let hidden = $state.raw<string[]>(saved?.hidden ?? []);
  let rails = $state<Record<string, DockRail>>(saved?.rails ?? {});
  const side = (id: string): DockRail =>
    rails[id] ??
    (id.startsWith("files:")
      ? "left"
      : id.startsWith("reader:")
        ? "right"
        : "bottom");
  let selected = $state("");
  let zoom = $state("");
  let surface = $state<HTMLDivElement>();
  let dragging = $state<string>();
  let history = $state.raw<string[]>([]);
  let mobileHidden = $state.raw<string[]>([]);
  let pickerOrigin: HTMLElement | null = null;
  let pickerPanel = $state.raw<DockEntry>();
  const entries = new SvelteMap<string, DockEntry>();
  let pickerEntries = $state.raw<DockEntry[]>([]);
  // A picker action can register panels while the drawer is sliding away.
  // Keep its last list during dismissal so the sheet stays the same height.
  $effect.pre(() => {
    if (showPicker) pickerEntries = [...entries.values()];
  });
  const available = $derived(new Set(entries.keys()));
  const tree = $derived(node ? dockFill(node, initial, available) : initial);
  const geometry = $derived(dockGeometry(tree, available, new Set(hidden)));
  const visibleZoom = $derived(
    available.has(zoom) && !hidden.includes(zoom) ? zoom : "",
  );
  const mobile = $derived(
    [mobilePanel, ...dockIds(initial), ...available].find(
      (id) => available.has(id) && !mobileHidden.includes(id),
    ),
  );
  const full: DockRect = { x: 0, y: 0, width: 1, height: 1 };
  const minimized = $derived(
    hidden.map((id) => entries.get(id)).filter((entry) => entry !== undefined),
  );

  $effect(() => {
    store(
      key,
      node
        ? JSON.stringify({
            version: 1,
            node,
            hidden,
            rails: edgeRails ? rails : undefined,
          })
        : null,
    );
  });

  export function rect(id: string): DockRect | undefined {
    if (compact) return mobile === id ? full : undefined;
    if (visibleZoom) return visibleZoom === id ? full : undefined;
    return geometry.groups.get(id)?.rect;
  }

  const activePanel = $derived(
    visibleZoom ||
      (geometry.groups.has(selected)
        ? selected
        : geometry.groups.keys().next().value),
  );

  export function commandTarget() {
    return activePanel ?? "";
  }

  export function activeCommands() {
    return !compact && activePanel ? entries.get(activePanel) : undefined;
  }
  export function commandPanel(id: string) {
    const panel = entries.get(id);
    if (!edgeRails || compact || !panel || !rect(id)) return;
    const zoomed = visibleZoom === id;
    const destinations = zoomed ? [] : targets(id);
    return { panel, zoomed, targets: destinations };
  }

  export function isVisible(id: string) {
    return !!rect(id);
  }

  export function hasPanel(id: string) {
    return entries.has(id);
  }

  export function show(id: string) {
    selected = id;
    if (compact) {
      if (mobile && mobile !== id)
        history = [...history.filter((value) => value !== mobile), mobile];
      mobileHidden = mobileHidden.filter((value) => value !== id);
      mobilePanel = id;
    } else {
      const keepZoom =
        edgeRails && geometry.groups.get(visibleZoom)?.ids.includes(id);
      const activated = dockActivate(tree, id);
      if (activated !== tree) node = activated;
      if (hidden.includes(id)) hidden = hidden.filter((value) => value !== id);
      if (zoom !== id) zoom = keepZoom ? id : "";
    }
    onSelectPanel?.(id);
  }

  export function add(
    id: string,
    target: string,
    edge: DockEdge,
    useDefault = false,
  ) {
    selected = id;
    if (!useDefault || node) node = dockMove(tree, id, target, edge);
    if (compact) show(id);
  }

  export function remove(id: string) {
    delete rails[id];
    node = dockRemove(node ?? tree, id);
    hidden = hidden.filter((value) => value !== id);
    mobileHidden = mobileHidden.filter((value) => value !== id);
    history = history.filter((value) => value !== id);
    if (zoom === id) zoom = "";
    if (mobilePanel === id) mobilePanel = "";
  }

  export function reset() {
    rails = {};
    node = undefined;
    hidden = [];
    zoom = "";
  }

  export function toggleZoom(id: string) {
    if (!compact && isVisible(id)) zoom = visibleZoom === id ? "" : id;
  }

  export function minimize(id: string, destination?: DockRail) {
    if (compact) {
      mobileHidden = [...mobileHidden.filter((value) => value !== id), id];
      mobilePanel =
        history
          .filter(
            (value) => available.has(value) && !mobileHidden.includes(value),
          )
          .at(-1) ?? "";
      history = history.filter(
        (value) => value !== id && value !== mobilePanel,
      );
      if (!mobilePanel && (!edgeRails || !mobile)) {
        pickerPanel = undefined;
        showPicker = true;
      }
      if (edgeRails && mobile) onSelectPanel?.(mobile);
    } else {
      if (destination) rails[id] = destination;
      node = tree;
      hidden = [...hidden.filter((value) => value !== id), id];
      if (zoom === id) zoom = "";
    }
  }

  export function move(id: string, target: string, edge: DockEdge) {
    add(id, target, edge);
    zoom = "";
    if (edgeRails) return;
    void tick().then(() =>
      surface
        ?.querySelector<HTMLElement>(
          `[data-dock-panel="${CSS.escape(id)}"] [data-dock-actions]`,
        )
        ?.focus(),
    );
  }

  function targets(id: string) {
    return [...geometry.groups].flatMap(([key, group]) => {
      const target = key === id ? group.ids.find((value) => value !== id) : key;
      const entry = target ? entries.get(target) : undefined;
      return entry ? [entry] : [];
    });
  }

  const previous = $derived.by(() => {
    for (let i = history.length - 1; i >= 0; i--) {
      const id = history[i]!;
      if (id !== mobile && available.has(id) && !mobileHidden.includes(id))
        return id;
    }
  });
  setDock({
    canGoBack: () => compact && !!previous,
    back() {
      if (!previous) return;
      const id = previous;
      history = history.slice(0, history.lastIndexOf(id));
      mobilePanel = id;
      onSelectPanel?.(id);
    },
    register(entry) {
      entries.set(entry.id, entry);
      return () => {
        if (entries.get(entry.id) === entry) entries.delete(entry.id);
      };
    },
    mount: (element) => mountDockHost(element, () => surface),
    rect,
    active: (id) => !compact && activePanel === id,
    select(id) {
      selected = id;
      onSelectPanel?.(id);
    },
    add,
    remove,
    tabs: (id) =>
      (geometry.groups.get(id)?.ids ?? [])
        .map((key) => entries.get(key))
        .filter((entry) => entry !== undefined),
    targets,
    compact: () => compact,
    zoomed: (id) => visibleZoom === id,
    show,
    edgeRails: () => edgeRails,
    minimize,
    toggleZoom,
    move,
    drag: (id) => (dragging = id),
  });

  export function openPicker() {
    if (showPicker) return;
    pickerPanel =
      compact && edgeRails && mobile ? entries.get(mobile) : undefined;
    pickerOrigin =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    showPicker = true;
  }
  function closePicker() {
    showPicker = false;
  }

  function hidePickedPanel() {
    const id = pickerPanel?.id;
    if (!compact || !id || mobile !== id) return;
    closePicker();
    minimize(id);
  }
</script>

<div
  class="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
  data-dock-workspace
  data-dock-flow={flow || undefined}
>
  {#if pickerTrigger || (!edgeRails && !compact && minimized.length)}
    <div class="dock-shelf">
      {#if pickerTrigger}
        <Button size="sm-touch" variant="ghost" onclick={openPicker}>
          <IconLayout aria-hidden="true" />
          {m.s4_open_panel()}
        </Button>
      {/if}
      {#if !edgeRails && !compact}
        {#each minimized as entry (entry.id)}
          <Button size="sm" variant="ghost" onclick={() => show(entry.id)}>
            {entry.title}
          </Button>
        {/each}
      {/if}
    </div>
  {/if}
  <div
    class={edgeRails && !compact ? "flex min-h-0 min-w-0 flex-1" : "contents"}
  >
    {@render rail("left")}
    <div
      class="relative flex min-h-0 min-w-0 flex-1 flex-col"
      bind:this={surface}
    >
      {#if !compact && !visibleZoom}
        {#each geometry.dividers as divider (divider.path)}
          <DockSplitter
            {divider}
            bounds={() => surface?.getBoundingClientRect()}
            resize={(ratio) => (node = dockResize(tree, divider.path, ratio))}
          />
        {/each}
      {/if}
      {#if dragging && !compact}
        <DockDragLayer
          id={dragging}
          {tree}
          groups={geometry.groups}
          {available}
          {hidden}
          finish={(next) => {
            if (next) node = next;
            dragging = undefined;
          }}
        />
      {/if}
    </div>
    {@render rail("right")}
  </div>
  {@render rail("bottom")}
  {@render children()}
</div>

{#snippet rail(position: DockRail)}
  {#if edgeRails && !compact}
    <DockEdgeRail
      side={position}
      entries={minimized.filter((entry) => side(entry.id) === position)}
      restore={show}
    />
  {/if}
{/snippet}

{#snippet choices()}
  <div class="min-h-0 space-y-1 overflow-y-auto px-3 pb-3">
    {#each pickerEntries as entry (entry.id)}
      <ListButton
        variant="action-sheet"
        data-dock-picker={entry.id}
        aria-current={isVisible(entry.id) ? "true" : undefined}
        onclick={() => {
          show(entry.id);
          closePicker();
        }}
      >
        <span class="min-w-0 flex-1">{entry.title}</span>
        {#if isVisible(entry.id)}<IconCheck aria-hidden="true" />{/if}
      </ListButton>
    {/each}
    {@render pickerActions?.(
      closePicker,
      compact ? pickerPanel : undefined,
      compact && pickerPanel && (!showPicker || mobile === pickerPanel.id)
        ? hidePickedPanel
        : undefined,
    )}
  </div>
{/snippet}

{#if !compact && !pickerTrigger}
  <Dialog.Root bind:open={showPicker}
    ><Dialog.Content
      onCloseAutoFocus={(event) => {
        if (!pickerOrigin?.isConnected) return;
        event.preventDefault();
        pickerOrigin.focus();
        pickerOrigin = null;
      }}
      ><Dialog.Header
        ><Dialog.Title>{m.s4_open_panel()}</Dialog.Title></Dialog.Header
      >{@render choices()}<Dialog.Footer
        ><Button variant="outline" onclick={closePicker}>{m.s1_cancel()}</Button
        ></Dialog.Footer
      ></Dialog.Content
    ></Dialog.Root
  >
{:else}
  <Drawer.Root bind:open={showPicker}>
    <Drawer.Content
      onCloseAutoFocus={(event) => {
        if (!pickerOrigin?.isConnected) return;
        event.preventDefault();
        pickerOrigin.focus();
        pickerOrigin = null;
      }}
    >
      <Drawer.Header>
        <Drawer.Title>{m.s4_open_panel()}</Drawer.Title>
      </Drawer.Header>
      {@render choices()}
      <Drawer.Footer>
        <Button variant="outline" onclick={closePicker}>{m.s1_cancel()}</Button>
      </Drawer.Footer>
    </Drawer.Content>
  </Drawer.Root>
{/if}

<style>
  [data-dock-flow="true"] {
    height: auto;
    overflow: visible;
  }

  [data-dock-flow="true"] :global(.dock-panel:not([hidden])) {
    position: relative;
    inset: auto;
    height: auto;
    flex: 1;
    overflow: visible;
  }

  [data-dock-flow="true"] :global([data-dock-body]) {
    overflow: visible;
  }

  .dock-shelf {
    display: flex;
    flex: none;
    align-items: center;
    overflow-x: auto;
    border-block-end: 1px solid var(--border);
  }
</style>
