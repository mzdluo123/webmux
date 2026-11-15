<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import type { ArchivedSession } from "../../contracts/schemas";

  // What an interruption may have cost a closed or interrupted hybrid
  // session: an unfinished turn and the prompts its queue lost.
  let { record }: { record: ArchivedSession } = $props();
</script>

{#if record.state === "interrupted" && (record.archive.activity === "turn" || record.archive.activity === "switching")}
  <p class="text-xs text-warning">
    {record.archive.activity === "turn"
      ? m.s3_record_interrupted_in_turn()
      : m.s3_record_interrupted_switching()}
  </p>
{/if}
{#if record.archive.queuedCount}
  <p class="text-xs text-muted-foreground">
    {m.s3_record_interrupted_queue_lost({
      count: String(record.archive.queuedCount),
    })}
  </p>
{/if}
