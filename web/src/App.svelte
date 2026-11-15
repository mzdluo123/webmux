<script lang="ts" module>
  type SessionPageModule = typeof import("./pages/Terminal.svelte");
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
  import {
    startSettings,
    stopSettings,
    settings,
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
    isSessionSwitcherShortcut,
    provideSessionSwitcher,
  } from "./hooks/shared/session-switcher";
  import SessionList from "./pages/SessionList.svelte";
  import { matchRoute, routeFeature, type Route } from "./hooks/shared/route";
  import { enterHome, openSession } from "./hooks/shared/shell.svelte";
  import { createTerminalDrafts } from "./domains/terminal/terminal-draft.svelte";
  import { browserDraftStorage } from "./domains/terminal/draft-storage";
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
    requestFeatureHostRefresh,
  } from "./platform/feature-host.svelte";
  import {
    loadFeatureContributions,
    disposeFeatureContributions,
    featureContributions,
  } from "./features/registry.svelte";
  import { watchFeatureHostRefresh } from "./features/host-refresh";
  import { onActiveHostChange } from "./platform/storage";
  import { useDeviceShell } from "./hooks/shared/device-shell.svelte";
  import { tvInput } from "./hooks/shared/tv-input.svelte";
  import { xrPresentation } from "./hooks/shared/xr-presentation.svelte";
  import InlineNotice from "./components/shared/InlineNotice.svelte";
  import { clearRelayAuth } from "./platform/relay-auth";
  import { prepareHostAuthentication } from "./hooks/shared/host-authentication";
  import { useRouteFocus } from "./hooks/shared/route-focus";
  import { createAppLinkHandler } from "./app/app-link-navigation";
  import { m } from "$lib/paraglide/messages.js";

  useDeviceShell();
  const focusRoute = useRouteFocus();

  const drafts = createTerminalDrafts({ storage: browserDraftStorage() });
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
  const shellRoute = $derived(
    route.name === "sessions" ||
      route.name === "quick-start" ||
      route.name === "handoff" ||
      route.name === "terminal" ||
      route.name === "git"
      ? (route as Extract<
          Route,
          {
            name: "sessions" | "quick-start" | "terminal" | "handoff" | "git";
          }
        >)
      : null,
  );
  const canSwitch = $derived(
    ready && (shellRoute !== null || route.name === "settings"),
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

  function switcherShortcut(event: KeyboardEvent) {
    if (
      xrPresentation.active ||
      !canSwitch ||
      switcherOpen ||
      !isSessionSwitcherShortcut(event) ||
      document.querySelector(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"], [role="menu"][data-state="open"], dialog[open]',
      )
    )
      return;
    // xterm and the pane shortcuts must never receive this chord.
    event.preventDefault();
    event.stopPropagation();
    const active = document.activeElement;
    openSwitcher(active instanceof HTMLElement ? active : null);
  }

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
  // Settings can reconnect while no shell session feed is mounted.
  let settingsHadConnection = false;
  let settingsWasConnected = false;
  $effect(() => {
    const connected = settings.connected;
    if (ready && connected && !settingsWasConnected && settingsHadConnection)
      void requestFeatureHostRefresh();
    if (connected) settingsHadConnection = true;
    settingsWasConnected = connected;
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
      )
        return;
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
        enterHome();
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
        enterHome();
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
      if (ready && !shellRoute) enterHome();
      const pathChanged = path !== location.pathname;
      if (pathChanged) switcherOpen = false;
      pendingSessionOpen = "";
      path = location.pathname;
      search = location.search;
      if (pathChanged) focusRoute();
    };
    const auth = () => {
      clearRelayAuth();
      bootstrapEpoch++;
      bootstrapAbort?.abort();
      featureRefresh.cancel();
      disposeFeatureContributions();
      rebuildingFeatures = false;
      resetFeatureCapabilities();
      stopSettings();
      drafts.clearAll();
      clearHomeCache();
      clearSessionListSnapshot();
      uploadNotice = "";
      pendingSessionOpen = "";
      ready = false;
      switcherOpen = false;
      switcherInvoker = null;
      checking = false;
      authFailed = true;
      error = "";
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
    window.addEventListener("keydown", switcherShortcut, true);
    window.addEventListener("webmux:auth", auth);
    const tokenFromFragment = consumeTokenFragment();
    if (tokenFromFragment) token = stored("token");
    const disconnectPins = sessionPins.connect();
    const disconnectHost = onActiveHostChange(() => {
      auth();
      token = stored("token");
      void check();
    });
    const featureRefresh = watchFeatureHostRefresh({
      ready: () => ready,
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
      window.removeEventListener("keydown", switcherShortcut, true);
      window.removeEventListener("webmux:auth", auth);
    };
  });
</script>

{#if tvInput.message && !xrPresentation.active}
  <div
    class="pointer-events-none fixed inset-x-6 top-6 z-[100] mx-auto w-fit max-w-[calc(100%-3rem)] rounded-lg bg-background shadow-lg"
  >
    <InlineNotice>{tvInput.message}</InlineNotice>
  </div>
{/if}

{#if !ready}
  <ConnectionGate
    {checking}
    error={authFailed ? m.s1_token_invalid() : error}
    bind:token
    onconnect={() => {
      store("token", token);
      void check(true);
    }}
  />
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
  <AppShell bind:this={shell} route={shellRoute} {drafts}>
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

{#if ready && (discoveryError || discoveryFailureCause)}
  <div class="fixed inset-x-2 bottom-2 z-50">
    <InlineNotice tone="warning"
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

{#if ready && uploadNotice}
  <div
    class="fixed inset-x-2 top-2 z-50 flex items-start gap-2 rounded-md border bg-card p-3 text-sm shadow-lg"
    role="alert"
  >
    <p class="min-w-0 flex-1 break-all">{uploadNotice}</p>
    <Button variant="outline" size="sm" onclick={() => (uploadNotice = "")}
      >{m.s1_close()}</Button
    >
  </div>
{/if}
