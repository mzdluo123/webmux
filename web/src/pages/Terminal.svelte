<script lang="ts">
  import { findShell, shellUi } from "../hooks/shared/shell.svelte";
  import { onMount, onDestroy, tick, untrack } from "svelte";
  import { navigate, store, stored } from "../platform/api";
  import type { SessionInfo } from "../contracts/schemas";
  import { errorMessage } from "../platform/error";
  import { isIOS, hasTouchInput } from "../platform/touch-input";
  import { chatDocumentScroll, isStandalone } from "../platform/capabilities";
  import type { ChatScrollState } from "../platform/capabilities";
  import {
    setBrowserChrome,
    resetBrowserChrome,
  } from "../platform/browser-chrome";
  import TerminalSplitMenu from "../components/terminal/TerminalSplitMenu.svelte";
  import TerminalPaneDivider from "../components/terminal/TerminalPaneDivider.svelte";
  import TerminalPaneTabs from "../components/terminal/TerminalPaneTabs.svelte";
  import TerminalPageDialogs from "../components/terminal/TerminalPageDialogs.svelte";
  import TerminalPageNotices from "../components/terminal/TerminalPageNotices.svelte";
  import { useViewport } from "../hooks/shared/viewport.svelte";
  import type { TerminalDrafts } from "../domains/terminal/terminal-draft.svelte";
  import TerminalSessionHost from "../components/terminal/TerminalSessionHost.svelte";
  import {
    MAX_PANES,
    createSplitShellSession,
    directionalPane,
    layoutGeometry,
    paneIds,
    parsePrefixKeydown,
    removePane,
    resizeSplit,
    restoreLayout,
    splitPane,
    type SplitDirection,
    type PaneLayout,
  } from "../domains/terminal/terminal-layout";
  import { useTerminalPageFeed } from "../hooks/terminal/terminal-page-feed.svelte";
  import { retainTerminalDrafts } from "../domains/terminal/terminal-draft-owners";
  import {
    loadTerminalPlugins,
    availableTerminalPlugins,
    type TerminalPlugins,
  } from "../plugins/terminal";
  import TerminalPluginScope from "../components/terminal/TerminalPluginScope.svelte";
  import { allowAutomaticInputFocus } from "../hooks/shared/input-focus";

  let {
    id,
    openTarget = "",
    onOpenTargetHandled,
    drafts,
    onUploadNotice,
  }: {
    id: string;
    openTarget?: string;
    onOpenTargetHandled?: () => void;
    drafts: TerminalDrafts;
    onUploadNotice: (message: string) => void;
  } = $props();
  const shell = findShell();
  const key = untrack(() => `layout.${id}`);
  let layout = $state<PaneLayout | undefined>(
    untrack(() => restoreLayout(stored(key), id)),
  );
  let active = $state(
    untrack(() => {
      const saved = stored(`${key}.active`);
      return paneIds(layout!).includes(saved) ? saved : paneIds(layout!)[0]!;
    }),
  );
  $effect(() => {
    shell?.setActivePane(active);
    return () => shell?.setActivePane("");
  });

  let info = $state<Record<string, SessionInfo>>({});
  let connections = $state<Record<string, string>>({});
  let revisions = $state<Record<string, number>>({});
  let zoomed = $state(false);
  let prefix = $state(false);
  let busy = $state(false);
  let error = $state("");
  let plugins = $state<TerminalPlugins>();
  const availablePlugins = $derived(
    plugins ? availableTerminalPlugins(plugins) : undefined,
  );
  let pluginScope = $state<TerminalPluginScope>();
  let observed = $state<string[]>([]);
  let immersive = $state(false);
  onMount(() => {
    let disposed = false;
    void loadTerminalPlugins()
      .then((value) => {
        if (!disposed) plugins = value;
      })
      .catch((cause) => {
        if (!disposed) error = errorMessage(cause);
      });
    return () => {
      disposed = true;
    };
  });
  let choosing = $state(false);
  let creating = $state<SessionInfo | null>(null);
  const splitBusy = $derived(busy || creating !== null);
  function createSession(target: string) {
    if (splitBusy || ids.length >= MAX_PANES || !info[target]) return;
    active = target;
    creating = info[target]!;
  }
  let width = $state(0);
  let height = $state(0);
  let workspace = $state<HTMLDivElement>();
  let mounted = false;
  const terminals = $state<Record<string, TerminalSessionHost | undefined>>({});
  let scrollStates = $state<Record<string, ChatScrollState>>({});
  const ids = $derived(layout ? paneIds(layout) : []);
  const owners = $derived([...new Set([...ids, ...observed])]);
  $effect(() => retainTerminalDrafts(drafts, owners));
  const documentScroll = $derived.by(() => {
    const firstId = ids[0];
    const current = firstId ? scrollStates[firstId] : undefined;
    if (!current) return false;
    return chatDocumentScroll({
      ...current,
      singlePane: ids.length === 1,
      standalone: isStandalone(),
    });
  });
  const viewport = useViewport(() => !documentScroll);
  const keyboardDismissed = () => hasTouchInput() && viewport.keyboardDismissed;
  onDestroy(resetBrowserChrome);
  $effect(() => {
    const current = info[active];
    if (current)
      setBrowserChrome(
        `${current.title} · WebMux`,
        current.status === "running" && connections[active] === "connected",
      );
  });
  const single = $derived(zoomed || width < 768);
  const geometry = $derived(
    layout
      ? layoutGeometry(layout, width, height)
      : { panes: [], dividers: [] },
  );
  const paneById = $derived(
    new Map(geometry.panes.map((pane) => [pane.sessionId, pane])),
  );
  $effect(() => store(key, layout ? JSON.stringify(layout) : ""));
  $effect(() => store(`${key}.active`, active));
  let forwarding = $state(false);
  const feed = useTerminalPageFeed({
    ids: () => owners,
    source: shell?.sessions,
    forward: () => availablePlugins?.forward,
    error: (message) => (error = message),
  });
  onMount(() => {
    mounted = true;
    if (openTarget) {
      onOpenTargetHandled?.();
      if (!activateSession(openTarget)) reopenSession(openTarget);
    }
    void revealMessage();
    return () => {
      mounted = false;
    };
  });

  async function focus(sessionId: string, keyboard = true) {
    active = sessionId;
    prefix = false;
    await tick();
    if (mounted && keyboard && allowAutomaticInputFocus())
      terminals[sessionId]?.focus();
  }
  // Removes a pane; the last one returns home.
  export function activateSession(sessionId: string) {
    if (!ids.includes(sessionId)) return false;
    void focus(sessionId);
    return true;
  }

  // A reference can name the route anchor after its pane was closed.
  // Reuse the split chooser's add path without remounting surviving panes.
  export function reopenSession(sessionId: string) {
    void openExistingPane(sessionId).catch(
      (cause: unknown) => (error = errorMessage(cause)),
    );
  }
  async function openExistingPane(sessionId: string) {
    if (activateSession(sessionId)) return;
    if (ids.length >= MAX_PANES)
      throw new Error(
        `最多同时打开 ${MAX_PANES} 个窗格；请先关闭一个再打开此会话。`,
      );
    error = "";
    await add(sessionId, "horizontal");
    if (!ids.includes(sessionId))
      throw new Error(error || "无法打开此会话，服务端会话未被修改。");
  }

  async function revealMessage() {
    const query = new URLSearchParams(location.search);
    const target = query.get("messageSession");
    if (!query.get("message") || !target) return;
    if (!ids.includes(target)) {
      if (ids.length >= MAX_PANES) {
        error = `最多同时打开 ${MAX_PANES} 个窗格。请先关闭一个窗格，再打开目标消息。`;
        return;
      }
      await add(target, "horizontal");
      if (!ids.includes(target)) return;
    }
    error = "";
    active = target;
    prefix = false;
    await tick();
    if (mounted) terminals[target]?.revealChat();
  }

  function close(sessionId: string) {
    if (!layout || !ids.includes(sessionId)) return;
    const next = removePane(layout, sessionId);
    const previous = ids.indexOf(sessionId);
    layout = next;
    if (!next) {
      active = "";
      if (!immersive) navigate(shellUi.home);
      return;
    }
    if (active === sessionId)
      void focus(paneIds(next)[Math.max(0, previous - 1)]!);
    if (paneIds(next).length === 1) zoomed = false;
  }
  function removeSession(sessionId: string) {
    pluginScope?.unobserve(sessionId);
    close(sessionId);
    delete info[sessionId];
    delete connections[sessionId];
    delete scrollStates[sessionId];
  }
  $effect(() => shell?.archiveSignal.subscribe(removeSession));
  async function add(
    sessionId: string,
    direction: SplitDirection,
    target = active,
  ) {
    if (
      !mounted ||
      (ids.length > 0 && !ids.includes(target)) ||
      ids.includes(sessionId) ||
      ids.length >= MAX_PANES
    )
      return;
    layout = layout
      ? splitPane(layout, target, sessionId, direction)
      : { kind: "pane", sessionId };
    zoomed = false;
    choosing = false;
    await focus(sessionId);
  }
  async function split(direction: SplitDirection, target: string) {
    if (splitBusy || ids.length >= MAX_PANES) return;
    const source = info[target];
    if (!source) return;
    active = target;
    busy = true;
    error = "";
    try {
      const session = await createSplitShellSession(source);
      if (mounted) {
        info[session.id] = session;
        await add(session.id, direction, target);
      }
    } catch (err) {
      if (mounted) error = `分屏失败：${errorMessage(err)}`;
    } finally {
      if (mounted) busy = false;
    }
  }
  function cycle() {
    void focus(ids[(ids.indexOf(active) + 1) % ids.length]!);
  }
  function zoom(sessionId: string) {
    active = sessionId;
    zoomed = !zoomed;
    void focus(sessionId);
  }
  function keydown(event: KeyboardEvent) {
    if (immersive) return;
    const cmd = parsePrefixKeydown(event, prefix);
    if (cmd.kind === "ignore") return;
    event.preventDefault();
    event.stopImmediatePropagation();
    if (cmd.kind === "enter-prefix") {
      prefix = true;
      return;
    }
    prefix = false;
    if (cmd.kind === "send-prefix") terminals[active]?.send("\x02");
    else if (cmd.kind === "split") void split(cmd.direction, active);
    else if (cmd.kind === "cycle") cycle();
    else if (cmd.kind === "zoom") zoom(active);
    else if (cmd.kind === "close" && !busy) close(active);
    else if (cmd.kind === "arrow") {
      const next = directionalPane(geometry.panes, active, cmd.key);
      if (next) void focus(next);
    }
  }
</script>

<svelte:window onpopstate={revealMessage} />

{#if plugins && availablePlugins}
  {#key plugins}
    <TerminalPluginScope
      {plugins}
      bind:this={pluginScope}
      bind:observed
      bind:immersive
      available={() => feed.sessions}
      ready={() => feed.ready}
      panes={() => ids}
      info={(sessionId) => info[sessionId]}
      exited={() => {
        if (mounted && !layout) navigate(shellUi.home);
      }}
      open={openExistingPane}
      error={(message) => (error = message)}
    >
      <main
        class="terminal-page"
        class:ios={isIOS()}
        data-document-scroll={documentScroll ? "true" : undefined}
        style:--viewport-height={`${viewport.height}px`}
        onkeydowncapture={keydown}
      >
        <TerminalPaneTabs
          {ids}
          {info}
          {active}
          {zoomed}
          {prefix}
          onfocus={(sessionId) => {
            void focus(sessionId);
          }}
          onzoom={() => zoom(active)}
        />
        <TerminalPageNotices
          pluginErrors={plugins.errors}
          {error}
          reloadPage={() => location.reload()}
          dismissError={() => (error = "")}
        />
        <div
          class="split-workspace"
          bind:this={workspace}
          bind:clientWidth={width}
          bind:clientHeight={height}
        >
          {#each owners as sessionId (sessionId)}
            {@const pane = paneById.get(sessionId)}
            {@const visible = !!pane && (!single || active === sessionId)}
            {#key `${sessionId}:${revisions[sessionId] ?? 0}`}
              <TerminalSessionHost
                bind:this={terminals[sessionId]}
                {plugins}
                id={sessionId}
                {pane}
                paneIndex={ids.indexOf(sessionId)}
                full={single || ids.length === 1}
                multiple={ids.length > 1}
                {immersive}
                onactivate={() => {
                  active = sessionId;
                  prefix = false;
                }}
                {drafts}
                {onUploadNotice}
                {visible}
                {keyboardDismissed}
                keyboardVisibility={() => viewport.keyboard}
                viewportHeight={() => viewport.height}
                active={!!pane && active === sessionId}
                reopen={() => {
                  revisions[sessionId] = (revisions[sessionId] ?? 0) + 1;
                }}
                onremoved={() => removeSession(sessionId)}
                onarchived={() => removeSession(sessionId)}
                onleave={pane && ids.length > 1
                  ? () => close(sessionId)
                  : undefined}
                showForwards={() => (forwarding = true)}
                feedSession={feed.byId.get(sessionId)}
                subscribeRemoved={feed.subscribeRemoved}
                documentScroll={!!pane && documentScroll && ids.length === 1}
                onscrollstate={(state) => {
                  scrollStates[sessionId] = state;
                }}
                oninfo={(value, connection) => {
                  info[value.id] = value;
                  connections[value.id] = connection;
                }}
              >
                {#snippet paneActions(inline: boolean)}
                  {#if pane}
                    <TerminalSplitMenu
                      {inline}
                      busy={splitBusy}
                      limit={ids.length >= MAX_PANES}
                      ready={!!info[sessionId]}
                      onsplit={(direction) => {
                        void split(direction, sessionId);
                      }}
                      oncreate={() => createSession(sessionId)}
                      onchoose={() => {
                        active = sessionId;
                        choosing = true;
                      }}
                      onclose={() => close(sessionId)}
                      onzoom={ids.length > 1
                        ? () => zoom(sessionId)
                        : undefined}
                      {zoomed}
                    />
                  {/if}
                {/snippet}
              </TerminalSessionHost>
            {/key}
          {/each}
          {#if !single}
            {#each geometry.dividers as divider (divider.id)}
              <TerminalPaneDivider
                {divider}
                getWorkspaceBounds={() => workspace!.getBoundingClientRect()}
                onresize={(dividerId, ratio) => {
                  if (layout) layout = resizeSplit(layout, dividerId, ratio);
                }}
              />
            {/each}
          {/if}
        </div>
      </main>
      <TerminalPageDialogs
        plugins={availablePlugins}
        bind:choosing
        bind:creating
        bind:forwarding
        excluded={ids}
        feedSessions={feed.sessions}
        onselect={(sessionId, direction) => {
          void add(sessionId, direction);
        }}
        oncreated={(session, direction, targetId) => {
          info[session.id] = session;
          void add(session.id, direction, targetId);
        }}
      />
    </TerminalPluginScope>
  {/key}
{/if}

<style>
  .split-workspace {
    position: relative;
    flex: 1;
    min-inline-size: 0;
    min-block-size: 0;
  }
</style>
