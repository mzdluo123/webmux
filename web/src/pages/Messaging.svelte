<script lang="ts">
  import { onMount } from "svelte";
  import { setBrowserChrome } from "../platform/browser-chrome";
  import { m } from "$lib/paraglide/messages.js";
  import IconMessages from "~icons/tabler/messages";
  import { Button } from "$lib/components/ui/button";
  import * as Empty from "$lib/components/ui/empty";
  import SidebarButton from "../components/shell/SidebarButton.svelte";
  import ChannelList from "../components/messaging/ChannelList.svelte";
  import Conversation from "../components/messaging/Conversation.svelte";
  import CreateChannelDialog from "../components/messaging/CreateChannelDialog.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { useMessaging } from "../hooks/messaging/messaging.svelte";
  const messaging = useMessaging();
  const state = messaging.state;
  onMount(() => setBrowserChrome(m.messaging_title(), false));
</script>

<div
  class="messaging-page flex min-h-0 min-w-0 flex-col bg-background"
  data-messaging-workspace
>
  <header
    class={[
      "flex min-h-(--app-header-height) shrink-0 items-center gap-2 border-b bg-card px-3 sm:px-5",
      state.showConversation && "max-sm:hidden",
    ]}
  >
    <SidebarButton class="sm:hidden" />
    <IconMessages class="size-5 text-primary" aria-hidden="true" />
    <h1 class="text-base font-semibold">{m.messaging_title()}</h1>
  </header>
  <div class="flex min-h-0 flex-1">
    <aside
      class={[
        "min-h-0 w-full shrink-0 flex-col border-r bg-card sm:flex sm:w-64",
        state.showConversation ? "hidden" : "flex",
      ]}
      aria-label={m.messaging_channels()}
    >
      <ChannelList {messaging} />
      {#if state.error && !state.channel}<div class="p-4">
          <InlineNotice tone="danger">{state.error}</InlineNotice><Button
            variant="outline"
            class="mt-2"
            onclick={() => void messaging.refresh()}
            >{m.messaging_refresh()}</Button
          >
        </div>{/if}
    </aside>
    <section
      class={[
        "min-h-0 min-w-0 flex-1 sm:block",
        state.showConversation ? "block" : "hidden",
      ]}
    >
      {#if state.channel}
        {#key state.channel}<Conversation {messaging} />{/key}
      {:else}
        <Empty.Root class="h-full">
          <Empty.Header
            ><Empty.Media variant="icon"
              ><IconMessages aria-hidden="true" /></Empty.Media
            ><Empty.Title>{m.messaging_welcome()}</Empty.Title
            ><Empty.Description>{m.messaging_welcome_desc()}</Empty.Description
            ></Empty.Header
          >
          <Empty.Content
            ><Button
              disabled={state.loading}
              onclick={() => (state.createOpen = true)}
              >{m.messaging_first_channel()}</Button
            ></Empty.Content
          >
        </Empty.Root>
      {/if}
    </section>
  </div>
</div>
{#if state.createOpen}<CreateChannelDialog {messaging} />{/if}
