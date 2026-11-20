<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { tick } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import type { DisplayEntry } from "../../domains/chat/chat-model";
  import type { ChatCodeLink } from "../../domains/code/markdown-link";
  import {
    createTranscriptWindow,
    entryKey,
  } from "../../hooks/chat/transcript-window.svelte";
  import TimelineItem from "./TimelineItem.svelte";

  let {
    sessionId,
    visible,
    entries,
    turnIds,
    documentScroll,
    scroller = null,
    following = true,
    preview,
    waiting,
    onreview,
    onopenlocation,
    onjump,
  }: {
    sessionId?: string;
    visible: boolean;
    entries: readonly DisplayEntry[];
    turnIds: ReadonlySet<string>;
    documentScroll: boolean;
    scroller?: HTMLElement | null;
    following?: boolean;
    preview: (path: string) => string | undefined;
    waiting: readonly string[];
    onreview?: (path: string) => void;
    onopenlocation?: (target: ChatCodeLink) => void;
    onjump: (id: string) => boolean;
  } = $props();

  const transcriptWindow = createTranscriptWindow({
    entries: () => entries,
    scroller: () => scroller,
    documentScroll: () => documentScroll,
    following: () => following,
  });

  export function reveal(messageId: string) {
    transcriptWindow.reveal(messageId);
  }
  export function clearReveal() {
    transcriptWindow.clearReveal();
  }

  function requestedMessage() {
    const query = new URLSearchParams(window.location.search);
    return query.get("messageSession") === sessionId
      ? query.get("message")
      : null;
  }
  let targetMessage = $state(requestedMessage());
  let selectedMessage = $state<string | null>(null);
  $effect(() => {
    const id = targetMessage;
    if (
      !visible ||
      !id ||
      !entries.some((entry) => entry.type === "message" && entry.id === id)
    )
      return;
    let cancelled = false;
    void tick().then(() => {
      if (cancelled || targetMessage !== id) return;
      if (!onjump(id)) return;
      selectedMessage = id;
      targetMessage = null;
      const url = new URL(window.location.href);
      url.searchParams.delete("message");
      url.searchParams.delete("messageSession");
      history.replaceState(history.state, "", url);
    });
    return () => {
      cancelled = true;
    };
  });
</script>

<svelte:window
  onpopstate={() => {
    targetMessage = requestedMessage();
    if (!targetMessage) selectedMessage = null;
  }}
/>

<ol class="flex flex-col gap-2.5" aria-label={m.s3_transcript_label()}>
  {#each entries as entry, index (entryKey(entry))}
    {@const key = entryKey(entry)}
    {@const mounted = transcriptWindow.isMounted(
      entry,
      index,
      selectedMessage,
      targetMessage,
    )}
    <li
      use:transcriptWindow.observeRow={key}
      class={[
        "min-w-0 [contain:layout_style]",
        documentScroll ? "scroll-mt-16" : "scroll-mt-4",
        !mounted && "min-h-(--transcript-placeholder-height)",
      ]}
      style:--transcript-placeholder-height={mounted
        ? undefined
        : `${transcriptWindow.placeholderHeight(entry, key)}px`}
      class:rounded-md={entry.type === "message" &&
        selectedMessage === entry.id}
      class:bg-muted={entry.type === "message" && selectedMessage === entry.id}
      data-message-id={entry.type === "message" ? entry.id : undefined}
      data-message-highlight={entry.type === "message" &&
      selectedMessage === entry.id
        ? ""
        : undefined}
      data-conversation-message={entry.type === "message" &&
      entry.role !== "thought" &&
      turnIds.has(entry.id)
        ? entry.id
        : undefined}
    >
      {#if mounted}
        <TimelineItem
          {entry}
          {sessionId}
          {preview}
          {waiting}
          {onreview}
          {onopenlocation}
        />
        {#if entry.type === "message" && selectedMessage === entry.id}
          <Button
            variant="ghost"
            size="xs"
            onclick={() => (selectedMessage = null)}
            >{m.s3_clear_message_highlight()}</Button
          >
        {/if}
      {/if}
    </li>
  {/each}
</ol>
