<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { useDockMotion } from "../../hooks/shared/dock-motion.svelte";
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowBarUp from "~icons/tabler/arrow-bar-up";
  import IconArrowNarrowUp from "~icons/tabler/arrow-narrow-up";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconCornerDownRight from "~icons/tabler/corner-down-right";
  import IconPlayerPause from "~icons/tabler/player-pause";
  import IconTrash from "~icons/tabler/trash";
  import { Button } from "$lib/components/ui/button";
  import { Textarea } from "$lib/components/ui/textarea";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import type { ChatActions } from "../../platform/chat-api";
  import { errorMessage } from "../../platform/error";
  import type { ChatState } from "../../contracts/schemas";

  // The server's prompt queue, shared by every device. Small viewports fold
  // it to one line with the count and the first prompt. After an interrupted
  // turn it pauses until the user continues or clears it.
  let {
    queue,
    running,
    disabled,
    actions,
    onerror,
  }: {
    queue: ChatState["queue"];
    running: boolean;
    disabled: boolean;
    actions: ChatActions;
    onerror: (message: string) => void;
  } = $props();

  let open = $state(
    new MediaQuery("(min-width: 640px) and (min-height: 480px)").current,
  );
  let list = $state<HTMLOListElement | null>(null);
  useDockMotion(
    () => list,
    () => open,
    () => JSON.stringify(queue.items.map((item) => item.id)),
  );
  let sendNow = $state<string | null>(null);
  let sendNowOpen = $state(false);

  function run(action: () => Promise<unknown>) {
    action().then(
      () => onerror(""),
      (err: unknown) => onerror(errorMessage(err)),
    );
  }
  function edit(
    itemId: string,
    before: string,
    text: string,
    hasAttachments: boolean,
  ) {
    const next = text.trim();
    if (next !== before && (next || hasAttachments))
      run(() => actions.editQueued(itemId, next));
  }
</script>

<section
  class="rounded-lg border bg-card p-1 text-sm"
  aria-label={m.s3_queued_messages_label()}
>
  <div class="flex flex-wrap items-center gap-2 px-2 py-0.5">
    <Button
      variant="quiet"
      size="xs"
      class="max-w-full min-w-0"
      aria-expanded={open}
      onclick={() => (open = !open)}
      >{m.s3_queue_count({ count: queue.items.length })}{#if !open}<span
          class="min-w-0 truncate"
          >· {queue.items[0]?.text || m.s3_attachment_short()}</span
        >{/if}<IconChevronDown
        class={[
          "motion-safe:transition-transform motion-safe:duration-200",
          open && "rotate-180",
        ]}
        aria-hidden="true"
      /></Button
    >
    {#if queue.paused}
      <span class="flex items-center gap-1 text-xs text-warning" role="status">
        <IconPlayerPause
          class="size-3.5"
          aria-hidden="true"
        />{m.s3_queue_paused_status()}
      </span>
      <span class="ml-auto flex gap-1">
        <Button
          variant="outline"
          size="xs"
          {disabled}
          onclick={() => run(actions.resumeQueue)}>{m.s3_queue_resume()}</Button
        >
        <Button
          variant="quiet"
          size="xs"
          {disabled}
          onclick={() => run(actions.clearQueue)}>{m.s3_queue_clear()}</Button
        >
      </span>
    {/if}
  </div>
  <ol bind:this={list} hidden={!open} inert={!open}>
    {#each queue.items as item, i (item.id)}
      <li class="flex flex-wrap items-center justify-end gap-x-1 pr-1 pl-2">
        <IconCornerDownRight
          class="size-3.5 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        <Textarea
          variant="ghost"
          rows={1}
          class="min-h-0 min-w-0 flex-1 basis-40"
          aria-label={m.s3_queue_edit_item_label({ index: i + 1 })}
          {disabled}
          value={item.text}
          onkeydown={(event) => {
            if (event.key !== "Enter" || event.shiftKey || event.isComposing)
              return;
            event.preventDefault();
            event.currentTarget.blur();
          }}
          onchange={(event) =>
            edit(
              item.id,
              item.text,
              event.currentTarget.value,
              !!item.attachments?.length,
            )}
        />
        {#if item.attachments?.length}
          <span
            class="flex min-w-0 basis-full flex-wrap justify-end gap-1 pr-1 pl-5 text-xs text-muted-foreground"
            aria-label={m.s3_queue_item_attachments_label()}
          >
            {#each item.attachments as attachment (attachment.path)}
              <span class="max-w-40 truncate"
                >{attachment.name ?? attachment.path}</span
              >
            {/each}
          </span>
        {/if}
        {#if i > 0}
          <Button
            variant="quiet"
            size="icon-sm"
            aria-label={m.s3_move_up()}
            title={m.s3_move_up()}
            {disabled}
            onclick={() => run(() => actions.moveQueuedUp(item.id))}
            ><IconArrowNarrowUp aria-hidden="true" /></Button
          >
        {/if}
        <Button
          variant="quiet"
          size="icon-sm"
          aria-label={m.s3_send_now()}
          title={running ? m.s3_send_now_interrupt_title() : m.s3_send_now()}
          {disabled}
          onclick={() => {
            if (!running) {
              run(() => actions.sendQueuedNow(item.id));
              return;
            }
            sendNow = item.id;
            sendNowOpen = true;
          }}><IconArrowBarUp aria-hidden="true" /></Button
        >
        <Button
          variant="quiet"
          size="icon-sm"
          aria-label={m.s3_delete()}
          title={m.s3_delete()}
          {disabled}
          onclick={() => run(() => actions.removeQueued(item.id))}
          ><IconTrash aria-hidden="true" /></Button
        >
      </li>
    {/each}
  </ol>
</section>

<ConfirmDialog
  bind:open={sendNowOpen}
  title={m.s3_send_now_confirm_title()}
  onconfirm={() => {
    const itemId = sendNow;
    sendNowOpen = false;
    if (itemId) run(() => actions.sendQueuedNow(itemId));
  }}>{m.s3_send_now_confirm_body()}</ConfirmDialog
>
