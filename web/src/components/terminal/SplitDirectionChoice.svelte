<script lang="ts">
  import type { SplitDirection } from "../../domains/terminal/terminal-layout";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";

  let { direction = $bindable() }: { direction: SplitDirection } = $props();
</script>

<SegmentedControl.Root
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
  <SegmentedControl.Item value="horizontal"
    >{m.s4_split_horizontal()}</SegmentedControl.Item
  >
  <SegmentedControl.Item value="vertical"
    >{m.s4_split_vertical()}</SegmentedControl.Item
  >
</SegmentedControl.Root>
