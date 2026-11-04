<script lang="ts" module>
  import { tv, type VariantProps } from "tailwind-variants";

  // A single-choice group (radio semantics) drawn like the line tabs: an
  // underline marks the chosen item instead of a filled segment.
  export const toggleGroupVariants = tv({
    base: "group/toggle-group inline-flex w-fit items-center justify-center gap-1 p-[3px] text-muted-foreground",
    variants: {
      size: {
        default: "h-8",
        form: "h-[calc(var(--control-default)+6px)]",
      },
      // Items flow onto more rows on narrow screens; each row keeps room for
      // the underline below it.
      wrap: { true: "h-auto flex-wrap justify-start gap-y-2" },
    },
    defaultVariants: { size: "default" },
  });
</script>

<script lang="ts">
  import { ToggleGroup as ToggleGroupPrimitive } from "bits-ui";
  import { cn } from "$lib/utils.js";

  let {
    ref = $bindable(null),
    value = $bindable(""),
    onValueChange,
    size = "default",
    wrap = false,
    class: className,
    ...restProps
  }: Omit<
    Extract<ToggleGroupPrimitive.RootProps, { type: "single" }>,
    "type"
  > & {
    size?: VariantProps<typeof toggleGroupVariants>["size"];
    wrap?: boolean;
  } = $props();
</script>

<!-- Like a radio group, a choice cannot be cleared: pressing the chosen item
     again keeps it. -->
<ToggleGroupPrimitive.Root
  bind:ref
  bind:value={
    () => value,
    (next) => {
      if (!next || next === value) return;
      value = next;
      onValueChange?.(next);
    }
  }
  type="single"
  data-slot="toggle-group"
  data-size={size}
  data-wrap={wrap || undefined}
  class={cn(toggleGroupVariants({ size, wrap }), className)}
  {...restProps}
/>
