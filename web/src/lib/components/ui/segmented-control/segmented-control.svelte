<script lang="ts" module>
  import { tv, type VariantProps } from "tailwind-variants";

  // A single-choice radio group drawn like the line tabs: an underline marks
  // the chosen segment instead of a filled one.
  const segmentedControlVariants = tv({
    base: "group/segmented-control inline-flex w-fit items-center justify-center gap-1 p-[3px] text-muted-foreground",
    variants: {
      framed: {
        true: "relative isolate gap-0 rounded-lg border border-border/80 bg-muted/25 p-0",
      },
      size: {
        default: "h-8",
        form: "h-[calc(var(--control-default)+6px)]",
        navigation:
          "min-h-8 w-full items-stretch max-sm:min-h-[calc(var(--control-touch)+8px)] any-pointer-coarse:min-h-[calc(var(--control-touch)+8px)]",
      },
      // Segments flow onto more rows on narrow screens; each row keeps room
      // for the underline below it.
      wrap: { true: "h-auto max-w-full flex-wrap justify-start gap-y-2" },
    },
    defaultVariants: { size: "default" },
  });
</script>

<script lang="ts">
  import { RadioGroup as RadioGroupPrimitive } from "bits-ui";
  import { cn } from "$lib/utils.js";

  let {
    ref = $bindable(null),
    value = $bindable(""),
    size = "default",
    wrap = false,
    framed = false,
    class: className,
    ...restProps
  }: Omit<RadioGroupPrimitive.RootProps, "orientation"> & {
    size?: VariantProps<typeof segmentedControlVariants>["size"];
    wrap?: boolean;
    framed?: boolean;
  } = $props();
</script>

<RadioGroupPrimitive.Root
  bind:ref
  bind:value
  orientation="horizontal"
  data-slot="segmented-control"
  data-size={size}
  data-wrap={wrap || undefined}
  data-framed={framed || undefined}
  class={cn(segmentedControlVariants({ size, wrap, framed }), className)}
  {...restProps}
/>
