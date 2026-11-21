<script lang="ts">
  import { onMount } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import SessionSearch from "../components/sessions/SessionSearch.svelte";
  import * as Item from "$lib/components/ui/item";
  import IconPlus from "~icons/tabler/plus";
  import { getShell } from "../hooks/shared/shell.svelte";
  import SidebarButton from "../components/shell/SidebarButton.svelte";
  import SessionSwitcherTrigger from "../components/shell/SessionSwitcherTrigger.svelte";
  import SessionViewControls from "../components/sessions/SessionViewControls.svelte";
  import { homeSessionView } from "../domains/sessions/home-session-view.svelte";
  import DormantSessionLinks from "../components/sessions/DormantSessionLinks.svelte";
  import SessionRow from "../components/sessions/SessionRow.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import SessionListEmpty from "../components/sessions/SessionListEmpty.svelte";
  import HomeUpdateNotice from "../components/shared/HomeUpdateNotice.svelte";
  import { matchesSessionQuery } from "../domains/sessions/session-location";
  import { resumesOnOpen } from "../domains/sessions/archived";
  import {
    resetBrowserChrome,
    setBrowserChrome,
  } from "../platform/browser-chrome";
  import { m } from "$lib/paraglide/messages.js";

  const shell = getShell();
  let query = $state("");
  let searchInput = $state<HTMLInputElement | null>(null);
  const compact = $derived(homeSessionView.compact);
  function clearSearch() {
    query = "";
    searchInput?.focus();
  }
  let now = $state(Date.now());
  const sessions = $derived(
    shell.sessions.sessions.filter((session) => session.state === "active"),
  );
  const visible = $derived(
    sessions.filter((session) =>
      matchesSessionQuery(session, query.trim().toLocaleLowerCase()),
    ),
  );
  const dormant = $derived(
    (shell.archived?.records ?? []).filter(
      (record) =>
        resumesOnOpen(record) &&
        matchesSessionQuery(record, query.trim().toLocaleLowerCase()),
    ),
  );
  $effect(() =>
    setBrowserChrome(
      m.s1_chrome_sessions(),
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

<div class="safe-area-page">
  <main class="mx-auto w-full max-w-6xl px-4 pb-8 sm:px-6 lg:px-8">
    <header
      class="flex flex-wrap items-center gap-x-3 gap-y-2 pt-2 pb-4 sm:pt-8 sm:pb-6"
    >
      <SidebarButton />
      <h1 class="mr-auto text-lg font-semibold tracking-tight sm:text-3xl">
        {m.s1_sessions()}
      </h1>
      <SessionSwitcherTrigger size="icon-touch" />
      <Button class="w-full sm:w-auto" onclick={() => shell.start()}>
        <IconPlus aria-hidden="true" />{m.s1_new_session()}
      </Button>
    </header>
    <HomeUpdateNotice update={shell.setup.value.pendingUpdate} />
    <div
      class="mb-6 flex flex-wrap items-center gap-2 border-b pb-4 sm:mb-8 sm:gap-4"
    >
      <div class="max-w-full min-w-[min(100%,10rem)] flex-1">
        <SessionSearch
          placeholder={m.s1_search_sessions_or_dirs()}
          bind:input={searchInput}
          bind:query
          onclear={clearSearch}
        />
      </div>
      <SessionViewControls
        {compact}
        onchange={(value) => (homeSessionView.compact = value)}
      />
    </div>
    {#if shell.sessions.error}<InlineNotice tone="danger"
        >{shell.sessions.error}</InlineNotice
      >{/if}
    {#if visible.length}
      <Item.Group
        aria-label={m.s1_session_list_aria()}
        variant={compact ? "panel" : "default"}
        class={compact ? undefined : "grid session-preview-columns gap-3"}
      >
        {#each visible as session (session.id)}
          <SessionRow {session} {now} {compact} onclose={shell.close} />
        {/each}
      </Item.Group>
    {/if}
    <DormantSessionLinks records={dormant} />
    {#if shell.sessions.loading}
      <p role="status" class="text-sm text-muted-foreground">
        {m.s1_loading_sessions_list()}
      </p>
    {:else if !visible.length && !dormant.length && (sessions.length || !shell.sessions.error)}
      {#if query.trim()}
        <SessionListEmpty filtered description="" onclear={clearSearch} />
      {:else}
        <SessionListEmpty
          filtered={false}
          description={m.s1_empty_sessions_core()}
          onclear={clearSearch}
        />
      {/if}
    {/if}
  </main>
</div>
