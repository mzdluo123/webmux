<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import * as Popover from "$lib/components/ui/popover";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { conversationTurnGroups } from "../../domains/chat/conversation-turns";
  import type { TimelineEntry } from "../../domains/chat/chat-model";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { ConversationTurnItem } from "../../domains/chat/conversation-turns";

  let {
    items,
    entries,
    activeId,
    documentScroll,
    onselect,
  }: {
    items: ConversationTurnItem[];
    entries: readonly TimelineEntry[];
    activeId: string;
    documentScroll: boolean;
    onselect: (id: string) => void;
  } = $props();

  let list = $state<HTMLDivElement | null>(null);

  const label = (item: ConversationTurnItem) =>
    m.s3_turn_item_label({
      prefix: item.turn
        ? m.s3_turn_number({ turn: item.turn })
        : m.s3_turn_earlier(),
      speaker:
        item.role === "user"
          ? m.s3_turn_role_you()
          : item.messageIds.length > 1
            ? m.s3_turn_role_ai_many({ count: item.messageIds.length })
            : "AI",
      summary: item.summary,
    });

  let rail = $state<HTMLElement | null>(null);
  let capacity = $state(40);
  let openId = $state("");
  $effect(() => {
    const node = rail;
    if (!node) return;
    const observer = new ResizeObserver(() => {
      capacity = Math.max(1, Math.floor((node.clientHeight - 8) / 12));
    });
    observer.observe(node);
    return () => observer.disconnect();
  });
  const groups = $derived(conversationTurnGroups(items, capacity));
  const messages = $derived(
    new Map(
      entries.flatMap((entry) =>
        entry.type === "message" ? [[entry.id, entry] as const] : [],
      ),
    ),
  );
  const groupIds = (group: ConversationTurnItem[]) =>
    group.flatMap((item) => item.messageIds);
  const currentGroup = $derived(
    groups.findIndex((group) => groupIds(group).includes(activeId)),
  );
  function groupLabel(group: ConversationTurnItem[]) {
    if (group.length === 1) return label(group[0]!);
    return m.s3_turn_group_label({
      first: group[0]!.turn,
      last: group.at(-1)!.turn,
      count: groupIds(group).length,
    });
  }
  function choose(id: string) {
    openId = "";
    onselect(id);
  }

  const volume = (count: number) =>
    count >= 8
      ? "many"
      : count >= 4
        ? "several"
        : count >= 2
          ? "few"
          : "single";

  function move(event: KeyboardEvent, index: number) {
    let next: number;
    switch (event.key) {
      case "ArrowUp":
        next = Math.max(0, index - 1);
        break;
      case "ArrowDown":
        next = Math.min(groups.length - 1, index + 1);
        break;
      case "Home":
        next = 0;
        break;
      case "End":
        next = groups.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    const item = groups[next]?.[0];
    if (!item) return;
    onselect(item.id);
    const selected = list?.querySelector<HTMLElement>(
      `[data-turn-index="${next}"]`,
    );
    selected?.focus({ preventScroll: true });
  }
</script>

<nav
  bind:this={rail}
  aria-label={m.s3_conversation_turns_nav()}
  class={documentScroll
    ? "pointer-events-none fixed top-16 bottom-40 left-[calc(var(--app-sidebar-width,0px)+0.5rem)] z-10 hidden flex-col justify-center @[56rem]:flex"
    : "pointer-events-none absolute top-16 bottom-40 left-2 z-10 hidden flex-col justify-center @[56rem]:flex"}
>
  <div
    bind:this={list}
    class="pointer-events-auto relative flex min-h-0 max-h-full flex-col overflow-clip py-1"
  >
    {#each groups as group, index (group[0]!.id)}
      {@const item = group[0]!}
      {@const ids = groupIds(group)}
      {@const grouped = ids.length > 1}
      <Popover.Root
        open={openId === item.id}
        onOpenChange={(value) => (openId = value ? item.id : "")}
      >
        <Popover.Trigger>
          {#snippet child({ props })}
            <ListButton
              {...props}
              variant="turn"
              data-turn-id={item.id}
              data-turn-index={index}
              data-turn-role={group.length === 1 ? item.role : "group"}
              data-turn-volume={volume(ids.length)}
              aria-label={groupLabel(group)}
              aria-current={index === currentGroup ? "location" : undefined}
              tabindex={index === currentGroup ||
              (currentGroup < 0 && index === 0)
                ? 0
                : -1}
              title={groupLabel(group)}
              onclick={() => {
                if (grouped) openId = openId === item.id ? "" : item.id;
                else onselect(item.id);
              }}
              onkeydown={(event) => move(event, index)}
            >
              <span aria-hidden="true"></span>
            </ListButton>
          {/snippet}
        </Popover.Trigger>
        {#if grouped}
          <Popover.Content
            role="dialog"
            aria-label={groupLabel(group)}
            side="right"
            align="start"
            class="max-h-[min(24rem,var(--bits-popover-content-available-height))]"
            onCloseAutoFocus={(event) => {
              event.preventDefault();
              list
                ?.querySelector<HTMLElement>(`[data-turn-index="${index}"]`)
                ?.focus({ preventScroll: true });
            }}
          >
            <Popover.Title
              >{m.s3_turn_directory_title({ count: ids.length })}</Popover.Title
            >
            <ScrollArea
              viewportClasses="max-h-[min(18rem,calc(var(--bits-popover-content-available-height)_-_5rem))]"
            >
              {#each group as member (member.id)}
                {#each member.messageIds as id (id)}
                  {@const message = messages.get(id)}
                  {@const summary =
                    message?.text.replace(/\s+/g, " ").trim().slice(0, 160) ||
                    member.summary}
                  <ListButton
                    variant="navigation-action"
                    class="w-full"
                    aria-label={label({ ...member, summary, messageIds: [id] })}
                    aria-current={id === activeId ? "page" : undefined}
                    title={message?.text || member.summary}
                    onclick={() => choose(id)}
                  >
                    <span class="min-w-0 truncate"
                      >{label({ ...member, summary, messageIds: [id] })}</span
                    >
                  </ListButton>
                {/each}
              {/each}
            </ScrollArea>
          </Popover.Content>
        {/if}
      </Popover.Root>
    {/each}
  </div>
</nav>
