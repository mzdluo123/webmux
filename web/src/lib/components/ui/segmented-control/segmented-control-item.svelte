<script lang="ts">
  import { RadioGroup as RadioGroupPrimitive } from "bits-ui";
  import { cn } from "$lib/utils.js";

  let {
    ref = $bindable(null),
    class: className,
    ...restProps
  }: RadioGroupPrimitive.ItemProps = $props();
</script>

<RadioGroupPrimitive.Item
  bind:ref
  data-slot="segmented-control-item"
  class={cn(
    "relative inline-flex h-[calc(100%_-_1px)] flex-1 items-center justify-center gap-1.5 rounded-md border border-transparent px-1.5 py-0.5 text-xs font-medium whitespace-nowrap text-foreground/60 transition-colors hover:text-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 data-[state=checked]:text-foreground dark:text-muted-foreground dark:hover:text-foreground dark:data-[state=checked]:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
    "after:absolute after:inset-x-0 after:bottom-[-5px] after:h-0.5 after:bg-foreground after:opacity-0 after:transition-opacity data-[state=checked]:after:opacity-100",
    "group-data-[size=form]/segmented-control:px-3 group-data-[size=form]/segmented-control:text-sm group-data-wrap/segmented-control:flex-none",
    "group-data-[size=navigation]/segmented-control:h-auto group-data-[size=navigation]/segmented-control:min-w-0 group-data-[size=navigation]/segmented-control:py-1 group-data-[size=navigation]/segmented-control:text-center group-data-[size=navigation]/segmented-control:whitespace-normal",
    "group-data-[size=header]/segmented-control:z-10 group-data-[size=header]/segmented-control:h-[max(36px,1.25rem)] group-data-[size=header]/segmented-control:min-w-[44px] group-data-[size=header]/segmented-control:gap-(--grouping-icon-gap) group-data-[size=header]/segmented-control:px-(--grouping-inline-padding) group-data-[size=header]/segmented-control:py-0 group-data-[size=header]/segmented-control:text-[length:0.8125rem] group-data-[size=header]/segmented-control:after:hidden group-data-[size=header]/segmented-control:hover:bg-foreground/5 group-data-[size=header]/segmented-control:focus-visible:z-20 max-sm:group-data-[size=header]/segmented-control:h-[max(var(--control-touch),36px,1.25rem)] any-pointer-coarse:group-data-[size=header]/segmented-control:h-[max(var(--control-touch),36px,1.25rem)]",
    className,
  )}
  {...restProps}
/>
