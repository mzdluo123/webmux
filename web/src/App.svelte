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
  import ConnectionGate from "./components/shell/ConnectionGate.svelte";
  import RelayAccessOverlay from "./components/shell/RelayAccessOverlay.svelte";
  import AppOverlays from "./components/shell/AppOverlays.svelte";
  import {
    startSettings,
    stopSettings,
  } from "./domains/settings/settings.svelte";
  import { errorMessage } from "./platform/error";
  import { onMount } from "svelte";
  import {
    api,
    consumeTokenFragment,
    navigate,
    store,
    stored,
  } from "./platform/api";
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
  import { openSession } from "./hooks/shared/shell.svelte";
  import { createTerminalDrafts } from "./domains/terminal/terminal-draft.svelte";
  import { activeBrowserDraftStorage } from "./domains/terminal/draft-storage";
  import { clearHomeCache } from "./domains/workspace/home-cache";
  import { sessionPins } from "./domains/sessions/session-pins.svelte";
  import {
    clearSessionListSnapshot,
    sessionListSnapshot,
  } from "./hooks/sessions/session-list.svelte";
  import { CapabilitiesSchema } from "./contracts/feature-schemas";
  import {
    featureEnabled,
    featureAvailable,
    setFeatureCapabilities,
    resetFeatureCapabilities,
  } from "./platform/feature-host.svelte";
  import {
    loadFeatureContributions,
    disposeFeatureContributions,
    featureContributions,
  } from "./features/registry.svelte";
  import { watchFeatureHostRefresh } from "./features/host-refresh";
  import { onActiveHostChange } from "./platform/storage";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { clearRelayAuth, resetRelayAuth } from "./platform/relay-auth";
  import {
    relayAuth,
    relayBlocked,
    relayGate,
    relayReconnecting,
  } from "./platform/relay-auth.svelte";
  import { useRelayAuthRecovery } from "./hooks/shared/relay-auth-recovery.svelte";
  import { prepareHostAuthentication } from "./hooks/shared/host-authentication";
  import { useRouteFocus } from "./hooks/shared/route-focus";
  import { createAppLinkHandler } from "./app/app-link-navigation";
  import { m } from "$lib/paraglide/messages.js";

  const focusRoute = useRouteFocus();

  const drafts = createTerminalDrafts({ storage: activeBrowserDraftStorage() });
  let uploadNotice = $state("");

  let path = $state(location.pathname);
  let search = $state(location.search);
  let ready = $state(false);
  let rebuildingFeatures = $state(false);
  let checking = $state(true);
  let token = $state(stored("token"));
  let error = $state("");
  let authFailed = $state(false);
  let discoveryError = $state("");
  let discoveryFailureCause = $state("");
  let bootstrapEpoch = 0;
  let bootstrapAbort: AbortController | undefined;
  const reconnect = useRelayAuthRecovery(
    () => ready,
    () => void check(),
  );
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
  const canSwitch = $derived(
    ready && !relayGate() && (shellRoute !== null || route.name === "settings"),
  );
  let shell = $state<AppShell>();
  let terminal = $state<TerminalPage>();
  let switcherOpen = $state(false);
  let switcherInvoker: HTMLElement | null = null;
  let pendingSessionOpen = $state("");

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
  async function check(preferToken = false) {
    checking = true;
    bootstrapAbort?.abort();
    const controller = new AbortController();
    bootstrapAbort = controller;
    const epoch = ++bootstrapEpoch;
    try {
      disposeFeatureContributions();
      resetFeatureCapabilities();
      if (
        !(await prepareHostAuthentication(
          preferToken,
          controller.signal,
          () => epoch === bootstrapEpoch,
        ))
      ) {
        if (relayAuth.reason === "reconnecting") reconnect.start();
        return;
      }
      discoveryError = "";
      discoveryFailureCause = "";
      try {
        const capabilities = await api(
          "/capabilities",
          CapabilitiesSchema,
          "GET",
          undefined,
          controller.signal,
        );
        if (epoch !== bootstrapEpoch) return;
        setFeatureCapabilities(capabilities);
        path = location.pathname;
        await loadFeatureContributions({
          forwardAuth: route.name === "forward-auth",
          codePopout: route.name === "code",
        });
      } catch (discoveryFailure) {
        if (epoch !== bootstrapEpoch) return;
        resetFeatureCapabilities();
        disposeFeatureContributions();
        discoveryFailureCause = errorMessage(discoveryFailure);
        path = location.pathname;
      }
      if (epoch !== bootstrapEpoch) return;
      startSettings();
      ready = true;
      authFailed = false;
      error = "";
    } catch (err) {
      if (epoch === bootstrapEpoch) {
        authFailed = false;
        error = errorMessage(err);
      }
    } finally {
      if (epoch === bootstrapEpoch) checking = false;
    }
  }

  onMount(() => {
    const change = () => {
      const pathChanged = path !== location.pathname;
      if (pathChanged) switcherOpen = false;
      pendingSessionOpen = "";
      path = location.pathname;
      search = location.search;
      if (pathChanged) focusRoute();
    };
    const auth = () => {
      const relay = relayAuth.relay;
      if (relay && relayAuth.authenticated) clearRelayAuth("reconnecting");
      else if (!relay) {
        reconnect.cancel();
        clearRelayAuth();
      }
      bootstrapEpoch++;
      bootstrapAbort?.abort();
      featureRefresh.cancel();
      stopSettings();
      checking = false;
      switcherOpen = false;
      error = "";
      if (relay) {
        if (relayAuth.reason === "reconnecting") reconnect.start();
        else reconnect.cancel();
        return;
      }
      disposeFeatureContributions();
      rebuildingFeatures = false;
      resetFeatureCapabilities();
      drafts.clearAll();
      clearHomeCache();
      clearSessionListSnapshot();
      uploadNotice = "";
      pendingSessionOpen = "";
      ready = false;
      switcherInvoker = null;
      authFailed = true;
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
    window.addEventListener("webmux:auth", auth);
    window.addEventListener("webmux:relay-check", auth);
    const tokenFromFragment = consumeTokenFragment();
    if (tokenFromFragment) token = stored("token");
    const disconnectPins = sessionPins.connect();
    const disconnectHost = onActiveHostChange(() => {
      resetRelayAuth();
      auth();
      token = stored("token");
      void check();
    });
    const featureRefresh = watchFeatureHostRefresh({
      ready: () => ready && !relayBlocked(),
      epoch: () => bootstrapEpoch,
      context: () => ({
        forwardAuth: route.name === "forward-auth",
        codePopout: route.name === "code",
      }),
      onRebuilding: (value) => {
        rebuildingFeatures = value;
        if (value) switcherOpen = false;
      },
      onError: (message) => (discoveryError = message),
    });
    void check(tokenFromFragment);
    return () => {
      reconnect.cancel();
      disconnectPins();
      disconnectHost();
      featureRefresh.dispose();
      bootstrapAbort?.abort();
      bootstrapEpoch++;
      disposeFeatureContributions();
      resetFeatureCapabilities();
      stopSettings();
      drafts.clearAll();
      document.removeEventListener("click", link);
      window.removeEventListener("popstate", change);
      disconnectShortcut();
      window.removeEventListener("webmux:auth", auth);
      window.removeEventListener("webmux:relay-check", auth);
    };
  });
</script>

<AppOverlays {ready} bind:uploadNotice />

<RelayAccessOverlay
  checking={reconnect.checking}
  returnTo={path + search + location.hash}
  hasShell={ready &&
    !rebuildingFeatures &&
    !routeUnavailable &&
    shellRoute !== null}
  onretry={() => reconnect.start()}
/>

<div class="contents" inert={relayGate()} aria-hidden={relayGate()}>
  {#if !ready}
    {#if !relayBlocked()}
      <ConnectionGate
        {checking}
        error={authFailed ? m.s1_token_invalid() : error}
        bind:token
        onconnect={() => {
          store("token", token);
          void check(true);
        }}
      />
    {/if}
  {:else if rebuildingFeatures}
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
            checking: reconnect.checking,
            returnTo: path + search + location.hash,
            onretry: () => reconnect.start(),
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

{#if ready && (discoveryError || discoveryFailureCause)}
  <div class="fixed inset-x-2 bottom-2 z-50">
    <InlineNotice tone="warning" surface="floating"
      >{discoveryError ||
        m.s1_discovery_failed({ error: discoveryFailureCause })}</InlineNotice
    >
  </div>
{/if}

{#if ready}
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
        if (ready) uploadNotice = message;
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
