<script lang="ts">
  import type { Snippet } from "svelte";
  import IconAlertTriangle from "~icons/tabler/alert-triangle";
  import IconCircleCheck from "~icons/tabler/circle-check";
  import IconInfoCircle from "~icons/tabler/info-circle";
  import { cn } from "$lib/utils";
  import { tv } from "tailwind-variants";

  let {
    tone = "info",
    size = "default",
    surface = "inline",
    actions,
    children,
    class: className,
  }: {
    tone?: "info" | "success" | "warning" | "danger";
    size?: "default" | "compact";
    surface?: "inline" | "floating";
    actions?: Snippet;
    children: Snippet;
    class?: string;
  } = $props();
  const noticeVariants = tv({
    base: "flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border text-sm leading-relaxed",
    variants: {
      tone: {
        info: "text-info",
        success: "text-success",
        warning: "text-warning",
        danger: "text-destructive",
      },
      size: {
        default: "p-3",
        compact: "px-3 py-1.5",
      },
      surface: {
        inline: "",
        floating: "bg-popover shadow-lg",
      },
    },
    compoundVariants: [
      {
        tone: "info",
        surface: "inline",
        class: "border-info/20 bg-info/5",
      },
      {
        tone: "success",
        surface: "inline",
        class: "border-success/20 bg-success/5",
      },
      {
        tone: "warning",
        surface: "inline",
        class: "border-warning/20 bg-warning/5",
      },
      {
        tone: "danger",
        surface: "inline",
        class: "border-destructive/20 bg-destructive/5",
      },
      {
        tone: "info",
        surface: "floating",
        class: "border-info/35",
      },
      {
        tone: "success",
        surface: "floating",
        class: "border-success/35",
      },
      {
        tone: "warning",
        surface: "floating",
        class: "border-warning/35",
      },
      {
        tone: "danger",
        surface: "floating",
        class: "border-destructive/35",
      },
    ],
  });
</script>

<div
  data-slot="inline-notice"
  data-tone={tone}
  data-surface={surface}
  role={tone === "danger" ? "alert" : "status"}
  class={cn(noticeVariants({ tone, size, surface }), className)}
>
  <div class={["flex min-w-0 flex-1 items-start gap-2", actions && "basis-32"]}>
    <span class="flex h-[1.625em] shrink-0 items-center" aria-hidden="true">
      {#if tone === "danger" || tone === "warning"}
        <IconAlertTriangle class="size-4" />
      {:else if tone === "success"}
        <IconCircleCheck class="size-4" />
      {:else}
        <IconInfoCircle class="size-4" />
      {/if}
    </span>
    <div class="min-w-0 flex-1 wrap-anywhere">
      {@render children()}
    </div>
  </div>
  {#if actions}
    <div
      data-slot="inline-notice-actions"
      class="ml-auto flex max-w-full shrink-0 flex-wrap items-center gap-1.5 text-foreground empty:hidden"
    >
      {@render actions()}
    </div>
  {/if}
</div>
