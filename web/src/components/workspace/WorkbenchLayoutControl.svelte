<script lang="ts">
  import * as Select from "$lib/components/ui/select";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import { m } from "$lib/paraglide/messages.js";
  import { homeSessionView } from "../../domains/sessions/home-session-view.svelte";
  import { workbenchLayout } from "../../domains/workspace/workbench-layout.svelte";

  const layouts = $derived([
    { value: "queue", label: m.s8_queue_queue() },
    { value: "workspaces", label: m.s8_queue_workspaces() },
  ]);
  const previewId = $props.id();
</script>

<div class="flex flex-wrap items-center gap-x-5 gap-y-2">
  <Select.Root
    type="single"
    value={workbenchLayout.value}
    items={layouts}
    onValueChange={(value) => {
      if (value === "queue" || value === "workspaces")
        workbenchLayout.value = value;
    }}
  >
    <Select.Trigger class="min-h-9 w-fit" aria-label={m.s8_queue_view()}>
      {workbenchLayout.value === "queue"
        ? m.s8_queue_queue()
        : m.s8_queue_workspaces()}
    </Select.Trigger>
    <Select.Content>
      {#each layouts as layout (layout.value)}
        <Select.Item value={layout.value} label={layout.label}>
          {layout.label}
        </Select.Item>
      {/each}
    </Select.Content>
  </Select.Root>

  {#if workbenchLayout.value === "workspaces"}
    <div class="flex min-h-9 items-center gap-2">
      <Switch
        id={previewId}
        checked={!homeSessionView.compact}
        onCheckedChange={(checked) => (homeSessionView.compact = !checked)}
      />
      <Label for={previewId}>{m.s8_show_output_preview()}</Label>
    </div>
  {/if}
</div>
