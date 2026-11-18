<script lang="ts">
  import * as Item from "$lib/components/ui/item";
  import { m } from "$lib/paraglide/messages.js";
  import { recordTitle } from "../../domains/sessions/archived";
  import type { ArchivedSession } from "../../contracts/schemas";
  import HybridRecordHints from "./HybridRecordHints.svelte";

  let { records }: { records: ArchivedSession[] } = $props();
</script>

{#if records.length}
  <Item.Group aria-label={m.sessions_resume_on_open()}>
    {#each records as record (record.id)}
      <Item.Root variant="divided">
        {#snippet child({ props })}
          <a
            {...props}
            href={`/s/${record.id}`}
            aria-label={recordTitle(record)}
          >
            <Item.Content class="min-w-0">
              <Item.Title>{recordTitle(record)}</Item.Title>
              <Item.Description class="truncate">
                {record.agent.title} · {record.cwd}
              </Item.Description>
              <HybridRecordHints {record} />
            </Item.Content>
            <Item.Description>{m.sessions_resume_on_open()}</Item.Description>
          </a>
        {/snippet}
      </Item.Root>
    {/each}
  </Item.Group>
{/if}
