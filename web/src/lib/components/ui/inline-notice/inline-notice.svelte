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
    children,
    class: className,
  }: {
    tone?: "info" | "success" | "warning" | "danger";
    size?: "default" | "compact";
    children: Snippet;
    class?: string;
  } = $props();
  const noticeVariants = tv({
    base: "flex min-w-0 items-start gap-2 rounded-lg border text-sm leading-relaxed",
    variants: {
      tone: {
        info: "border-info/20 bg-info/5 text-info",
        success: "border-success/20 bg-success/5 text-success",
        warning: "border-warning/20 bg-warning/5 text-warning",
        danger: "border-destructive/20 bg-destructive/5 text-destructive",
      },
      size: {
        default: "p-3",
        compact: "px-3 py-1.5",
      },
    },
  });
</script>

<div
  data-slot="inline-notice"
  data-tone={tone}
  role={tone === "danger" ? "alert" : "status"}
  class={cn(noticeVariants({ tone, size }), className)}
>
  {#if tone === "danger" || tone === "warning"}
    <IconAlertTriangle class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
  {:else if tone === "success"}
    <IconCircleCheck class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
  {:else}
    <IconInfoCircle class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
  {/if}
  <div class="min-w-0 flex-1 wrap-anywhere">
    {@render children()}
  </div>
</div>
