<script lang="ts">
  import IconSearch from "~icons/tabler/search";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import { Button } from "$lib/components/ui/button";
  import * as Empty from "$lib/components/ui/empty";
  import * as m from "$lib/paraglide/messages.js";

  let {
    filtered,
    canStart,
    onclear,
    onstart,
  }: {
    // Filters hide every session, as opposed to there being none at all.
    filtered: boolean;
    canStart: boolean;
    onclear: () => void;
    onstart: () => void;
  } = $props();
</script>

{#if filtered}
  <Empty.Root variant="card" class="min-h-72">
    <Empty.Header>
      <Empty.Media variant="icon"><IconSearch aria-hidden="true" /></Empty.Media
      >
      <Empty.Title>{m.s3_empty_filtered_title()}</Empty.Title>
      <Empty.Description>{m.s3_empty_filtered_desc()}</Empty.Description>
    </Empty.Header>
    <Empty.Content>
      <Button variant="outline" onclick={onclear}>{m.s3_clear_filters()}</Button
      >
    </Empty.Content>
  </Empty.Root>
{:else}
  <Empty.Root variant="card" class="min-h-72">
    <Empty.Header>
      <Empty.Media variant="icon"
        ><IconTerminal2 aria-hidden="true" /></Empty.Media
      >
      <Empty.Title>{m.s3_empty_no_sessions_title()}</Empty.Title>
      <Empty.Description>
        {m.s3_empty_no_sessions_desc()}
      </Empty.Description>
    </Empty.Header>
    <Empty.Content>
      <Button disabled={!canStart} onclick={() => onstart()}
        >{m.s3_plus_new_session()}</Button
      >
    </Empty.Content>
  </Empty.Root>
{/if}
