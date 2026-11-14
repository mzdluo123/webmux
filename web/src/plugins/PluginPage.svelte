<script lang="ts">
  import type { Route } from "../hooks/shared/route";
  import LazyPage from "../components/shared/LazyPage.svelte";
  import { pluginContributions } from "./registry.svelte";
  let { route, search = "" }: { route: Route; search?: string } = $props();
  const entries = $derived(pluginContributions.entries);
</script>

{#if route.name === "code" && entries.code}
  <LazyPage
    load={entries.code.pages.popout}
    label="代码"
    errorMessage={() => "代码窗口加载失败，请关闭后重新打开。"}
  >
    {#snippet children(page)}<page.default
        id={route.name === "code" ? route.id : ""}
      />{/snippet}
  </LazyPage>
{:else if route.name === "settings-tailscale" && entries.tailscale}
  <LazyPage
    load={entries.tailscale.pages.settings}
    label="Tailscale 设置"
    loadingText="正在加载 Tailscale 设置…"
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "settings-tunnel" && entries.cloudflare}
  <LazyPage load={entries.cloudflare.pages.settings} label="公网访问设置">
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "forward-auth" && entries.forward}
  <LazyPage
    load={entries.forward.pages.auth}
    label="端口转发"
    loadingText="正在打开转发…"
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "handoff" && entries.chat}
  <LazyPage load={entries.chat.pages.handoff} label="交接到新聊天">
    {#snippet children(page)}
      {#key route.name === "handoff" ? route.sourceId : ""}<page.default
          sourceId={route.name === "handoff" ? route.sourceId : ""}
        />{/key}
    {/snippet}
  </LazyPage>
{:else if route.name === "git" && entries.workspace}
  <LazyPage load={entries.workspace.pages.git} label="Git 工作台">
    {#snippet children(page)}<page.default
        repositoryId={route.name === "git" ? route.repositoryId : ""}
        worktreeId={route.name === "git" ? route.worktreeId : undefined}
        {search}
      />{/snippet}
  </LazyPage>
{:else if route.name === "quick-start" && entries.chat}
  <LazyPage load={entries.chat.pages["quick-start"]} label="快速开始">
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "sessions" && entries.workspace}
  <LazyPage load={entries.workspace.pages.sessions} label="工作区">
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{/if}
