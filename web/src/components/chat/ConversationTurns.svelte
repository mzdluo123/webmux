<script lang="ts">
  import { ListButton } from "$lib/components/ui/list-button";
  import type { ConversationTurnItem } from "../../domains/chat/conversation-turns";

  let {
    items,
    activeId,
    documentScroll,
    onselect,
  }: {
    items: ConversationTurnItem[];
    activeId: string;
    documentScroll: boolean;
    onselect: (id: string) => void;
  } = $props();

  let list = $state<HTMLDivElement | null>(null);

  const label = (item: ConversationTurnItem) =>
    `${item.turn ? `第 ${item.turn} 轮` : "此前"} · ${
      item.role === "user"
        ? "你"
        : item.messageIds.length > 1
          ? `AI · ${item.messageIds.length} 条消息`
          : "AI"
    }：${item.summary}`;

  const selectedId = $derived(
    items.find((item) => item.messageIds.includes(activeId))?.id ?? "",
  );

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
        next = Math.min(items.length - 1, index + 1);
        break;
      case "Home":
        next = 0;
        break;
      case "End":
        next = items.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    const item = items[next];
    if (!item) return;
    onselect(item.id);
    const selected = list?.querySelector<HTMLElement>(
      `[data-turn-index="${next}"]`,
    );
    selected?.focus({ preventScroll: true });
  }
</script>

<nav
  aria-label="对话轮次"
  class={documentScroll
    ? "pointer-events-none fixed top-16 bottom-40 left-[calc(var(--app-sidebar-width,0px)+0.5rem)] z-10 hidden flex-col justify-center @[56rem]:flex"
    : "pointer-events-none absolute top-16 bottom-40 left-2 z-10 hidden flex-col justify-center @[56rem]:flex"}
>
  <div
    bind:this={list}
    class="pointer-events-auto relative flex min-h-0 max-h-full flex-col overflow-clip py-1"
  >
    {#each items as item, index (item.id)}
      <ListButton
        variant="turn"
        data-turn-id={item.id}
        data-turn-index={index}
        data-turn-role={item.role}
        data-turn-volume={volume(item.messageIds.length)}
        aria-label={label(item)}
        aria-current={item.id === selectedId ? "location" : undefined}
        tabindex={item.id === selectedId || (!selectedId && index === 0)
          ? 0
          : -1}
        title={label(item)}
        onclick={() => onselect(item.id)}
        onkeydown={(event) => move(event, index)}
      >
        <span aria-hidden="true"></span>
      </ListButton>
    {/each}
  </div>
</nav>
