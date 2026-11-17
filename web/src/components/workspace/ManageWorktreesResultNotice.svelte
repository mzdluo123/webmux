<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { BatchRemoveResult } from "../../contracts/schemas";
  import { InlineNotice } from "$lib/components/ui/inline-notice";

  let {
    removed,
    skipped,
    failed,
  }: {
    removed: number;
    skipped: number;
    failed: BatchRemoveResult["results"];
  } = $props();
</script>

<div class="rounded-lg border bg-muted/40 p-3 text-sm">
  <p class="font-medium">
    {skipped > 0
      ? m.s8_batch_res_summary_with_skipped({
          removed: String(removed),
          skipped: String(skipped),
          failed: String(failed.length),
        })
      : m.s8_batch_res_summary({
          removed: String(removed),
          failed: String(failed.length),
        })}
  </p>
  <p class="mt-1 text-xs text-muted-foreground">
    {m.s8_batch_res_hint()}
  </p>
</div>

<InlineNotice tone="danger">
  <ul class="space-y-1.5">
    {#each failed as item (item.worktreeId)}
      <li class="break-all">
        <span class="font-medium">{item.branch || item.path}</span>
        {#if item.branch}
          <span class="font-mono text-xs"
            >{m.s8_parens_value({ value: item.path })}</span
          >
        {/if}{m.s8_colon_error({
          error: item.error || m.s8_batch_res_incomplete(),
        })}
      </li>
    {/each}
  </ul>
</InlineNotice>
