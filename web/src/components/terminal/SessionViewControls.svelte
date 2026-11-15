<script lang="ts">
  import IconList from "~icons/tabler/list";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  let {
    compact,
    onchange,
  }: { compact: boolean; onchange: (compact: boolean) => void } = $props();
</script>

<SegmentedControl.Root
  value={compact ? "compact" : "preview"}
  onValueChange={(value) => onchange(value === "compact")}
  aria-label={m.s4_session_view_mode()}
  class="h-auto min-w-0"
>
  <SegmentedControl.Item
    value="compact"
    class="min-h-11 min-w-11 sm:min-h-8 sm:min-w-0"
    aria-label={m.s4_compact_list()}
    title={m.s4_compact_list()}
  >
    <IconList aria-hidden="true" /><span class="hidden sm:inline"
      >{m.s4_compact_short()}</span
    >
  </SegmentedControl.Item>
  <SegmentedControl.Item
    value="preview"
    class="min-h-11 min-w-11 sm:min-h-8 sm:min-w-0"
    aria-label={m.s4_output_preview()}
    title={m.s4_output_preview()}
  >
    <IconTerminal2 aria-hidden="true" /><span class="hidden sm:inline"
      >{m.s4_preview_short()}</span
    >
  </SegmentedControl.Item>
</SegmentedControl.Root>
