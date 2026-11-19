<script lang="ts">
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconMessages from "~icons/tabler/messages";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";

  let {
    byBranch,
    controls,
    onchange,
  }: {
    byBranch: boolean;
    controls?: string;
    onchange: (checked: boolean) => void;
  } = $props();
</script>

<SegmentedControl
  size="header"
  aria-label={m.s8_sidebar_grouping()}
  aria-controls={controls}
  value={byBranch ? "branch" : "conversation"}
  onValueChange={(value) => onchange(value === "branch")}
>
  <SegmentedControlItem
    value="conversation"
    aria-label={m.s8_sidebar_by_conversation()}
    title={m.s8_sidebar_by_conversation()}
  >
    <IconMessages aria-hidden="true" />
  </SegmentedControlItem>
  <SegmentedControlItem
    value="branch"
    aria-label={m.s8_sidebar_by_branch()}
    title={m.s8_sidebar_by_branch()}
  >
    <IconGitBranch aria-hidden="true" />
  </SegmentedControlItem>
  <span
    data-slot="grouping-indicator"
    aria-hidden="true"
    class={[
      "pointer-events-none absolute start-0 top-1/2 h-[max(32px,1.125rem)] w-1/2 -translate-y-1/2 px-0.5 motion-safe:transition-transform motion-safe:duration-200 motion-safe:ease-out motion-reduce:transition-none",
      byBranch && "translate-x-full rtl:-translate-x-full",
    ]}
  >
    <span
      class="block h-full rounded-md bg-foreground/10 ring-1 ring-inset ring-foreground/8"
    ></span>
  </span>
</SegmentedControl>
