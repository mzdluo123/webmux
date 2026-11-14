<script lang="ts">
  import type { Snippet } from "svelte";
  import SidebarButton from "../shell/SidebarButton.svelte";

  let {
    title,
    reconnecting,
    actions,
  }: {
    title: string;
    reconnecting: boolean;
    actions: Snippet;
  } = $props();
</script>

<!-- Phones reach the sidebar here; wider screens already show the sidebar. -->
<header
  class="app-header flex min-h-(--app-header-height) flex-wrap items-center gap-1 border-b bg-card px-1 sm:hidden"
>
  <SidebarButton />
  <h1 class="min-w-0 truncate text-sm font-medium">{title}</h1>
  {#if reconnecting}
    <span role="status" class="shrink-0 text-xs text-warning"
      >连接已断开，重连中…</span
    >
  {/if}
  <div class="ml-auto flex max-w-full flex-wrap items-center justify-end gap-1">
    {@render actions()}
  </div>
</header>
