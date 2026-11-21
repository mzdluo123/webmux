<script lang="ts" module>
  import type * as TerminalModule from "./pages/Terminal.svelte";
  type SessionPageModule = typeof TerminalModule;
  // Kept after the first visit. A resolved import still paints `{#await}`'s
  // pending branch, so later opens must render the module directly.
  let sessionPage: SessionPageModule | undefined;

  function cachedSessionPage() {
    return sessionPage;
  }

  function loadSessionPage() {
    const pending = import("./pages/Terminal.svelte");
    void pending.then((mod) => {
      sessionPage = mod;
    });
    return pending;
  }
</script>

<script lang="ts">
  import AppShellSkeleton from "./components/shell/AppShellSkeleton.svelte";
  import BootSplash from "./components/shell/BootSplash.svelte";
  import ConnectionGate from "./components/shell/ConnectionGate.svelte";
  import RelayAccessOverlay from "./components/shell/RelayAccessOverlay.svelte";
  import AppOverlays from "./components/shell/AppOverlays.svelte";
  import { onMount } from "svelte";
  import { navigate } from "./platform/api";
  import { Button } from "$lib/components/ui/button";
  import LazyPage from "./components/shared/LazyPage.svelte";
  import AppShell from "./components/shell/AppShell.svelte";
  import FeaturePage from "./features/FeaturePage.svelte";
  import FeatureLoadNotices from "./features/FeatureLoadNotices.svelte";
  import SessionSwitcher from "./components/shell/SessionSwitcher.svelte";
  import type TerminalPage from "./pages/Terminal.svelte";
  import {
    watchSessionSwitcherShortcut,
    provideSessionSwitcher,
  } from "./hooks/shared/session-switcher";
  import SessionList from "./pages/SessionList.svelte";
  import {
    matchRoute,
    routeFeature,
    shellRouteFor,
  } from "./hooks/shared/route";
  import { openSession, shellUi } from "./hooks/shared/shell.svelte";
  import { createTerminalDrafts } from "./domains/terminal/terminal-draft.svelte";
  import { activeBrowserDraftStorage } from "./domains/terminal/draft-storage";
  import { workbenchLayout } from "./domains/workspace/workbench-layout.svelte";
  import { sessionPins } from "./domains/sessions/session-pins.svelte";
  import { sessionListSnapshot } from "./hooks/sessions/session-list.svelte";
  import {
    featureEnabled,
    featureAvailable,
  } from "./platform/feature-host.svelte";
  import { featureContributions } from "./features/registry.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import {
    relayBlocked,
    relayGate,
    relayReconnecting,
  } from "./platform/relay-auth.svelte";
  import { useRouteFocus } from "./hooks/shared/route-focus";
  import { createAppBootstrap } from "./app/app-bootstrap.svelte";
  import { createAppLinkHandler } from "./app/app-link-navigation";
  import { m } from "$lib/paraglide/messages.js";

  const focusRoute = useRouteFocus();
  const drafts = createTerminalDrafts({ storage: activeBrowserDraftStorage() });
  let uploadNotice = $state("");

  function normalizeLegacyNewPath(pathname: string): string {
    if (pathname !== "/new") return pathname;
    workbenchLayout.value = "queue";
    workbenchLayout.attentionOnly = false;
    workbenchLayout.queueView = "composer";
    history.replaceState(null, "", "/");
    shellUi.requestQuickStartFocus("auto");
    return "/";
  }

  let path = $state(normalizeLegacyNewPath(location.pathname));
  let search = $state(location.search);
  const route = $derived(matchRoute(path));
  const contributions = $derived(featureContributions.entries);
  const requiredFeature = $derived(routeFeature(route));
  const routeUnavailable = $derived(
    requiredFeature &&
      (!featureEnabled(requiredFeature) ||
        featureContributions.errors[requiredFeature] ||
        (route.name === "git" && !featureAvailable("workspace.git-workbench"))),
  );
  const sessionId = $derived(route.name === "terminal" ? route.id : "");
  const shellRoute = $derived(shellRouteFor(route));
  let shell = $state<AppShell>();
  let terminal = $state<TerminalPage>();
  let switcherOpen = $state(false);
  let switcherInvoker: HTMLElement | null = null;
  let pendingSessionOpen = $state("");

  const boot = createAppBootstrap({
    route: () => route,
    preloadSessionPage: () => {
      if (!cachedSessionPage()) void loadSessionPage().catch(() => undefined);
    },
    onPathNormalized: () => {
      path = normalizeLegacyNewPath(location.pathname);
    },
    onClearUiForAuth: (relay) => {
      switcherOpen = false;
      if (relay) return;
      drafts.clearAll();
      uploadNotice = "";
      pendingSessionOpen = "";
      switcherInvoker = null;
    },
    onRebuilding: (value) => {
      if (value) switcherOpen = false;
    },
  });

  const canSwitch = $derived(
    boot.ready &&
      !relayGate() &&
      (shellRoute !== null ||
        route.name === "settings" ||
        route.name === "settings-tailscale" ||
        route.name === "settings-tunnel" ||
        route.name === "settings-relay"),
  );

  function openSwitcher(invoker: HTMLElement | null) {
    switcherInvoker = invoker;
    switcherOpen = true;
  }
  provideSessionSwitcher(openSwitcher);

  function restoreSwitcherFocus() {
    if (switcherInvoker?.isConnected) switcherInvoker.focus();
    switcherInvoker = null;
  }

  function selectSession(id: string, messageId?: string) {
    switcherInvoker = null;
    if (messageId) {
      const anchor = terminal?.activateSession(id) ? sessionId : id;
      const query = new URLSearchParams({
        message: messageId,
        messageSession: id,
      });
      navigate(`/s/${anchor}?${query}`);
      return;
    }
    if (terminal?.activateSession(id)) return;
    if (id === sessionId && terminal) {
      terminal.reopenSession(id);
      return;
    }
    pendingSessionOpen = id;
    openSession({ id });
  }

  // The list already names the side. The loading line must not call a chat
  // session a terminal while this page's chunk is still arriving.
  const pageLabel = $derived(
    sessionListSnapshot()?.find((session) => session.id === sessionId)?.view ===
      "chat"
      ? m.s1_page_chat()
      : m.s1_page_terminal(),
  );
  // Re-read the cached chunk when the open session changes. Filling the
  // cache during this visit must not replace the page already on screen.
  const openedPage = $derived.by(() => {
    void sessionId;
    return cachedSessionPage();
  });

  onMount(() => {
    const change = () => {
      const nextPath = normalizeLegacyNewPath(location.pathname);
      const pathChanged = path !== nextPath;
      if (pathChanged) switcherOpen = false;
      pendingSessionOpen = "";
      path = nextPath;
      search = location.search;
      if (pathChanged) focusRoute();
    };
    const link = createAppLinkHandler({
      sessionId: () => sessionId,
      activateSession: (id) => Boolean(terminal?.activateSession(id)),
      reopenSession: (id) => {
        if (!terminal) return false;
        terminal.reopenSession(id);
        return true;
      },
      setPendingSessionOpen: (id) => (pendingSessionOpen = id),
    });
    document.addEventListener("click", link);
    window.addEventListener("popstate", change);
    const disconnectShortcut = watchSessionSwitcherShortcut(
      () => canSwitch && !switcherOpen,
      openSwitcher,
    );
    const disconnectPins = sessionPins.connect();
    const stopBoot = boot.start();
    return () => {
      stopBoot();
      disconnectPins();
      drafts.clearAll();
      document.removeEventListener("click", link);
      window.removeEventListener("popstate", change);
      disconnectShortcut();
    };
  });
</script>

<AppOverlays ready={boot.ready} bind:uploadNotice />

<RelayAccessOverlay
  checking={boot.reconnectChecking}
  returnTo={path + search + location.hash}
  hasShell={boot.ready &&
    !boot.rebuildingFeatures &&
    !routeUnavailable &&
    shellRoute !== null}
  onretry={boot.retryRelayReconnect}
/>

<div class="contents" inert={relayGate()} aria-hidden={relayGate()}>
  {#if !boot.ready}
    {#if !relayBlocked()}
      {#if boot.gate === "probing"}
        {#if boot.optimisticBoot && shellRoute !== null}
          <AppShellSkeleton terminal={route.name === "terminal"} />
        {:else}
          <BootSplash />
        {/if}
      {:else}
        <ConnectionGate
          mode={boot.gate}
          checking={boot.checking}
          error={boot.authFailed ? m.s1_token_invalid() : boot.error}
          bind:token={boot.token}
          onconnect={boot.connectWithToken}
          onretry={boot.retryConnection}
          onusetoken={boot.switchToTokenGate}
        />
      {/if}
    {/if}
  {:else if boot.rebuildingFeatures}
    <main class="mx-auto max-w-xl p-6" aria-busy="true">
      <InlineNotice>{m.s1_reconnecting_capabilities()}</InlineNotice>
    </main>
  {:else if routeUnavailable}
    <main class="mx-auto max-w-xl space-y-4 p-6">
      <h1 class="text-2xl font-semibold">{m.s1_feature_disabled()}</h1>
      <InlineNotice tone="warning"
        >{requiredFeature && featureContributions.errors[requiredFeature]
          ? m.s1_feature_load_failed({
              feature: requiredFeature,
              error: featureContributions.errors[requiredFeature],
            })
          : route.name === "git" && featureEnabled("workspace")
            ? m.s1_git_workbench_requires_features()
            : m.s1_page_requires_feature({
                feature: String(requiredFeature ?? ""),
              })}</InlineNotice
      >
      <Button href="/">{m.s1_back_to_sessions()}</Button>
    </main>
  {:else if route.name === "settings"}
    <LazyPage
      load={() => import("./pages/Settings.svelte")}
      label={m.s1_settings()}
    >
      {#snippet children(page)}<page.default />{/snippet}
    </LazyPage>
  {:else if route.name === "settings-relay"}
    <LazyPage
      load={() => import("./pages/RelaySettingsPage.svelte")}
      label={m.relay_title()}
    >
      {#snippet children(page)}<page.default />{/snippet}
    </LazyPage>
  {:else if !shellRoute}
    <FeaturePage {route} {search} />
  {:else if shellRoute}
    <AppShell
      bind:this={shell}
      route={shellRoute}
      {drafts}
      reconnection={relayReconnecting()
        ? {
            checking: boot.reconnectChecking,
            returnTo: path + search + location.hash,
            onretry: boot.retryRelayReconnect,
          }
        : undefined}
    >
      {#if sessionId}
        {#if openedPage}
          {@render session(openedPage)}
        {:else}
          <LazyPage
            load={loadSessionPage}
            label={pageLabel}
            errorMessage={(loadError) =>
              loadError instanceof Error
                ? loadError.message
                : m.s1_page_load_failed({ label: pageLabel })}
          >
            {#snippet children(page)}
              {@render session(page)}
            {/snippet}
          </LazyPage>
        {/if}
      {:else if route.name !== "sessions" || contributions.workspace}
        <FeaturePage {route} {search} />
      {:else}
        <SessionList />
      {/if}
    </AppShell>
  {/if}
</div>

{#if boot.ready && (boot.discoveryError || boot.discoveryFailureCause)}
  <div class="fixed inset-x-2 bottom-2 z-50">
    <InlineNotice tone="warning" surface="floating"
      >{boot.discoveryError ||
        m.s1_discovery_failed({
          error: boot.discoveryFailureCause,
        })}</InlineNotice
    >
  </div>
{/if}

{#if boot.ready}
  <FeatureLoadNotices errors={featureContributions.errors} />
{/if}

{#snippet session(page: SessionPageModule)}
  {#key sessionId}
    <page.default
      bind:this={terminal}
      id={sessionId}
      openTarget={pendingSessionOpen === sessionId ? pendingSessionOpen : ""}
      onOpenTargetHandled={() => {
        if (pendingSessionOpen === sessionId) pendingSessionOpen = "";
      }}
      {drafts}
      onUploadNotice={(message: string) => {
        if (boot.ready) uploadNotice = message;
      }}
    />
  {/key}
{/snippet}

{#if canSwitch}
  <SessionSwitcher
    bind:open={switcherOpen}
    source={shell?.sessionSwitcherSource()}
    onselect={selectSession}
    oncancel={restoreSwitcherFocus}
  />
{/if}
