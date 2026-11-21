<script lang="ts">
  import IconClock from "~icons/tabler/clock";
  import IconTriangle from "~icons/tabler/triangle";
  import {
    rowStatusLabels,
    type RowStatus,
  } from "../../domains/sessions/session-status";
  import { underRowAction } from "../../hooks/shared/sidebar-row";

  // The row's accessible name and tooltip carry the state text. The row
  // menu takes the mark's place while hovered, focused or open.
  let { status }: { status: RowStatus } = $props();
  const tones: Record<Exclude<RowStatus, "released" | "dormant">, string> = {
    working: "bg-success",
    waiting: "bg-warning",
    idle: "bg-muted-foreground/70",
    ended: "border border-muted-foreground/70",
  };
</script>

<span
  class={`flex size-4 shrink-0 items-center justify-end ${underRowAction}`}
  title={rowStatusLabels[status]}
  data-status={status}
  >{#if status === "released"}<IconTriangle
      class="size-2.5 text-muted-foreground/70"
      aria-hidden="true"
    />{:else if status === "dormant"}<IconClock
      class="size-3.5 text-muted-foreground"
      aria-hidden="true"
    />{:else}<span
      class={`size-2 rounded-full ${tones[status]}`}
      aria-hidden="true"
    ></span>{/if}</span
>
