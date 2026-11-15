<script lang="ts">
  import { onMount } from "svelte";
  import * as Command from "../../src/lib/components/ui/command";
  import { useTvNavigation } from "../../src/hooks/shared/tv-navigation.svelte";
  import { tvMode } from "../../src/hooks/shared/tv-mode.svelte";
  let selected = $state("");
  tvMode.enabled = true;
  useTvNavigation(() => false);
  onMount(() => () => {
    tvMode.enabled = false;
  });
</script>

<Command.Root>
  <Command.Input aria-label="项目搜索" />
  <Command.List>
    <Command.Item value="project-a" onSelect={() => (selected = "Project A")}
      >Project A</Command.Item
    >
    <Command.Item value="project-b" onSelect={() => (selected = "Project B")}
      >Project B</Command.Item
    >
  </Command.List>
</Command.Root>
<p role="status" aria-label="选择结果">{selected}</p>
