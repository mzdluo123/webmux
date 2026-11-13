<script lang="ts">
  import { onMount } from "svelte";
  import { Button } from "../../src/lib/components/ui/button";
  import * as Command from "../../src/lib/components/ui/command";
  import SegmentedControl from "../../src/lib/components/ui/segmented-control/segmented-control.svelte";
  import SegmentedItem from "../../src/lib/components/ui/segmented-control/segmented-control-item.svelte";
  import { useTvNavigation } from "../../src/hooks/shared/tv-navigation.svelte";
  import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";

  let value = $state("list");
  let choice = $state("none");
  tvMode.enabled = true;
  useTvNavigation(() => false);
  onMount(() => () => {
    tvMode.enabled = false;
  });
</script>

<div
  style="position:fixed;inset:0;display:flex;flex-direction:column;align-items:flex-start;gap:24px;padding:20px"
>
  <Button>上方按钮</Button>
  <SegmentedControl bind:value aria-label="列表显示">
    <SegmentedItem value="list">列表</SegmentedItem>
    <SegmentedItem value="preview">预览</SegmentedItem>
  </SegmentedControl>
  <Button>下方按钮</Button>
  <div style="width:420px;height:220px">
    <Command.Root label="命令">
      <Command.Input aria-label="命令选择" placeholder="命令选择" />
      <Command.List>
        <Command.Item value="first" onSelect={() => (choice = "first")}
          >First command</Command.Item
        >
        <Command.Item value="second" onSelect={() => (choice = "second")}
          >Second command</Command.Item
        >
      </Command.List>
    </Command.Root>
  </div>
  <output aria-label="已选命令">{choice}</output>
</div>
