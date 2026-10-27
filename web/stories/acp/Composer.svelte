<script lang="ts">
  import IconArrowBarUp from "~icons/tabler/arrow-bar-up";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconCornerDownRight from "~icons/tabler/corner-down-right";
  import IconMessageBolt from "~icons/tabler/message-bolt";
  import IconPlayerPause from "~icons/tabler/player-pause";
  import IconPlayerStopFilled from "~icons/tabler/player-stop-filled";
  import IconTrash from "~icons/tabler/trash";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";
  import { Textarea } from "$lib/components/ui/textarea";
  import UsageRing from "./UsageRing.svelte";
  import type { ChatState } from "./fixtures";

  // A Codex-style composer card. Enter sends while idle and queues while a
  // turn runs (server-side FIFO, D6); "插话" appears only when the agent
  // declares `_meta.steering`. Config options render as menus in the footer.
  let {
    chat = $bindable(),
    onsend,
    onsteer,
    onstop,
    onsendnow,
  }: {
    chat: ChatState;
    onsend: (text: string) => void;
    onsteer: (text: string) => void;
    onstop: () => void;
    onsendnow: (id: string) => void;
  } = $props();
  const query = $derived(
    /^\/\S*$/.test(chat.draft) ? chat.draft.slice(1).toLowerCase() : null,
  );
  const matches = $derived(
    query === null
      ? []
      : chat.commands.filter((command) => command.name.startsWith(query)),
  );
  const typed = $derived(chat.draft.trim() !== "");

  function take() {
    const text = chat.draft.trim();
    chat.draft = "";
    return text;
  }
  function submit() {
    if (typed) onsend(take());
  }
  function steer() {
    if (typed) onsteer(take());
  }
</script>

<div class="space-y-2">
  {#if chat.queue.length}
    <section
      class="rounded-2xl border bg-card px-1 py-1 text-sm"
      aria-label="排队的消息"
    >
      <div
        class="flex flex-wrap items-center gap-2 px-3 py-1 text-xs text-muted-foreground"
      >
        <span>排队中 · {chat.queue.length}</span>
        {#if chat.queuePaused}
          <span class="flex items-center gap-1 text-warning" role="status">
            <IconPlayerPause
              class="size-3.5"
              aria-hidden="true"
            />回合已中断，队列暂停
          </span>
          <span class="ml-auto flex gap-1">
            <Button
              variant="outline"
              size="xs"
              onclick={() => (chat.queuePaused = false)}>继续</Button
            >
            <Button variant="quiet" size="xs" onclick={() => (chat.queue = [])}
              >清空</Button
            >
          </span>
        {/if}
      </div>
      <ol>
        {#each chat.queue as item, i (item.id)}
          <li
            class="flex flex-wrap items-center justify-end gap-x-1 rounded-xl pr-1 pl-3 hover:bg-muted/50"
          >
            <IconCornerDownRight
              class="size-3.5 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
            <Textarea
              variant="ghost"
              rows={1}
              class="min-h-0 min-w-0 flex-1 basis-40 resize-none"
              aria-label={`编辑排队消息 ${i + 1}`}
              bind:value={item.text}
            />
            <Button
              variant="quiet"
              size="icon-sm"
              aria-label="立即发送（中断当前回合）"
              title="立即发送（中断当前回合）"
              onclick={() => onsendnow(item.id)}
              ><IconArrowBarUp aria-hidden="true" /></Button
            >
            <Button
              variant="quiet"
              size="icon-sm"
              aria-label="删除"
              title="删除"
              onclick={() =>
                (chat.queue = chat.queue.filter((q) => q.id !== item.id))}
              ><IconTrash aria-hidden="true" /></Button
            >
          </li>
        {/each}
      </ol>
    </section>
  {/if}

  <div class="rounded-2xl border bg-card shadow-sm focus-within:border-ring">
    {#if matches.length}
      <div
        class="max-h-52 overflow-y-auto border-b p-1"
        role="listbox"
        aria-label="命令"
      >
        {#each matches as command (command.name)}
          <ListButton
            variant="row"
            role="option"
            aria-selected="false"
            class="flex w-full items-baseline gap-2 text-sm"
            onclick={() => (chat.draft = `/${command.name} `)}
          >
            <span class="font-mono">/{command.name}</span>
            {#if command.hint}<span class="text-muted-foreground"
                >&lt;{command.hint}&gt;</span
              >{/if}
            <span class="ml-auto truncate text-xs text-muted-foreground"
              >{command.description}</span
            >
          </ListButton>
        {/each}
      </div>
    {/if}
    <Textarea
      variant="ghost"
      rows={2}
      class="max-h-48 min-h-14 resize-none px-4 pt-3"
      placeholder={chat.running
        ? "继续输入，回车加入排队"
        : "给 Agent 发消息，/ 打开命令"}
      aria-label="消息"
      bind:value={chat.draft}
      onkeydown={(event) => {
        if (event.key !== "Enter" || event.shiftKey || event.isComposing)
          return;
        event.preventDefault();
        if (event.ctrlKey && chat.steering && chat.running) steer();
        else submit();
      }}
    />
    <div class="flex items-center gap-1 px-2 pb-2">
      <div class="flex min-w-0 flex-1 flex-wrap items-center">
        {#each chat.config as option (option.id)}
          <DropdownMenu.Root>
            <DropdownMenu.Trigger>
              {#snippet child({ props })}
                <Button
                  {...props}
                  variant="quiet"
                  size="xs"
                  class="max-w-[min(9rem,100%)]"
                  aria-label={`${option.name}：${option.options.find((o) => o.value === option.value)?.name}`}
                  title={option.name}
                >
                  <span class="truncate"
                    >{option.options.find((o) => o.value === option.value)
                      ?.name}</span
                  >
                  <IconChevronDown aria-hidden="true" />
                </Button>
              {/snippet}
            </DropdownMenu.Trigger>
            <DropdownMenu.Content align="start">
              <DropdownMenu.Label>{option.name}</DropdownMenu.Label>
              <DropdownMenu.RadioGroup bind:value={option.value}>
                {#each option.options as choice (choice.value)}
                  <DropdownMenu.RadioItem value={choice.value}
                    >{choice.name}</DropdownMenu.RadioItem
                  >
                {/each}
              </DropdownMenu.RadioGroup>
            </DropdownMenu.Content>
          </DropdownMenu.Root>
        {/each}
      </div>
      <span
        class="flex shrink-0 items-center gap-1 px-1 text-xs text-muted-foreground tabular-nums"
        title={`上下文 ${(chat.usage.used / 1000).toFixed(1)}k / ${chat.usage.size / 1000}k · $${chat.usage.cost.toFixed(2)}`}
      >
        <UsageRing
          used={chat.usage.used}
          size={chat.usage.size}
          class="size-4"
        />
        <span class="max-sm:hidden"
          >{Math.round((chat.usage.used / chat.usage.size) * 100)}%</span
        >
      </span>
      {#if chat.running && chat.steering && typed}
        <Button
          variant="outline"
          size="icon-touch"
          aria-label="插话"
          title="插话：注入当前回合（Ctrl+Enter）"
          onclick={steer}><IconMessageBolt aria-hidden="true" /></Button
        >
      {/if}
      {#if chat.running && !typed}
        <Button
          variant="secondary"
          size="icon-touch"
          aria-label="停止"
          title="停止"
          onclick={onstop}><IconPlayerStopFilled aria-hidden="true" /></Button
        >
      {:else}
        <Button
          size="icon-touch"
          aria-label={chat.running ? "加入排队" : "发送"}
          title={chat.running ? "加入排队" : "发送"}
          disabled={!typed}
          onclick={submit}><IconArrowUp aria-hidden="true" /></Button
        >
      {/if}
    </div>
  </div>
</div>
