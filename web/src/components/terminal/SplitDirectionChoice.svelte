<script lang="ts">
  import type { SplitDirection } from "../../domains/terminal/terminal-layout";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";

  let { direction = $bindable() }: { direction: SplitDirection } = $props();
</script>

<SegmentedControl
  bind:value={
    () => direction,
    (next) => {
      if (next === "horizontal" || next === "vertical") direction = next;
    }
  }
  aria-label={m.s4_split_direction()}
  size="form"
  class="w-full"
>
  <SegmentedControlItem value="horizontal"
    >{m.s4_split_horizontal()}</SegmentedControlItem
  >
  <SegmentedControlItem value="vertical"
    >{m.s4_split_vertical()}</SegmentedControlItem
  >
</SegmentedControl>
