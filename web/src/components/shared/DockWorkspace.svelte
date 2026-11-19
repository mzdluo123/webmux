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
    dockDropEdge,
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
  import DockEdgeRail from "./DockEdgeRail.svelte";
  import { useDockPeek } from "../../hooks/shared/dock-peek.svelte";
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
    pickerActions?: Snippet<[() => void]>;
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
  let zoom = $state("");
  let surface = $state<HTMLDivElement>();
  let dragging = $state<string>();
  let hint = $state<{ target: string; edge: DockEdge }>();
  let history = $state.raw<string[]>([]);
  let mobileHidden = $state.raw<string[]>([]);
  let pickerOrigin: HTMLElement | null = null;
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
  const peek = useDockPeek(
    () => edgeRails && !compact,
    (id) => available.has(id) && hidden.includes(id),
    side,
  );
  const preview = $derived.by(() => {
    if (!hint) return undefined;
    const box = geometry.groups.get(hint.target)?.rect;
    if (!box || hint.edge === "center") return box;
    if (hint.edge === "left") return { ...box, width: box.width / 2 };
    if (hint.edge === "right")
      return { ...box, x: box.x + box.width / 2, width: box.width / 2 };
    if (hint.edge === "top") return { ...box, height: box.height / 2 };
    return { ...box, y: box.y + box.height / 2, height: box.height / 2 };
  });

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
    const overlay = peek.rect(id);
    if (overlay) return overlay;
    if (visibleZoom) return visibleZoom === id ? full : undefined;
    return geometry.groups.get(id)?.rect;
  }

  export function isVisible(id: string) {
    return !!rect(id);
  }

  export function hasPanel(id: string) {
    return entries.has(id);
  }

  export function show(id: string) {
    peek.close();
    if (compact) {
      if (mobile && mobile !== id)
        history = [...history.filter((value) => value !== mobile), mobile];
      mobileHidden = mobileHidden.filter((value) => value !== id);
      mobilePanel = id;
    } else {
      const activated = dockActivate(tree, id);
      if (activated !== tree) node = activated;
      if (hidden.includes(id)) hidden = hidden.filter((value) => value !== id);
      if (zoom !== id) zoom = "";
    }
  }

  export function add(
    id: string,
    target: string,
    edge: DockEdge,
    useDefault = false,
  ) {
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
    peek.close();
    node = undefined;
    hidden = [];
    zoom = "";
  }

  export function toggleZoom(id: string) {
    if (!compact && isVisible(id)) zoom = visibleZoom === id ? "" : id;
  }

  function minimize(id: string, destination?: DockRail) {
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
      if (!mobilePanel) showPicker = true;
    } else {
      if (destination) rails[id] = destination;
      peek.close();
      node = tree;
      hidden = [...hidden.filter((value) => value !== id), id];
      if (zoom === id) zoom = "";
    }
  }

  function drag(id: string | undefined) {
    dragging = id;
    hint = undefined;
  }

  function dragOver(event: DragEvent, target: string) {
    if (!dragging || dragging === target || !event.dataTransfer) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect();
    const edge = dockDropEdge(
      (event.clientX - box.left) / box.width,
      (event.clientY - box.top) / box.height,
    );
    if (hint?.target !== target || hint.edge !== edge) hint = { target, edge };
  }

  function move(id: string, target: string, edge: DockEdge) {
    add(id, target, edge);
    zoom = "";
    void tick().then(() =>
      surface
        ?.querySelector<HTMLElement>(
          `[data-dock-panel="${CSS.escape(id)}"] [data-dock-actions]`,
        )
        ?.focus(),
    );
  }

  const previous = $derived(
    history
      .filter(
        (id) =>
          available.has(id) && !mobileHidden.includes(id) && id !== mobile,
      )
      .at(-1),
  );
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
    select: (id) => onSelectPanel?.(id),
    add,
    remove,
    tabs: (id) =>
      (geometry.groups.get(id)?.ids ?? [])
        .map((key) => entries.get(key))
        .filter((entry) => entry !== undefined),
    targets: (id) =>
      [...geometry.groups].flatMap(([key, group]) => {
        const target =
          key === id ? group.ids.find((value) => value !== id) : key;
        const entry = target ? entries.get(target) : undefined;
        return entry ? [entry] : [];
      }),
    compact: () => compact,
    zoomed: (id) => visibleZoom === id,
    show,
    edgeRails: () => edgeRails,
    peeked: (id) => !!peek.rect(id),
    minimize,
    toggleZoom,
    move,
    drag,
    dragOver,
    drop(event, target) {
      dragOver(event, target);
      if (dragging && hint?.target === target && dragging !== target)
        move(dragging, target, hint.edge);
      drag(undefined);
    },
  });

  export function openPicker() {
    pickerOrigin =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    showPicker = true;
  }
  function closePicker() {
    showPicker = false;
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
      {#if preview && !compact}
        <div
          class="dock-preview"
          data-dock-preview
          aria-hidden="true"
          style:--dock-x={`${preview.x * 100}%`}
          style:--dock-y={`${preview.y * 100}%`}
          style:--dock-w={`${preview.width * 100}%`}
          style:--dock-h={`${preview.height * 100}%`}
        ></div>
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
      peek={peek.id}
      toggle={(id, source) => {
        peek.toggle(id, source);
        if (peek.id) onSelectPanel?.(id);
      }}
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
          onSelectPanel?.(entry.id);
          closePicker();
        }}
      >
        <span class="min-w-0 flex-1">{entry.title}</span>
        {#if isVisible(entry.id)}<IconCheck aria-hidden="true" />{/if}
      </ListButton>
    {/each}
    {@render pickerActions?.(closePicker)}
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
  .dock-preview {
    pointer-events: none;
    z-index: 20;
    border: 1px solid var(--primary);
    background: color-mix(in oklab, var(--primary) 20%, transparent);
  }
</style>
