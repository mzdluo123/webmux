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
  import { onMount, tick } from "svelte";
  import {
    api,
    consumeTokenFragment,
    navigate,
    store,
    stored,
  } from "./platform/api";
  import { HealthSchema } from "./contracts/schemas";
  import { Button } from "$lib/components/ui/button";
  import LazyPage from "./components/shared/LazyPage.svelte";
  import AppShell from "./components/shell/AppShell.svelte";
  import PluginPage from "./plugins/PluginPage.svelte";
  import PluginLoadNotices from "./plugins/PluginLoadNotices.svelte";
  import SessionSwitcher from "./components/shell/SessionSwitcher.svelte";
  import type TerminalPage from "./pages/Terminal.svelte";
  import {
    isSessionSwitcherShortcut,
    provideSessionSwitcher,
  } from "./hooks/shared/session-switcher";
  import SessionList from "./pages/SessionList.svelte";
  import { matchRoute, routePlugin, type Route } from "./hooks/shared/route";
  import { enterHome, openSession } from "./hooks/shared/shell.svelte";
  import { createTerminalDrafts } from "./domains/terminal/terminal-draft.svelte";
  import { browserDraftStorage } from "./domains/terminal/draft-storage";
  import { clearHomeCache } from "./domains/workspace/home-cache";
  import { sessionPins } from "./domains/sessions/session-pins.svelte";
  import {
    clearSessionListSnapshot,
    sessionListSnapshot,
  } from "./hooks/sessions/session-list.svelte";
  import { CapabilitiesSchema } from "./contracts/plugin-schemas";
  import {
    pluginEnabled,
    pluginAvailable,
    setPluginCapabilities,
    resetPluginCapabilities,
    requestPluginHostRefresh,
  } from "./platform/plugin-host.svelte";
  import {
    loadPluginContributions,
    disposePluginContributions,
    pluginContributions,
  } from "./plugins/registry.svelte";
  import { watchPluginHostRefresh } from "./plugins/host-refresh";
  import { onActiveHostChange } from "./platform/storage";
  import { useDeviceShell } from "./hooks/shared/device-shell.svelte";
  import { tvInput } from "./hooks/shared/tv-input.svelte";
  import { xrPresentation } from "./hooks/shared/xr-presentation.svelte";
  import InlineNotice from "./components/shared/InlineNotice.svelte";

  useDeviceShell();

  const drafts = createTerminalDrafts({ storage: browserDraftStorage() });
  let uploadNotice = $state("");

  let path = $state(location.pathname);
  let search = $state(location.search);
  let ready = $state(false);
  let rebuildingPlugins = $state(false);
  let checking = $state(true);
  let token = $state(stored("token"));
  let error = $state("");
  let discoveryError = $state("");
  let bootstrapEpoch = 0;
  let bootstrapAbort: AbortController | undefined;
  const route = $derived(matchRoute(path));
  const contributions = $derived(pluginContributions.entries);
  const requiredPlugin = $derived(routePlugin(route));
  const routeUnavailable = $derived(
    requiredPlugin &&
      (!pluginEnabled(requiredPlugin) ||
        pluginContributions.errors[requiredPlugin] ||
        (route.name === "git" && !pluginAvailable("workspace.git-workbench"))),
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
      ? "聊天"
      : "终端",
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
      void requestPluginHostRefresh();
    if (connected) settingsHadConnection = true;
    settingsWasConnected = connected;
  });

  async function check() {
    checking = true;
    bootstrapAbort?.abort();
    const controller = new AbortController();
    bootstrapAbort = controller;
    const epoch = ++bootstrapEpoch;
    try {
      disposePluginContributions();
      resetPluginCapabilities();
      await api("/health", HealthSchema, "GET", undefined, controller.signal);
      discoveryError = "";
      try {
        const capabilities = await api(
          "/capabilities",
          CapabilitiesSchema,
          "GET",
          undefined,
          controller.signal,
        );
        if (epoch !== bootstrapEpoch) return;
        setPluginCapabilities(capabilities);
        enterHome();
        path = location.pathname;
        await loadPluginContributions({
          forwardAuth: route.name === "forward-auth",
          codePopout: route.name === "code",
        });
      } catch (discoveryFailure) {
        if (epoch !== bootstrapEpoch) return;
        resetPluginCapabilities();
        disposePluginContributions();
        discoveryError = `无法发现可选功能；终端仍可使用。${errorMessage(discoveryFailure)}`;
        enterHome();
        path = location.pathname;
      }
      if (epoch !== bootstrapEpoch) return;
      startSettings();
      ready = true;
      error = "";
    } catch (err) {
      if (epoch === bootstrapEpoch) error = errorMessage(err);
    } finally {
      if (epoch === bootstrapEpoch) checking = false;
    }
  }

  onMount(() => {
    let focusRoute = false;
    const focusMain = () => {
      if (!focusRoute) return;
      const main = document.querySelector("main");
      if (!main) return;
      focusRoute = false;
      // A page that already placed focus (quick start's input) keeps it.
      if (main.contains(document.activeElement)) return;
      main.tabIndex = -1;
      main.focus({ preventScroll: true });
    };
    const routes = new MutationObserver(focusMain);
    routes.observe(document.getElementById("app")!, {
      childList: true,
      subtree: true,
    });
    const change = () => {
      if (ready && !shellRoute) enterHome();
      const pathChanged = path !== location.pathname;
      if (pathChanged) switcherOpen = false;
      pendingSessionOpen = "";
      path = location.pathname;
      search = location.search;
      if (pathChanged) {
        focusRoute = true;
        void tick().then(focusMain);
      }
    };
    const auth = () => {
      bootstrapEpoch++;
      bootstrapAbort?.abort();
      pluginRefresh.cancel();
      disposePluginContributions();
      rebuildingPlugins = false;
      resetPluginCapabilities();
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
      error = "Token 无效或已更改";
    };
    // Plain anchors to app routes would reload the whole document, which
    // blanks an iOS standalone PWA; route them through history instead.
    const link = (event: MouseEvent) => {
      if (
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      )
        return;
      const anchor =
        event.target instanceof Element ? event.target.closest("a") : null;
      if (!anchor || anchor.hasAttribute("download")) return;
      if (anchor.target && anchor.target !== "_self") return;
      const url = new URL(anchor.href, location.href);
      if (url.origin !== location.origin) return;
      const targetRoute = matchRoute(url.pathname);
      const appRoute = url.pathname === "/" || targetRoute.name !== "sessions";
      if (!appRoute) return;
      // References to an open split pane select it without replacing the
      // layout, including when its ID is the current route's anchor.
      if (targetRoute.name === "terminal" && !url.search && !url.hash) {
        event.preventDefault();
        if (terminal?.activateSession(targetRoute.id)) return;
        if (targetRoute.id === sessionId && terminal) {
          terminal.reopenSession(targetRoute.id);
          return;
        }
        pendingSessionOpen = targetRoute.id;
        navigate(url.pathname);
        return;
      }
      if (url.pathname === location.pathname && url.search === location.search)
        return;
      event.preventDefault();
      navigate(url.pathname + url.search + url.hash);
    };
    document.addEventListener("click", link);
    window.addEventListener("popstate", change);
    window.addEventListener("keydown", switcherShortcut, true);
    window.addEventListener("webmux:auth", auth);
    if (consumeTokenFragment()) token = stored("token");
    const disconnectPins = sessionPins.connect();
    const disconnectHost = onActiveHostChange(() => {
      auth();
      token = stored("token");
      void check();
    });
    const pluginRefresh = watchPluginHostRefresh({
      ready: () => ready,
      epoch: () => bootstrapEpoch,
      context: () => ({
        forwardAuth: route.name === "forward-auth",
        codePopout: route.name === "code",
      }),
      onRebuilding: (value) => {
        rebuildingPlugins = value;
        if (value) switcherOpen = false;
      },
      onError: (message) => (discoveryError = message),
    });
    void check();
    return () => {
      disconnectPins();
      disconnectHost();
      pluginRefresh.dispose();
      bootstrapAbort?.abort();
      bootstrapEpoch++;
      disposePluginContributions();
      resetPluginCapabilities();
      stopSettings();
      routes.disconnect();
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
    {error}
    bind:token
    onconnect={() => {
      store("token", token);
      void check();
    }}
  />
{:else if rebuildingPlugins}
  <main class="mx-auto max-w-xl p-6" aria-busy="true">
    <InlineNotice>实例功能已更改，正在重新连接…</InlineNotice>
  </main>
{:else if routeUnavailable}
  <main class="mx-auto max-w-xl space-y-4 p-6">
    <h1 class="text-2xl font-semibold">功能未启用</h1>
    <InlineNotice tone="warning"
      >{requiredPlugin && pluginContributions.errors[requiredPlugin]
        ? `${requiredPlugin} 加载失败：${pluginContributions.errors[requiredPlugin]}`
        : route.name === "git" && pluginEnabled("workspace")
          ? "Git 工作台需要同时启用工作区与代码插件，并确保服务可用。"
          : `此页面需要 ${requiredPlugin} 插件。请在服务端配置中启用后重启。`}</InlineNotice
    >
    <Button href="/">返回会话</Button>
  </main>
{:else if route.name === "settings"}
  <LazyPage load={() => import("./pages/Settings.svelte")} label="设置">
    {#snippet children(page)}<page.default />{/snippet}
  </LazyPage>
{:else if !shellRoute}
  <PluginPage {route} {search} />
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
              : `${pageLabel}加载失败`}
        >
          {#snippet children(page)}
            {@render session(page)}
          {/snippet}
        </LazyPage>
      {/if}
    {:else if route.name !== "sessions" || contributions.workspace}
      <PluginPage {route} {search} />
    {:else}
      <SessionList />
    {/if}
  </AppShell>
{/if}

{#if ready && discoveryError}
  <div class="fixed inset-x-2 bottom-2 z-50">
    <InlineNotice tone="warning">{discoveryError}</InlineNotice>
  </div>
{/if}

{#if ready}
  <PluginLoadNotices errors={pluginContributions.errors} />
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
      >关闭</Button
    >
  </div>
{/if}
