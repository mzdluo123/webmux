<script lang="ts">
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { getShell } from "../hooks/shared/shell.svelte";
  import SidebarButton from "../components/shell/SidebarButton.svelte";
  import SessionSwitcherTrigger from "../components/shell/SessionSwitcherTrigger.svelte";
  import SessionRow from "../components/sessions/SessionRow.svelte";
  import InlineNotice from "../components/shared/InlineNotice.svelte";
  import { matchesSessionQuery } from "../domains/sessions/session-location";
  import {
    resetBrowserChrome,
    setBrowserChrome,
  } from "../platform/browser-chrome";

  const shell = getShell();
  let query = $state("");
  let now = $state(Date.now());
  const sessions = $derived(
    shell.sessions.sessions.filter((session) => session.state === "active"),
  );
  const visible = $derived(
    sessions.filter((session) =>
      matchesSessionQuery(session, query.trim().toLocaleLowerCase()),
    ),
  );
  $effect(() =>
    setBrowserChrome(
      "WebMux · 会话",
      sessions.some((session) => session.status === "running"),
    ),
  );
  onMount(() => {
    const timer = setInterval(() => {
      now = Date.now();
    }, 30_000);
    return () => {
      clearInterval(timer);
      resetBrowserChrome();
    };
  });
</script>

<main class="mx-auto w-full max-w-5xl space-y-6 p-4 sm:p-6">
  <header class="flex items-center gap-2">
    <SidebarButton />
    <h1 class="mr-auto text-2xl font-semibold">会话</h1>
    <SessionSwitcherTrigger size="icon-touch" />
    <Button onclick={() => shell.start()}>新建会话</Button>
  </header>
  <Input
    aria-label="搜索会话"
    placeholder="搜索会话或目录"
    bind:value={query}
  />
  {#if shell.sessions.error}<InlineNotice tone="danger"
      >{shell.sessions.error}</InlineNotice
    >{/if}
  {#if shell.sessions.loading}
    <p role="status" class="text-sm text-muted-foreground">正在加载会话…</p>
  {:else if !sessions.length}
    <p class="text-sm text-muted-foreground">暂无会话。新建会话以启动终端。</p>
  {:else if !visible.length}
    <p class="text-sm text-muted-foreground">没有匹配的会话。</p>
  {:else}
    <div role="list" aria-label="会话列表" class="space-y-3">
      {#each visible as session (session.id)}
        <SessionRow {session} {now} onclose={shell.close} />
      {/each}
    </div>
  {/if}
</main>
