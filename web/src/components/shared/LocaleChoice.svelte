<script lang="ts">
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import { locale } from "$lib/locale";
  import * as m from "$lib/paraglide/messages.js";

  let {
    size = "default",
  }: {
    size?: "default" | "form";
  } = $props();
</script>

<SegmentedControl.Root
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
  <SegmentedControl.Item value="system"
    >{m.locale_follow_system()}</SegmentedControl.Item
  >
  <SegmentedControl.Item value="zh-CN">简体中文</SegmentedControl.Item>
  <SegmentedControl.Item value="en">English</SegmentedControl.Item>
</SegmentedControl.Root>
