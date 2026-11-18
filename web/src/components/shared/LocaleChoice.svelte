<script lang="ts">
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { locale } from "$lib/locale";
  import { m } from "$lib/paraglide/messages.js";

  let {
    size = "default",
  }: {
    size?: "default" | "form";
  } = $props();
</script>

<SegmentedControl
  value={locale.preference}
  onValueChange={(value) => {
    if (value === "system" || value === "zh-CN" || value === "en") {
      locale.setPreference(value);
    }
  }}
  aria-label={m.locale_label()}
  {size}
  wrap
>
  <SegmentedControlItem value="system"
    >{m.locale_follow_system()}</SegmentedControlItem
  >
  <SegmentedControlItem value="zh-CN">简体中文</SegmentedControlItem>
  <SegmentedControlItem value="en">English</SegmentedControlItem>
</SegmentedControl>
