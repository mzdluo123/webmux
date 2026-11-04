<script lang="ts">
  import { untrack, type Snippet } from "svelte";
  import IconLayoutDashboard from "~icons/tabler/layout-dashboard";
  import IconLayoutSidebarLeftExpand from "~icons/tabler/layout-sidebar-left-expand";
  import IconMessageCircle from "~icons/tabler/message-circle";
  import IconMessagePlus from "~icons/tabler/message-plus";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import type { Mode } from "./data";
  import Sidebar from "./Sidebar.svelte";

  // Prototype of the app shell around the home page, quick start and
  // session pages: the sidebar (a rail when collapsed, a left drawer below
  // `sm`) and the main area. Switching mode on a home page swaps both the
  // sidebar and the main area; on a session page (`session` set) only the
  // sidebar changes. Opening a session only reports where it would go.
  type View = { mode: Mode; scope: string; worktree?: string };
  let {
    mode: initialMode = "workbench",
    session = null,
    children,
  }: {
    mode?: Mode;
    /** Set on a session page: the session stays open across mode switches. */
    session?: string | null;
    children: Snippet<[View, () => void]>;
  } = $props();
  let mode = $state<Mode>(untrack(() => initialMode));
  let scope = $state("all");
  let worktree = $state<string | undefined>();
  let collapsed = $state(false);
  let drawerOpen = $state(false);
  let notice = $state("");
  const current = $derived(session);
  function setMode(next: Mode) {
    mode = next;
    notice = "";
  }
  function newItem() {
    drawerOpen = false;
    notice =
      mode === "chat"
        ? session
          ? "原型：将在当前标签页进入快速开始"
          : ""
        : "原型：将打开现有的新建会话弹窗";
  }
  function setScope(next: string, tree?: string) {
    drawerOpen = false;
    scope = next;
    worktree = tree;
    notice = session ? "原型：将在当前标签页进入工作台首页的对应视图" : "";
  }
  function open(id: string) {
    drawerOpen = false;
    notice = `原型：将在当前标签页进入会话 ${id}`;
  }
</script>

{#snippet sidebar(collapsible: boolean)}
  <Sidebar
    {mode}
    {scope}
    {current}
    onmode={setMode}
    onnew={newItem}
    onscope={setScope}
    onopen={open}
    oncollapse={collapsible ? () => (collapsed = true) : undefined}
  />
{/snippet}

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
        aria-label="聊天模式"
        aria-pressed={mode === "chat"}
        title="聊天模式"
        onclick={() => setMode("chat")}
        ><IconMessageCircle aria-hidden="true" /></Button
      >
      <Button
        variant="ghost"
        size="icon-touch"
        aria-label="工作台模式"
        aria-pressed={mode === "workbench"}
        title="工作台模式"
        onclick={() => setMode("workbench")}
        ><IconLayoutDashboard aria-hidden="true" /></Button
      >
      <Button
        variant="ghost"
        size="icon-touch"
        aria-label={mode === "chat" ? "新聊天" : "新建会话"}
        title={mode === "chat" ? "新聊天" : "新建会话"}
        onclick={newItem}
        >{#if mode === "chat"}<IconMessagePlus
            aria-hidden="true"
          />{:else}<IconPlus aria-hidden="true" />{/if}</Button
      >
    </div>
  {:else}
    <aside class="w-72 shrink-0 border-r max-sm:hidden">
      {@render sidebar(true)}
    </aside>
  {/if}
  <!-- transform makes this box the containing block of the session page's
       fixed root, standing in for the --app-sidebar-width offset. -->
  <div class="relative min-w-0 flex-1 transform-gpu">
    {@render children({ mode, scope, worktree }, () => (drawerOpen = true))}
    {#if notice}<p
        role="status"
        class="absolute top-14 left-1/2 w-max max-w-[90%] -translate-x-1/2 rounded-md border bg-popover px-3 py-1.5 text-sm shadow-sm"
      >
        {notice}
      </p>{/if}
  </div>
</div>

<Drawer.Root direction="left" bind:open={drawerOpen}>
  <Drawer.Content class="w-80 max-w-[85vw]">
    <Drawer.Title class="sr-only">导航</Drawer.Title>
    {@render sidebar(false)}
  </Drawer.Content>
</Drawer.Root>
