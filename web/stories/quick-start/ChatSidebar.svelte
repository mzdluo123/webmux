<script lang="ts">
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconLayoutSidebarLeftCollapse from "~icons/tabler/layout-sidebar-left-collapse";
  import IconMessagePlus from "~icons/tabler/message-plus";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import LaunchConfigIcon from "../../src/components/LaunchConfigIcon.svelte";
  import { history, statusLabels, type ChatStatus } from "./history";

  // Prototype of the chat sidebar: "新聊天" and the overview on top, then
  // every chat session flat by latest activity, each with its agent icon and
  // status. The current row uses the navigation variant's line marker; the
  // implementation keys it on aria-current (see design.md §7).
  let {
    current,
    onnavigate,
    oncollapse,
  }: {
    current: string | null;
    onnavigate: (id: string | null) => void;
    /** Desktop only; the drawer closes on its own. */
    oncollapse?: () => void;
  } = $props();
  const tones: Record<ChatStatus, string> = {
    working: "text-success",
    waiting: "text-warning",
    idle: "text-muted-foreground",
    ended: "text-muted-foreground",
  };
</script>

<nav class="flex h-full min-h-0 flex-col gap-1 p-2" aria-label="聊天">
  <div class="flex items-center gap-1">
    <ListButton
      variant="navigation"
      aria-pressed={current === null}
      onclick={() => onnavigate(null)}
      ><IconMessagePlus aria-hidden="true" />新聊天</ListButton
    >
    {#if oncollapse}
      <Button
        variant="ghost"
        size="icon-touch"
        aria-label="收起侧栏"
        title="收起侧栏"
        onclick={oncollapse}
        ><IconLayoutSidebarLeftCollapse aria-hidden="true" /></Button
      >
    {/if}
  </div>
  <ListButton variant="navigation"
    ><IconLayoutDashboard aria-hidden="true" />会话总览</ListButton
  >
  <h2 class="px-3 pt-3 pb-1 text-xs text-muted-foreground">最近</h2>
  <ul class="min-h-0 flex-1 space-y-0.5 overflow-y-auto">
    {#each history as chat (chat.id)}
      <li>
        <ListButton
          variant="navigation"
          aria-pressed={chat.id === current}
          title={`${chat.title}（${chat.agentTitle} · ${statusLabels[chat.status]}）`}
          onclick={() => onnavigate(chat.id)}
        >
          <LaunchConfigIcon icon={chat.agentIcon} size="sm" />
          <span
            class={`min-w-0 flex-1 truncate ${chat.status === "ended" ? "text-muted-foreground" : ""}`}
            >{chat.title}</span
          >
          <span class={`shrink-0 text-xs font-normal ${tones[chat.status]}`}
            >{statusLabels[chat.status]}</span
          >
        </ListButton>
      </li>
    {/each}
  </ul>
</nav>
