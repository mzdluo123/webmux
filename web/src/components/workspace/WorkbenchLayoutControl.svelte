<script lang="ts">
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import { workbenchLayout } from "../../domains/workspace/workbench-layout.svelte";
</script>

<SegmentedControl
  aria-label={m.s8_queue_view()}
  value={workbenchLayout.value}
  onValueChange={(value) => {
    if (value === "queue" || value === "workspaces")
      workbenchLayout.value = value;
  }}
>
  <SegmentedControlItem class="min-h-9" value="queue"
    >{m.s8_queue_queue()}</SegmentedControlItem
  >
  <SegmentedControlItem class="min-h-9" value="workspaces"
    >{m.s8_queue_workspaces()}</SegmentedControlItem
  >
</SegmentedControl>
