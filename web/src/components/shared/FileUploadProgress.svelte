<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { cn } from "$lib/utils.js";
  import { m } from "$lib/paraglide/messages.js";
  import type { UploadProgress } from "../../hooks/shared/upload.svelte";

  // The upload strip or floating overlay shown while a batch is running.
  let {
    progress,
    cancel,
    variant = "strip",
    offset = false,
    class: className = "",
  }: {
    progress: UploadProgress;
    cancel: () => void;
    variant?: "strip" | "card" | "floating";
    /** Shifts a floating card below the terminal find bar when open. */
    offset?: boolean;
    class?: string;
  } = $props();
</script>

<div
  class={cn(
    variant === "card" && "space-y-1 border-t px-2 py-1.5",
    variant === "strip" && "shrink-0 space-y-1 border-b bg-muted px-3 py-2",
    variant === "floating" && [
      "absolute inset-x-2 z-20 space-y-1.5 rounded-lg border bg-popover p-2.5 text-popover-foreground shadow-md sm:left-auto sm:w-80",
      offset ? "top-24 max-sm:top-32" : "top-2",
    ],
    className,
  )}
  aria-label={m.s1_file_upload_status()}
>
  <div class="flex items-center gap-3 text-xs" role="status">
    <span class="min-w-0 flex-1 truncate"
      >{progress.waiting
        ? progress.total > 1
          ? m.s1_upload_waiting_terminal_batch({
              name: progress.name,
              index: progress.index,
              total: progress.total,
            })
          : m.s1_upload_waiting_terminal({ name: progress.name })
        : progress.total > 1
          ? m.s1_uploading_file_batch({
              name: progress.name,
              index: progress.index,
              total: progress.total,
            })
          : m.s1_uploading_file({ name: progress.name })}</span
    >
    <span class="shrink-0 tabular-nums">{progress.percent}%</span>
    <Button variant="ghost" size="sm" class="shrink-0" onclick={cancel}
      >{m.s1_cancel()}</Button
    >
  </div>
  <progress
    aria-label={m.s1_file_upload_progress()}
    max={100}
    value={progress.percent}
    class="block h-1 w-full accent-primary"
  ></progress>
</div>
