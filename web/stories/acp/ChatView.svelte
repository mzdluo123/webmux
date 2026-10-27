<script lang="ts">
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconCircle from "~icons/tabler/circle";
  import IconCircleCheck from "~icons/tabler/circle-check";
  import IconLoader2 from "~icons/tabler/loader-2";
  import { Button } from "$lib/components/ui/button";
  import * as Collapsible from "$lib/components/ui/collapsible";
  import Composer from "./Composer.svelte";
  import PermissionPrompt from "./PermissionPrompt.svelte";
  import TimelineItem from "./TimelineItem.svelte";
  import type { ChatState, PermissionOption } from "./fixtures";

  // The ACP side of a hybrid session: a centered transcript and a bottom
  // dock with the plan, the change summary, a pending permission request and
  // the composer. It fills the pane where the terminal normally renders.
  let {
    chat = $bindable(),
    onreview,
    oncancel,
    onsendnow,
  }: {
    chat: ChatState;
    /** Opens the existing code panel on the session's changes. */
    onreview: () => void;
    oncancel: () => void;
    onsendnow: (id: string) => void;
  } = $props();
  let nextId = 0;
  const id = () => `local-${nextId++}`;
  const pending = $derived(
    chat.timeline.find((entry) => entry.type === "tool" && entry.permission),
  );
  const current = $derived(
    chat.plan.find((entry) => entry.status === "in_progress"),
  );
  const done = $derived(
    chat.plan.filter((entry) => entry.status === "completed").length,
  );
  const added = $derived(chat.changes.reduce((n, c) => n + c.added, 0));
  const removed = $derived(chat.changes.reduce((n, c) => n + c.removed, 0));

  function send(text: string) {
    if (chat.running) chat.queue.push({ id: id(), text });
    else {
      chat.timeline.push({ type: "user", id: id(), text });
      chat.running = true;
    }
  }
  function resolve(entryId: string, option: PermissionOption) {
    const entry = chat.timeline.find((item) => item.id === entryId);
    if (entry?.type !== "tool") return;
    entry.permission = undefined;
    entry.call.status = option.kind.startsWith("allow")
      ? "in_progress"
      : "failed";
  }
</script>

<div class="flex h-full min-h-0 flex-col bg-background" data-testid="acp-chat">
  <div class="min-h-0 flex-1 overflow-y-auto">
    <ol class="mx-auto flex max-w-3xl flex-col gap-2.5 px-4 py-6">
      {#each chat.timeline as entry (entry.id)}
        <li class="min-w-0"><TimelineItem {entry} /></li>
      {/each}
      {#if chat.running && !pending}
        <li class="flex items-center gap-2 text-sm text-muted-foreground">
          <IconLoader2 class="size-3.5 animate-spin" aria-hidden="true" />
          正在工作…
        </li>
      {/if}
    </ol>
  </div>
  <div class="mx-auto w-full max-w-3xl space-y-2 px-3 pb-safe-4 sm:px-4">
    {#if chat.plan.length || chat.changes.length}
      <div class="rounded-2xl border bg-card text-sm">
        <Collapsible.Root>
          <div
            class="flex min-w-0 flex-wrap items-center gap-x-2 py-1 pr-1 pl-3"
          >
            <Collapsible.Trigger>
              {#snippet child({ props })}
                <Button
                  {...props}
                  variant="quiet"
                  size="xs"
                  class="group/plan -ml-2 max-w-full min-w-0 justify-start"
                >
                  {#if current}
                    <IconLoader2
                      class="animate-spin text-info"
                      aria-hidden="true"
                    />
                  {:else}
                    <IconCircleCheck class="text-success" aria-hidden="true" />
                  {/if}
                  <span class="shrink-0">计划 {done}/{chat.plan.length}</span>
                  {#if current}<span class="truncate text-foreground"
                      >{current.content}</span
                    >{/if}
                  <IconChevronDown
                    class="shrink-0 transition-transform group-aria-expanded/plan:rotate-180"
                    aria-hidden="true"
                  />
                </Button>
              {/snippet}
            </Collapsible.Trigger>
            <Button
              variant="quiet"
              size="xs"
              class="ml-auto"
              title="在代码面板中审阅改动"
              onclick={onreview}
            >
              <span class="max-sm:hidden">{chat.changes.length} 个文件</span>
              <span class="font-mono tabular-nums">
                <span class="text-success">+{added}</span>
                <span class="text-destructive">−{removed}</span>
              </span>
              <span class="text-foreground">审阅</span>
            </Button>
          </div>
          <Collapsible.Content>
            <ol class="space-y-1.5 border-t px-3 py-2">
              {#each chat.plan as entry (entry.content)}
                <li class="flex items-center gap-2">
                  {#if entry.status === "completed"}
                    <IconCircleCheck
                      class="size-4 shrink-0 text-success"
                      aria-label="完成"
                    />
                  {:else if entry.status === "in_progress"}
                    <IconLoader2
                      class="size-4 shrink-0 animate-spin text-info"
                      aria-label="进行中"
                    />
                  {:else}
                    <IconCircle
                      class="size-4 shrink-0 text-muted-foreground"
                      aria-label="待办"
                    />
                  {/if}
                  <span
                    class={entry.status === "completed"
                      ? "text-muted-foreground line-through"
                      : undefined}>{entry.content}</span
                  >
                </li>
              {/each}
            </ol>
          </Collapsible.Content>
        </Collapsible.Root>
      </div>
    {/if}
    {#if pending?.type === "tool" && pending.permission}
      {@const entry = pending}
      <PermissionPrompt
        call={entry.call}
        options={entry.permission ?? []}
        onresolve={(option) => resolve(entry.id, option)}
      />
    {:else}
      <Composer
        bind:chat
        onsend={send}
        onsteer={(text) =>
          chat.timeline.push({ type: "user", id: id(), text: `插话：${text}` })}
        onstop={oncancel}
        {onsendnow}
      />
    {/if}
  </div>
</div>
