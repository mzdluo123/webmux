<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconHash from "~icons/tabler/hash";
  import IconPlus from "~icons/tabler/plus";
  import IconSearch from "~icons/tabler/search";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as InputGroup from "$lib/components/ui/input-group";
  import * as Empty from "$lib/components/ui/empty";
  import type { MessagingController } from "../../hooks/messaging/messaging.svelte";
  let { messaging }: { messaging: MessagingController } = $props();
  const view = $derived(messaging.state);
  const channels = $derived(
    view.channels.filter((item) =>
      `${item.name} ${item.topic}`
        .toLowerCase()
        .includes(view.query.toLowerCase()),
    ),
  );
</script>

<div class="flex min-h-0 flex-1 flex-col">
  <div class="flex items-center justify-between gap-2 px-4 py-3">
    <h2 class="font-semibold">{m.messaging_channels()}</h2>
    <Button
      variant="ghost"
      size="icon-touch"
      aria-label={m.messaging_new_channel()}
      title={m.messaging_new_channel()}
      disabled={view.loading || view.sending || view.creating}
      onclick={() => (view.createOpen = true)}
      ><IconPlus aria-hidden="true" /></Button
    >
  </div>
  <div class="px-4 pb-3">
    <InputGroup.Root>
      <InputGroup.Input
        aria-label={m.messaging_search()}
        placeholder={m.messaging_search()}
        bind:value={view.query}
      />
      <InputGroup.Addon><IconSearch aria-hidden="true" /></InputGroup.Addon>
    </InputGroup.Root>
  </div>
  <nav
    class="min-h-0 flex-1 space-y-1 overflow-y-auto px-2 pb-4"
    aria-label={m.messaging_channels()}
  >
    {#each channels as channel (channel.id)}
      <ListButton
        variant="navigation"
        class="w-full"
        aria-label={m.messaging_open_channel({ name: channel.name })}
        aria-current={view.channel === channel.name ? "page" : undefined}
        disabled={view.sending || view.creating}
        onclick={() => messaging.select(channel.name)}
      >
        <span
          class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"
          ><IconHash class="size-5" aria-hidden="true" /></span
        >
        <span class="flex min-w-0 flex-1 flex-col gap-1">
          <span class="truncate" title={channel.name}>{channel.name}</span>
          <span class="truncate text-xs font-normal text-muted-foreground"
            >{channel.latestText ||
              channel.topic ||
              m.messaging_empty_preview()}</span
          >
        </span>
      </ListButton>
    {:else}
      {#if !view.loading}
        <Empty.Root>
          <Empty.Header
            ><Empty.Title
              >{view.query
                ? m.messaging_no_results()
                : m.messaging_empty_channels()}</Empty.Title
            ></Empty.Header
          >
          {#if !view.query}<Empty.Content
              ><Button
                class="sm:hidden"
                onclick={() => (view.createOpen = true)}
                ><IconPlus
                  aria-hidden="true"
                />{m.messaging_new_channel()}</Button
              ></Empty.Content
            >{/if}
        </Empty.Root>
      {/if}
    {/each}
  </nav>
</div>
