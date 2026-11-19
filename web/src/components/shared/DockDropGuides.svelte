<script lang="ts">
  import { dockEdges, type DockEdge, type DockRect } from "$lib/dock-layout";

  let {
    rect,
    active,
    outer = false,
    workspace,
  }: {
    rect?: DockRect;
    active?: DockEdge;
    outer?: boolean;
    workspace: boolean;
  } = $props();
</script>

{#snippet marker(edge: DockEdge, scope: "workspace" | "pane")}
  <div
    class="guide-marker"
    class:active={active === edge && outer === (scope === "workspace")}
    data-guide-edge={edge}
    data-guide-scope={scope}
  >
    <span class="guide-diagram"></span>
  </div>
{/snippet}

<div class="guide-layer" aria-hidden="true">
  {#if rect}
    <div
      class="guide-pane"
      style:--guide-x={`${rect.x * 100}%`}
      style:--guide-y={`${rect.y * 100}%`}
      style:--guide-width={`${rect.width * 100}%`}
      style:--guide-height={`${rect.height * 100}%`}
    >
      <div class="guide-layout guide-cross">
        {#each dockEdges as edge (edge)}
          {@render marker(edge, "pane")}
        {/each}
      </div>
    </div>
  {/if}
  {#if workspace}
    <div class="guide-layout guide-workspace">
      {#each dockEdges as edge (edge)}
        {#if edge !== "center"}
          {@render marker(edge, "workspace")}
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .guide-layer,
  .guide-layout {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }
  .guide-layer,
  .guide-pane {
    container-type: size;
  }
  .guide-pane {
    position: absolute;
    top: var(--guide-y);
    left: var(--guide-x);
    width: var(--guide-width);
    height: var(--guide-height);
  }
  .guide-layout {
    display: grid;
    direction: ltr;
    grid-template-areas:
      ". top ."
      "left center right"
      ". bottom .";
    place-items: center;
  }
  .guide-cross {
    --guide-size: min(var(--control-sm), 30cqw, 30cqh);
    gap: min(0.25rem, 4cqw, 4cqh);
    place-content: center;
  }
  .guide-workspace {
    --guide-size: min(var(--control-default), 30cqw, 30cqh);
    grid-template-columns: auto 1fr auto;
    grid-template-rows: auto 1fr auto;
    padding: min(0.75rem, 4cqw, 4cqh);
  }
  .guide-marker {
    display: grid;
    place-items: center;
    width: var(--guide-size);
    height: var(--guide-size);
    border: 1px solid var(--muted-foreground);
    border-radius: var(--radius-md);
    background: var(--popover);
    color: var(--popover-foreground);
    pointer-events: auto;
  }
  .guide-marker.active {
    border-color: var(--primary);
    outline: 1px solid var(--primary);
    background: var(--primary);
    color: var(--primary-foreground);
  }
  .guide-diagram {
    position: relative;
    width: 58%;
    height: 46%;
    border: 1px solid currentColor;
    border-radius: var(--radius-sm);
    overflow: hidden;
    pointer-events: none;
  }
  .guide-diagram::before {
    position: absolute;
    inset: var(--guide-fill);
    background: currentColor;
    opacity: 0.45;
    content: "";
  }
  [data-guide-edge="left"] {
    grid-area: left;
    --guide-fill: 0 50% 0 0;
  }
  [data-guide-edge="right"] {
    grid-area: right;
    --guide-fill: 0 0 0 50%;
  }
  [data-guide-edge="top"] {
    grid-area: top;
    --guide-fill: 0 0 50%;
  }
  [data-guide-edge="bottom"] {
    grid-area: bottom;
    --guide-fill: 50% 0 0;
  }
  [data-guide-edge="center"] {
    grid-area: center;
    --guide-fill: 0 50% 58% 0;
  }
  [data-guide-edge="center"] .guide-diagram::after {
    position: absolute;
    inset: 42% 0 auto;
    border-top: 1px solid currentColor;
    content: "";
  }
</style>
