<script lang="ts">
  import type { Snippet } from "svelte";
  import IconLayoutSidebarLeftExpand from "~icons/tabler/layout-sidebar-left-expand";
  import IconMessagePlus from "~icons/tabler/message-plus";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import ChatSidebar from "./ChatSidebar.svelte";

  // Prototype of the layout around the quick-start page and hybrid session
  // pages: the sidebar on the left (a rail when collapsed), a left drawer
  // below `sm`. The page gets `openDrawer` for its own top-bar button.
  let {
    current,
    children,
  }: { current: string | null; children: Snippet<[() => void]> } = $props();
  let collapsed = $state(false);
  let drawerOpen = $state(false);
  let notice = $state("");
  function navigate(id: string | null) {
    drawerOpen = false;
    notice = id
      ? `原型：将在当前标签页进入会话 ${id}`
      : "原型：将在当前标签页进入快速开始";
  }
</script>

<div class="flex h-dvh">
  {#if collapsed}
    <div class="flex flex-col items-center gap-1 border-r p-2 max-sm:hidden">
      <Button
        variant="ghost"
        size="icon-touch"
        aria-label="展开侧栏"
        title="展开侧栏"
        onclick={() => (collapsed = false)}
        ><IconLayoutSidebarLeftExpand aria-hidden="true" /></Button
      >
      <Button
        variant="ghost"
        size="icon-touch"
        aria-label="新聊天"
        title="新聊天"
        onclick={() => navigate(null)}
        ><IconMessagePlus aria-hidden="true" /></Button
      >
    </div>
  {:else}
    <aside class="w-64 shrink-0 border-r max-sm:hidden">
      <ChatSidebar
        {current}
        onnavigate={navigate}
        oncollapse={() => (collapsed = true)}
      />
    </aside>
  {/if}
  <!-- transform makes this box the containing block of the session page's
       fixed root, standing in for the sidebar-width offset. -->
  <div class="relative min-w-0 flex-1 transform-gpu">
    {@render children(() => (drawerOpen = true))}
    {#if notice}<p
        role="status"
        class="absolute top-14 left-1/2 -translate-x-1/2 rounded-md border bg-popover px-3 py-1.5 text-sm shadow-sm"
      >
        {notice}
      </p>{/if}
  </div>
</div>

<Drawer.Root direction="left" bind:open={drawerOpen}>
  <Drawer.Content class="w-72">
    <Drawer.Title class="sr-only">聊天</Drawer.Title>
    <ChatSidebar {current} onnavigate={navigate} />
  </Drawer.Content>
</Drawer.Root>
