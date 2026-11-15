<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";
  import type { UploadProgress } from "../../hooks/shared/upload.svelte";

  // The upload strip shown by both editors while a batch is running.
  let {
    progress,
    cancel,
    variant = "strip",
  }: {
    progress: UploadProgress;
    cancel: () => void;
    variant?: "strip" | "card";
  } = $props();
</script>

<div
  class={variant === "card"
    ? "space-y-1 border-t px-2 py-1.5"
    : "shrink-0 space-y-1 border-b bg-muted px-3 py-2"}
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
    <span>{progress.percent}%</span>
    <Button variant="ghost" size="sm" onclick={cancel}>{m.s1_cancel()}</Button>
  </div>
  <progress
    aria-label={m.s1_file_upload_progress()}
    max={100}
    value={progress.percent}
    class="block h-1 w-full accent-primary"
  ></progress>
</div>
