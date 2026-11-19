<script lang="ts">
  import {
    dockDropEdge,
    dockEdges,
    dockGeometry,
    dockGroup,
    dockMove,
    dockRemove,
    dockSplit,
    type DockEdge,
    type DockNode,
    type DockRect,
  } from "$lib/dock-layout";
  import DockDropGuides from "./DockDropGuides.svelte";

  let {
    id,
    tree,
    groups,
    available,
    hidden,
    finish,
  }: {
    id: string;
    tree: DockNode;
    groups: ReadonlyMap<string, { rect: DockRect; ids: string[] }>;
    available: ReadonlySet<string>;
    hidden: readonly string[];
    finish: (next?: DockNode) => void;
  } = $props();
  type Destination = { target?: string; edge: DockEdge };
  let destination = $state.raw<Destination>();
  const workspace = $derived(
    groups.size > 1 || (groups.get(id)?.ids.length ?? 0) > 1,
  );
  const next = $derived.by(() => {
    if (!destination) return;
    const { edge, target } = destination;
    if (target !== undefined) return dockMove(tree, id, target, edge);
    const rest = dockRemove(tree, id);
    const pane = dockGroup(id);
    const first = edge === "left" || edge === "top";
    return dockSplit(
      edge === "left" || edge === "right" ? "horizontal" : "vertical",
      first ? pane : rest,
      first ? rest : pane,
    );
  });
  const preview = $derived(
    next && dockGeometry(next, available, new Set(hidden)).groups.get(id)?.rect,
  );

  function locate(event: DragEvent) {
    const box = (event.currentTarget as HTMLElement).getBoundingClientRect();
    const x = (event.clientX - box.left) / box.width,
      y = (event.clientY - box.top) / box.height;
    const marker =
      event.target instanceof Element
        ? event.target.closest<HTMLElement>("[data-guide-edge]")
        : null;
    const edge = dockEdges.find((value) => value === marker?.dataset.guideEdge);
    if (
      workspace &&
      marker?.dataset.guideScope === "workspace" &&
      edge &&
      edge !== "center"
    )
      return { edge };
    // A guide may extend outside a very small group's bounds. Its visible
    // marker still addresses the group for which that cross was rendered.
    if (edge && marker?.dataset.guideScope === "pane" && destination?.target)
      return { edge, target: destination.target };
    for (const [target, { rect }] of groups) {
      if (
        target !== id &&
        x >= rect.x &&
        x < rect.x + rect.width &&
        y >= rect.y &&
        y < rect.y + rect.height
      ) {
        return {
          target,
          edge: dockDropEdge(
            (x - rect.x) / rect.width,
            (y - rect.y) / rect.height,
          ),
        };
      }
    }
  }
  function update(event: DragEvent) {
    event.preventDefault();
    const hit = locate(event);
    if (hit?.target !== destination?.target || hit?.edge !== destination?.edge)
      destination = hit;
    if (event.dataTransfer)
      event.dataTransfer.dropEffect = hit ? "move" : "none";
  }
</script>

<div
  class="drag-layer"
  role="presentation"
  ondragenter={update}
  ondragover={update}
  ondrop={(event) => {
    update(event);
    finish(next);
  }}
  ondragleave={(event) => {
    if (
      !(event.relatedTarget instanceof Node) ||
      !event.currentTarget.contains(event.relatedTarget)
    )
      destination = undefined;
  }}
>
  {#if preview}
    <div
      data-dock-preview
      aria-hidden="true"
      style:--dock-x={`${preview.x * 100}%`}
      style:--dock-y={`${preview.y * 100}%`}
      style:--dock-w={`${preview.width * 100}%`}
      style:--dock-h={`${preview.height * 100}%`}
    ></div>
  {/if}
  <DockDropGuides
    rect={destination?.target
      ? groups.get(destination.target)?.rect
      : undefined}
    active={destination?.edge}
    outer={!!destination && !destination.target}
    {workspace}
  />
</div>

<style>
  .drag-layer {
    position: absolute;
    inset: 0;
    z-index: 30;
  }
  [data-dock-preview] {
    pointer-events: none;
    border: 1px solid var(--primary);
    background: color-mix(in oklab, var(--primary) 20%, transparent);
  }
</style>
