<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { Route } from "../hooks/shared/route";
  import LazyPage from "../components/shared/LazyPage.svelte";
  import { featureContributions } from "./registry.svelte";
  let { route, search = "" }: { route: Route; search?: string } = $props();
  const entries = $derived(featureContributions.entries);
</script>

{#if route.name === "code" && entries.code}
  <LazyPage
    load={entries.code.pages.popout}
    label={m.s1_feature_page_code()}
    errorMessage={() => m.s1_code_popout_load_failed()}
  >
    {#snippet children(page)}<page.default
        id={route.name === "code" ? route.id : ""}
      />{/snippet}
  </LazyPage>
{:else if route.name === "settings-tailscale" && entries.tailscale}
  <LazyPage
    load={entries.tailscale.pages.settings}
    label={m.s1_feature_page_tailscale()}
    loadingText={m.s1_loading_tailscale_settings()}
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "settings-tunnel" && entries.cloudflare}
  <LazyPage
    load={entries.cloudflare.pages.settings}
    label={m.s1_feature_page_public_access()}
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "forward-auth" && entries.forward}
  <LazyPage
    load={entries.forward.pages.auth}
    label={m.s1_port_forwarding()}
    loadingText={m.s1_opening_forward()}
  >
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "handoff" && entries.chat}
  <LazyPage
    load={entries.chat.pages.handoff}
    label={m.s1_handoff_to_new_chat()}
  >
    {#snippet children(page)}
      {#key route.name === "handoff" ? route.sourceId : ""}<page.default
          sourceId={route.name === "handoff" ? route.sourceId : ""}
        />{/key}
    {/snippet}
  </LazyPage>
{:else if route.name === "git" && entries.workspace}
  <LazyPage
    load={entries.workspace.pages.git}
    label={m.s1_feature_page_git_workbench()}
  >
    {#snippet children(page)}<page.default
        repositoryId={route.name === "git" ? route.repositoryId : ""}
        worktreeId={route.name === "git" ? route.worktreeId : undefined}
        {search}
      />{/snippet}
  </LazyPage>
{:else if route.name === "quick-start" && entries.chat}
  <LazyPage load={entries.chat.pages["quick-start"]} label={m.s1_quick_start()}>
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if route.name === "sessions" && entries.workspace}
  <LazyPage load={entries.workspace.pages.sessions} label={m.s1_workspace()}>
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{/if}
