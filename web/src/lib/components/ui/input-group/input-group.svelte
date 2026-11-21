<script lang="ts" module>
  import { tv, type VariantProps } from "tailwind-variants";

  const inputGroupVariants = tv({
    base: "group/input-group relative flex h-(--control-default) w-full min-w-0 items-center outline-none transition-colors has-[>textarea]:h-auto has-[>[data-align=block-end]]:h-auto has-[>[data-align=block-end]]:flex-col has-[>[data-align=block-start]]:h-auto has-[>[data-align=block-start]]:flex-col has-[>[data-align=block-end]]:[&>input]:pt-3 has-[>[data-align=block-start]]:[&>input]:pb-3 has-[>[data-align=inline-end]]:[&>input]:pr-1.5",
    variants: {
      variant: {
        default:
          "rounded-md border border-input bg-card dark:bg-input/30 has-[[data-slot=input-group-control]:focus-visible]:border-ring has-[[data-slot=input-group-control]:focus-visible]:ring-ring/50 has-[[data-slot][aria-invalid=true]]:ring-destructive/20 has-[[data-slot][aria-invalid=true]]:border-destructive dark:has-[[data-slot][aria-invalid=true]]:ring-destructive/40 has-disabled:bg-input/50 dark:has-disabled:bg-input/80 in-data-[slot=combobox-content]:focus-within:border-inherit in-data-[slot=combobox-content]:focus-within:ring-0 has-disabled:opacity-50 has-[[data-slot=input-group-control]:focus-visible]:ring-2 has-[[data-slot][aria-invalid=true]]:ring-1 has-[>[data-align=inline-start]]:[&>input]:pl-1.5",
        "list-search":
          "max-sm:h-(--control-touch) any-pointer-coarse:h-(--control-touch) shrink-0 border-b border-border focus-within:border-ring focus-within:inset-shadow-[0_-1px_0_var(--ring)] forced-colors:focus-within:border-b-2 has-[>input[aria-invalid=true]]:border-destructive has-[>[data-align=inline-start]]:[&>input]:pl-2 *:data-[slot=input-group-addon]:pl-3",
      },
    },
    defaultVariants: { variant: "default" },
  });
</script>

<script lang="ts">
  import { cn, type WithElementRef } from "$lib/utils.js";
  import type { HTMLAttributes } from "svelte/elements";

  let {
    ref = $bindable(null),
    class: className,
    children,
    variant,
    ...props
  }: WithElementRef<HTMLAttributes<HTMLDivElement>> &
    VariantProps<typeof inputGroupVariants> = $props();
</script>

<div
  bind:this={ref}
  data-slot="input-group"
  role="group"
  class={cn(inputGroupVariants({ variant }), className)}
  {...props}
>
  {@render children?.()}
</div>
