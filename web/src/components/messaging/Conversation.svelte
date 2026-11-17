<script lang="ts">
  import { tick, untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { getLocale } from "$lib/paraglide/runtime.js";
  import IconHash from "~icons/tabler/hash";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconRefresh from "~icons/tabler/refresh";
  import IconMessages from "~icons/tabler/messages";
  import { Button } from "$lib/components/ui/button";
  import * as InputGroup from "$lib/components/ui/input-group";
  import * as Empty from "$lib/components/ui/empty";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import type { MessagingController } from "../../hooks/messaging/messaging.svelte";

  let { messaging }: { messaging: MessagingController } = $props();
  const view = $derived(messaging.state);
  const channel = $derived(
    view.channels.find((item) => item.name === view.channel),
  );
  const bytes = $derived(new TextEncoder().encode(view.draft).length);
  let viewport = $state<HTMLDivElement | null>(null);
  let first = true;
  let pinned = true;
  let paging = false;
  const day = (value: string) =>
    new Date(value).toLocaleDateString(getLocale(), {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  const time = (value: string) =>
    new Date(value).toLocaleTimeString(getLocale(), {
      hour: "2-digit",
      minute: "2-digit",
    });

  function scrolled() {
    if (!viewport) return;
    pinned =
      viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop < 64;
    messaging.positions[view.channel] = {
      top: viewport.scrollTop,
      bottom: pinned,
    };
  }
  $effect(() => {
    void view.messages.length;
    const loading = view.loading;
    const sent = view.notice;
    if (loading) return;
    let active = true;
    untrack(() => {
      void tick().then(() => {
        if (!active || !viewport || paging) return;
        const saved = messaging.positions[view.channel];
        if (first && saved && !saved.bottom) viewport.scrollTop = saved.top;
        else if (first || pinned || sent)
          viewport.scrollTop = viewport.scrollHeight;
        first = false;
      });
    });
    return () => {
      active = false;
    };
  });
  async function older() {
    if (!viewport) return;
    const height = viewport.scrollHeight;
    const top = viewport.scrollTop;
    paging = true;
    await messaging.refresh(true);
    await tick();
    if (viewport) viewport.scrollTop = top + viewport.scrollHeight - height;
    paging = false;
  }
  function keydown(event: KeyboardEvent) {
    if (
      event.key === "Enter" &&
      !event.shiftKey &&
      !event.isComposing &&
      event.keyCode !== 229
    ) {
      event.preventDefault();
      void messaging.send();
    }
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col"
  aria-label={m.messaging_conversation({ name: view.channel })}
>
  <header
    class="flex min-h-(--app-header-height) shrink-0 items-center gap-2 border-b bg-card px-3 py-2 sm:px-5"
  >
    <Button
      variant="ghost"
      size="icon-touch"
      class="sm:hidden"
      aria-label={m.messaging_back()}
      title={m.messaging_back()}
      onclick={() => (view.showConversation = false)}
      ><IconArrowLeft aria-hidden="true" /></Button
    >
    <span
      class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary"
      ><IconHash class="size-6" aria-hidden="true" /></span
    >
    <div class="min-w-0 flex-1">
      <h2 class="truncate text-base font-semibold" title={view.channel}>
        {view.channel}
      </h2>
      {#if channel?.topic}<p
          class="truncate text-xs text-muted-foreground"
          title={channel.topic}
        >
          {channel.topic}
        </p>{/if}
    </div>
    <Button
      variant="ghost"
      size="icon-touch"
      aria-label={m.messaging_refresh()}
      title={m.messaging_refresh()}
      disabled={view.loading || view.sending || view.creating}
      onclick={() => void messaging.refresh()}
      ><IconRefresh aria-hidden="true" /></Button
    >
  </header>
  {#if view.error}<div class="px-4 pt-3">
      <InlineNotice tone="danger">{view.error}</InlineNotice>
    </div>{/if}
  <div
    bind:this={viewport}
    class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-5 sm:px-6"
    aria-label={m.messaging_history_label()}
    aria-busy={view.loading}
    onscroll={scrolled}
  >
    <div class="mx-auto flex min-h-full max-w-3xl flex-col gap-5">
      {#if view.hasOlder}<div class="flex justify-center">
          <Button
            variant="outline"
            disabled={view.loading || view.sending}
            onclick={() => void older()}>{m.messaging_older()}</Button
          >
        </div>{/if}
      {#if view.loading && view.messages.length === 0}<p
          class="text-sm text-muted-foreground"
          role="status"
        >
          {m.messaging_loading()}
        </p>{/if}
      {#each view.messages as message, index (message.id)}
        {#if index === 0 || day(message.createdAt) !== day(view.messages[index - 1]!.createdAt)}
          <div class="flex items-center gap-3 text-xs text-muted-foreground">
            <span class="h-px flex-1 bg-border"></span><span
              >{day(message.createdAt)}</span
            ><span class="h-px flex-1 bg-border"></span>
          </div>
        {/if}
        <article class="flex min-w-0 gap-3">
          <span
            class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-sm font-semibold text-primary"
            aria-hidden="true"
            >{(message.agentName || message.agentId)
              .slice(0, 1)
              .toUpperCase()}</span
          >
          <div class="min-w-0 flex-1 space-y-1">
            <div class="flex flex-wrap items-baseline gap-x-2 gap-y-1">
              <span class="text-sm font-semibold"
                >{message.agentName || message.agentId}</span
              ><time
                class="text-xs text-muted-foreground"
                datetime={message.createdAt}
                title={new Date(message.createdAt).toLocaleString(getLocale())}
                >{time(message.createdAt)}</time
              >
            </div>
            <p
              class="text-sm leading-relaxed wrap-anywhere whitespace-pre-wrap"
            >
              {message.text}
            </p>
            {#if message.id === view.notice}<p
                class="text-xs text-muted-foreground"
                role="status"
              >
                {m.messaging_sent()}
              </p>{/if}
          </div>
        </article>
      {:else}
        {#if !view.loading && !view.error}
          <Empty.Root class="my-auto">
            <Empty.Header
              ><Empty.Media variant="icon"
                ><IconMessages aria-hidden="true" /></Empty.Media
              ><Empty.Title>{m.messaging_start_conversation()}</Empty.Title
              ><Empty.Description
                >{m.messaging_first_message({
                  name: view.channel,
                })}</Empty.Description
              ></Empty.Header
            >
          </Empty.Root>
        {/if}
      {/each}
      {#if view.sending || view.failed}
        <article class="flex gap-3">
          <span
            class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-sm font-semibold text-primary"
            aria-hidden="true">W</span
          >
          <div class="min-w-0 flex-1 space-y-2">
            <p class="text-sm font-semibold">WebMux</p>
            <p class="text-sm wrap-anywhere whitespace-pre-wrap">
              {view.sending ? view.pendingText : view.failed?.text}
            </p>
            {#if view.failed}<InlineNotice tone="danger"
                >{view.failed.error}</InlineNotice
              ><Button
                variant="outline"
                onclick={() => void messaging.send(view.failed!.text)}
                >{m.messaging_retry()}</Button
              >{:else}<p class="text-xs text-muted-foreground" role="status">
                {m.messaging_sending()}
              </p>{/if}
          </div>
        </article>
      {/if}
    </div>
  </div>
  <div class="shrink-0 border-t bg-card px-3 pt-3 pb-safe-4 sm:px-5 sm:pb-4">
    <form
      class="mx-auto max-w-3xl"
      onsubmit={(event) => {
        event.preventDefault();
        void messaging.send();
      }}
    >
      <InputGroup.Root>
        <InputGroup.Textarea
          aria-label={m.messaging_message()}
          placeholder={m.messaging_placeholder({ name: view.channel })}
          bind:value={view.draft}
          disabled={view.sending}
          class="max-h-40 min-h-14"
          rows={2}
          onkeydown={keydown}
        />
        <InputGroup.Addon align="block-end" class="justify-between">
          <span class="text-xs font-normal text-muted-foreground"
            >{#if bytes > 28000}{bytes.toLocaleString()} / 32,000{:else}<span
                class="max-sm:hidden">{m.messaging_keyboard_hint()}</span
              >{/if}</span
          >
          <Button
            type="submit"
            size="sm-touch"
            aria-label={m.messaging_send()}
            disabled={view.sending ||
              view.loading ||
              view.creating ||
              !view.draft.trim() ||
              bytes > 32000}
            ><IconArrowUp aria-hidden="true" />{view.sending
              ? m.messaging_sending()
              : m.messaging_send_short()}</Button
          >
        </InputGroup.Addon>
      </InputGroup.Root>
    </form>
  </div>
</section>
