<script lang="ts">
  import IconSearch from "~icons/tabler/search";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import { Button } from "$lib/components/ui/button";
  import * as Empty from "$lib/components/ui/empty";
  import { m } from "$lib/paraglide/messages.js";

  let {
    filtered,
    description,
    onclear,
    onexpand,
  }: {
    // Filters hide every session, as opposed to there being none at all.
    filtered: boolean;
    description?: string;
    onclear: () => void;
    onexpand?: () => void;
  } = $props();
</script>

<Empty.Root variant="outline" class="min-h-72">
  <Empty.Header>
    <Empty.Media variant="icon">
      {#if filtered}<IconSearch aria-hidden="true" />
      {:else}<IconTerminal2 aria-hidden="true" />{/if}
    </Empty.Media>
    <Empty.Title>
      {filtered ? m.s3_empty_filtered_title() : m.s3_empty_no_sessions_title()}
    </Empty.Title>
    {#if description !== ""}
      <Empty.Description>
        {description ??
          (filtered
            ? m.s3_empty_filtered_desc()
            : m.s3_empty_no_sessions_desc())}
      </Empty.Description>
    {/if}
  </Empty.Header>
  {#if filtered}
    <Empty.Content>
      <div class="flex flex-wrap justify-center gap-2">
        <Button variant="outline" onclick={onclear}
          >{m.s3_clear_filters()}</Button
        >
        {#if onexpand}
          <Button variant="outline" onclick={onexpand}
            >{m.s8_search_all_repositories()}</Button
          >
        {/if}
      </div>
    </Empty.Content>
  {/if}
</Empty.Root>
