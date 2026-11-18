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

<div class="grouping @container px-2 pt-1 pb-1.5">
  <SegmentedControl
    size="navigation"
    framed
    aria-label={m.s8_sidebar_grouping()}
    aria-controls={controls}
    value={byBranch ? "branch" : "conversation"}
    onValueChange={(value) => onchange(value === "branch")}
  >
    <SegmentedControlItem value="conversation">
      <IconMessages aria-hidden="true" />
      <span class="min-w-0 wrap-anywhere">{m.s8_sidebar_by_conversation()}</span
      >
    </SegmentedControlItem>
    <SegmentedControlItem value="branch">
      <IconGitBranch aria-hidden="true" />
      <span class="min-w-0 wrap-anywhere">{m.s8_sidebar_by_branch()}</span>
    </SegmentedControlItem>
    <span
      data-slot="grouping-indicator"
      aria-hidden="true"
      class={[
        "pointer-events-none absolute start-0 bottom-0.5 w-1/2 px-3 motion-safe:transition-transform motion-safe:duration-200 motion-safe:ease-out motion-reduce:transition-none",
        byBranch && "translate-x-full rtl:-translate-x-full",
      ]}><span class="block h-0.5 rounded-full bg-foreground/80"></span></span
    >
  </SegmentedControl>
</div>

<style>
  @container (width < 14rem) {
    .grouping :global([data-slot="segmented-control-item"]) {
      padding-inline: 4px;
    }

    .grouping :global([data-slot="segmented-control-item"] > svg) {
      display: none;
    }
  }
</style>
