<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import { Label } from "$lib/components/ui/label";
  import { Input } from "$lib/components/ui/input";
  import { MessagingCreateChannelSchema } from "../../contracts/messaging-schemas";
  import { Textarea } from "$lib/components/ui/textarea";
  import { NativeSelect } from "$lib/components/ui/native-select";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { useMessaging } from "../../hooks/settings/messaging.svelte";

  const messaging = useMessaging();
  const state = messaging.state;
</script>

<form
  class="space-y-2"
  onsubmit={(event) => {
    event.preventDefault();
    void messaging.createChannel();
  }}
>
  <div class="grid gap-2 sm:grid-cols-2">
    <div class="space-y-2">
      <Label for="messaging-channel-name">{m.messaging_channel_name()}</Label>
      <Input
        id="messaging-channel-name"
        bind:value={state.newName}
        disabled={state.creating}
        maxlength={64}
        aria-describedby="messaging-name-hint"
      />
    </div>
    <div class="space-y-2">
      <Label for="messaging-channel-topic">{m.messaging_channel_topic()}</Label>
      <Input
        id="messaging-channel-topic"
        bind:value={state.newTopic}
        disabled={state.creating}
      />
    </div>
  </div>
  <p id="messaging-name-hint" class="text-xs text-muted-foreground">
    {m.messaging_name_hint()}
  </p>
  <Button
    type="submit"
    variant="outline"
    disabled={state.creating ||
      state.loading ||
      state.sending ||
      !MessagingCreateChannelSchema.safeParse({
        name: state.newName,
        topic: state.newTopic,
      }).success}>{m.messaging_new_channel()}</Button
  >
</form>
<div class="flex min-w-0 items-end gap-2">
  <div class="min-w-0 flex-1 space-y-2">
    <Label for="messaging-channel">{m.messaging_channel()}</Label>
    <NativeSelect
      id="messaging-channel"
      class="w-full"
      value={state.channel}
      disabled={state.sending || state.creating || state.channels.length === 0}
      onchange={(event) => messaging.select(event.currentTarget.value)}
    >
      {#each state.channels as channel (channel.id)}
        <option value={channel.name}>#{channel.name}</option>
      {/each}
    </NativeSelect>
  </div>
  <Button
    variant="outline"
    size="icon"
    aria-label={m.messaging_refresh()}
    title={m.messaging_refresh()}
    disabled={state.loading || state.sending || state.creating}
    onclick={() => void messaging.refresh()}
    ><IconRefresh aria-hidden="true" /></Button
  >
</div>
{#if state.error}
  <InlineNotice tone="danger">{state.error}</InlineNotice>
{/if}
{#if state.loading}
  <p class="text-sm text-muted-foreground" role="status">
    {m.messaging_loading()}
  </p>
{:else if state.channels.length === 0 && !state.error}
  <p class="text-sm text-muted-foreground">{m.messaging_empty_channels()}</p>
{/if}
{#if state.channel}
  {@const topic = state.channels.find(
    (item) => item.name === state.channel,
  )?.topic}
  {#if topic}<p class="text-sm break-words">{topic}</p>{/if}
  <p class="text-xs text-muted-foreground">{m.messaging_history()}</p>
  {#if state.hasOlder}
    <Button
      variant="outline"
      disabled={state.loading || state.sending || state.creating}
      onclick={() => void messaging.refresh(true)}>{m.messaging_older()}</Button
    >
  {/if}
  <div class="max-h-96 space-y-4 overflow-y-auto" aria-busy={state.loading}>
    {#each state.messages as message (message.id)}
      <article class="min-w-0 space-y-1">
        <div
          class="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-xs text-muted-foreground"
        >
          <span class="break-all font-medium text-foreground"
            >{message.agentName || message.agentId}</span
          >
          <time datetime={message.createdAt}>{message.createdAt}</time>
        </div>
        <p class="text-sm wrap-anywhere whitespace-pre-wrap">{message.text}</p>
      </article>
    {:else}
      {#if !state.loading && !state.error}<p
          class="text-sm text-muted-foreground"
        >
          {m.messaging_empty_messages()}
        </p>{/if}
    {/each}
  </div>
  <form
    class="space-y-2"
    onsubmit={(event) => {
      event.preventDefault();
      void messaging.send();
    }}
  >
    <Label for="messaging-message">{m.messaging_message()}</Label>
    <Textarea
      id="messaging-message"
      bind:value={state.draft}
      disabled={state.sending}
      rows={3}
    />
    <p class="text-xs text-muted-foreground">{m.messaging_limit()}</p>
    <Button
      type="submit"
      disabled={state.sending ||
        state.loading ||
        !state.draft.trim() ||
        new TextEncoder().encode(state.draft).length > 32000}
    >
      {state.sending ? m.messaging_sending() : m.messaging_send()}
    </Button>
  </form>
  {#if state.notice}<InlineNotice tone="success"
      >{m.messaging_sent()}</InlineNotice
    >{/if}
{/if}
