<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";

  // Added and removed line counts of a change.
  let { added, removed }: { added: number; removed: number } = $props();
</script>

<span class="shrink-0 font-mono text-xs tabular-nums"
  ><span class="sr-only"
    >{m.s3_diff_count_sr({
      added: String(added),
      removed: String(removed),
    })}</span
  ><span class="text-success" aria-hidden="true">+{added}</span>
  <span class="text-destructive" aria-hidden="true">−{removed}</span></span
>
