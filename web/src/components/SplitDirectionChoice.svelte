<script lang="ts">
  import type { SplitDirection } from "../terminal-layout";
  import * as ToggleGroup from "$lib/components/ui/toggle-group";

  let { direction = $bindable() }: { direction: SplitDirection } = $props();
</script>

<ToggleGroup.Root
  bind:value={
    () => direction,
    (next) => {
      if (next === "horizontal" || next === "vertical") direction = next;
    }
  }
  aria-label="分屏方向"
  size="form"
  class="w-full"
>
  <ToggleGroup.Item value="horizontal">左右分屏</ToggleGroup.Item>
  <ToggleGroup.Item value="vertical">上下分屏</ToggleGroup.Item>
</ToggleGroup.Root>
