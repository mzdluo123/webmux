<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import { MessagingCreateChannelSchema } from "../../contracts/messaging-schemas";
  import type { MessagingController } from "../../hooks/messaging/messaging.svelte";
  let { messaging }: { messaging: MessagingController } = $props();
  const view = $derived(messaging.state);
</script>

<ResponsiveDialog
  title={m.messaging_new_channel()}
  description={m.messaging_create_desc()}
  busy={view.creating}
  onclose={() => (view.createOpen = false)}
>
  <form
    class="space-y-4"
    onsubmit={(event) => {
      event.preventDefault();
      void messaging.createChannel();
    }}
  >
    <div class="space-y-2">
      <Label for="messaging-channel-name">{m.messaging_channel_name()}</Label>
      <Input
        id="messaging-channel-name"
        bind:value={view.newName}
        maxlength={64}
        placeholder="general"
        disabled={view.creating}
        aria-describedby="messaging-name-hint"
      />
      <p id="messaging-name-hint" class="text-xs text-muted-foreground">
        {m.messaging_name_hint()}
      </p>
    </div>
    <div class="space-y-2">
      <Label for="messaging-channel-topic">{m.messaging_channel_topic()}</Label>
      <Input
        id="messaging-channel-topic"
        bind:value={view.newTopic}
        disabled={view.creating}
        placeholder={m.messaging_topic_placeholder()}
      />
    </div>
    {#if view.createError}<InlineNotice tone="danger"
        >{view.createError}</InlineNotice
      >{/if}
    <div class="flex justify-end gap-2">
      <Button
        type="button"
        variant="outline"
        disabled={view.creating}
        onclick={() => (view.createOpen = false)}>{m.s1_cancel()}</Button
      >
      <Button
        type="submit"
        disabled={view.creating ||
          !MessagingCreateChannelSchema.safeParse({
            name: view.newName,
            topic: view.newTopic,
          }).success}
        >{view.creating
          ? m.messaging_creating()
          : m.messaging_new_channel()}</Button
      >
    </div>
  </form>
</ResponsiveDialog>
