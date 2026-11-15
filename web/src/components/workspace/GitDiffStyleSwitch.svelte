<script lang="ts">
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";

  let {
    value = "unified",
    ariaLabel: rawAriaLabel,
    onChange,
  }: {
    value?: "unified" | "split";
    ariaLabel?: string;
    onChange?: (mode: "unified" | "split") => void;
  } = $props();

  const ariaLabel = $derived(rawAriaLabel ?? m.s6_diff_view_mode_aria());
</script>

<SegmentedControl.Root
  {value}
  onValueChange={(val) => {
    if (val === "unified" || val === "split") {
      onChange?.(val);
    }
  }}
  aria-label={ariaLabel}
>
  <SegmentedControl.Item value="unified">
    {m.s6_diff_mode_unified()}
  </SegmentedControl.Item>
  <SegmentedControl.Item value="split">
    {m.s6_diff_mode_split()}
  </SegmentedControl.Item>
</SegmentedControl.Root>
