<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { DockDivider } from "$lib/dock-layout";

  let {
    divider,
    bounds,
    resize,
  }: {
    divider: DockDivider;
    bounds: () => DOMRect | undefined;
    resize: (ratio: number) => void;
  } = $props();
  const horizontal = $derived(divider.direction === "horizontal");

  function move(event: PointerEvent) {
    if (
      !(event.currentTarget as HTMLElement).hasPointerCapture(event.pointerId)
    )
      return;
    const box = bounds();
    if (!box?.width || !box.height) return;
    const { parent } = divider;
    resize(
      horizontal
        ? ((event.clientX - box.left) / box.width - parent.x) / parent.width
        : ((event.clientY - box.top) / box.height - parent.y) / parent.height,
    );
  }

  function key(event: KeyboardEvent) {
    const previous = horizontal ? "ArrowLeft" : "ArrowUp";
    const next = horizontal ? "ArrowRight" : "ArrowDown";
    if (event.key === previous) resize(divider.ratio - 0.05);
    else if (event.key === next) resize(divider.ratio + 0.05);
    else if (event.key === "Home") resize(0.1);
    else if (event.key === "End") resize(0.9);
    else if (event.key === "Enter") resize(0.5);
    else return;
    event.preventDefault();
  }
</script>

<!-- A focusable separator is an interactive window splitter. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
  class="dock-divider"
  class:vertical={horizontal}
  role="separator"
  tabindex="0"
  aria-orientation={horizontal ? "vertical" : "horizontal"}
  aria-label={horizontal ? m.s4_resize_pane_width() : m.s4_resize_pane_height()}
  aria-valuemin={10}
  aria-valuemax={90}
  aria-valuenow={Math.round(divider.ratio * 100)}
  style:--dock-x={`${divider.rect.x * 100}%`}
  style:--dock-y={`${divider.rect.y * 100}%`}
  style:--dock-w={`${divider.rect.width * 100}%`}
  style:--dock-h={`${divider.rect.height * 100}%`}
  onpointerdown={(event) => {
    if (event.button !== 0) return;
    const handle = event.currentTarget;
    handle.setPointerCapture(event.pointerId);
    handle.focus();
    event.preventDefault();
  }}
  onpointermove={move}
  ondblclick={() => resize(0.5)}
  onkeydown={key}
></div>

<style>
  .dock-divider {
    position: absolute;
    z-index: 2;
    inset-inline-start: var(--dock-x);
    inset-block-start: var(--dock-y);
    inline-size: var(--dock-w);
    block-size: 8px;
    transform: translateY(-50%);
    cursor: row-resize;
    touch-action: none;
  }

  .dock-divider.vertical {
    inline-size: 8px;
    block-size: var(--dock-h);
    transform: translateX(-50%);
    cursor: col-resize;
  }

  .dock-divider::after {
    position: absolute;
    inset: 50% 0 auto;
    block-size: 1px;
    background: var(--border);
    content: "";
  }

  .dock-divider.vertical::after {
    inset: 0 auto 0 50%;
    inline-size: 1px;
    block-size: auto;
  }

  .dock-divider:hover::after,
  .dock-divider:focus-visible::after {
    background: var(--ring);
  }
</style>
