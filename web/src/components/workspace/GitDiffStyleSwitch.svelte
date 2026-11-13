<script lang="ts">
  import * as SegmentedControl from "$lib/components/ui/segmented-control";

  let {
    value = "unified",
    label = "差异视图模式",
    onChange,
  }: {
    value?: "unified" | "split";
    label?: string;
    onChange?: (mode: "unified" | "split") => void;
  } = $props();
</script>

<SegmentedControl.Root
  {value}
  onValueChange={(val) => {
    if (val === "unified" || val === "split") {
      onChange?.(val);
    }
  }}
  aria-label={label}
>
  <SegmentedControl.Item value="unified">统一</SegmentedControl.Item>
  <SegmentedControl.Item value="split">双栏</SegmentedControl.Item>
</SegmentedControl.Root>
