<script lang="ts">
  import { ListButton } from "$lib/components/ui/list-button";
  import LaunchConfigIcon from "../../src/components/LaunchConfigIcon.svelte";
  import { chats, statusLabels } from "./data";
  import StatusText from "./StatusText.svelte";

  // Prototype of the chat-mode sidebar content: every chat session flat by
  // latest activity. The current row uses the navigation variant's line
  // marker; the implementation keys it on aria-current (design.md §8).
  let {
    current,
    onopen,
  }: { current: string | null; onopen: (id: string) => void } = $props();
</script>

<h2 class="px-3 pt-3 pb-1 text-xs text-muted-foreground">最近</h2>
<ul class="space-y-0.5" aria-label="聊天历史">
  {#each chats as chat (chat.id)}
    <li>
      <ListButton
        variant="navigation"
        aria-pressed={chat.id === current}
        title={`${chat.title}（${chat.agentTitle} · ${statusLabels[chat.status]}）`}
        onclick={() => onopen(chat.id)}
      >
        <LaunchConfigIcon icon={chat.icon} size="sm" />
        <span
          class={`min-w-0 flex-1 truncate ${chat.status === "ended" ? "text-muted-foreground" : ""}`}
          >{chat.title}</span
        >
        <StatusText status={chat.status} />
      </ListButton>
    </li>
  {/each}
</ul>
