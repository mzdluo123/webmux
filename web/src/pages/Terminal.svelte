<script lang="ts">
  import { findShell } from "../hooks/shared/shell.svelte";
  import { onMount, onDestroy, tick, untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { dockGroup } from "$lib/dock-layout";
  import TerminalPageToolbar from "../components/terminal/TerminalPageToolbar.svelte";
  import { MediaQuery } from "svelte/reactivity";
  import { navigate, store, stored } from "../platform/api";
  import type { SessionInfo } from "../contracts/schemas";
  import { errorMessage } from "../platform/error";
  import { isIOS, hasTouchInput } from "../platform/touch-input";
  import type { ChatScrollState } from "../platform/capabilities";
  import {
    setBrowserChrome,
    resetBrowserChrome,
  } from "../platform/browser-chrome";
  import TerminalSplitPanel from "../components/terminal/TerminalSplitPanel.svelte";
  import DockWorkspace from "../components/shared/DockWorkspace.svelte";
  import TerminalPageDialogs from "../components/terminal/TerminalPageDialogs.svelte";
  import { useViewport } from "../hooks/shared/viewport.svelte";
  import type { TerminalDrafts } from "../domains/terminal/terminal-draft.svelte";
  import SessionTerminal from "../components/terminal/SessionTerminal.svelte";
  import {
    dockSessionLabels,
    ownsSessionPanel,
    panelSession,
    sessionDockTree,
    sessionPanelIds,
  } from "../domains/terminal/session-dock";
  import { migrateSessionLayout } from "../domains/terminal/session-layout-migration";
  import {
    MAX_PANES,
    paneIds,
    removePane,
    restoreLayout,
    splitPane,
    type SplitDirection,
    type PaneLayout,
  } from "../domains/terminal/terminal-layout";
  import {
    ensureExistingPane,
    ensureMessagePane,
    handleTerminalPageKeydown,
    resolveTerminalDocumentScroll,
    runTerminalSplit,
  } from "../domains/terminal/terminal-page-keydown";
  import { useTerminalPageFeed } from "../hooks/terminal/terminal-page-feed.svelte";
  import { retainTerminalDrafts } from "../domains/terminal/terminal-draft-owners";
  import {
    availableTerminalFeatures,
    cachedTerminalFeatures,
    loadTerminalFeatures,
    type TerminalFeatures,
  } from "../features/terminal";
  import { allowAutomaticInputFocus } from "../hooks/shared/input-focus";
  import type { SessionCacheTier } from "../domains/sessions/session-cache";

  let {
    id,
    pageActive = true,
    cacheTier = "active",
    openTarget = "",
    onOpenTargetHandled,
    onJoinSession,
    onEvict,
    drafts,
    onUploadNotice,
  }: {
    id: string;
    pageActive?: boolean;
    cacheTier?: SessionCacheTier;
    openTarget?: string;
    onOpenTargetHandled?: () => void;
    onJoinSession?: (sessionId: string) => void;
    onEvict?: () => void;
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
    if (!pageActive) return;
    shell?.setActivePane(active);
    return () => shell?.setActivePane("");
  });
  let lastInput = $state(untrack(() => active));
  let info = $state<Record<string, SessionInfo>>({});
  let connections = $state<Record<string, string>>({});
  let revisions = $state<Record<string, number>>({});
  let dock = $state<DockWorkspace>();
  let mobilePanel = $state(
    untrack(() => stored(`${key}.mobile-panel`).replace(/^chat:/, "")),
  );
  let showPicker = $state(false);
  const compact = new MediaQuery("(max-width: 639px)");
  let prefix = $state(false);
  let busy = $state(false);
  let error = $state("");
  let features = $state<TerminalFeatures | undefined>(cachedTerminalFeatures());
  const availableFeatures = $derived(
    features ? availableTerminalFeatures(features) : undefined,
  );
  onMount(() => {
    mounted = true;
    let disposed = false;
    if (!features) {
      void loadTerminalFeatures().then(
        (v) => {
          if (!disposed) features = v;
        },
        (c) => {
          if (!disposed) error = errorMessage(c);
        },
      );
    }
    return () => {
      mounted = false;
      disposed = true;
    };
  });
  let choosing = $state(false);
  let joinTarget = "";
  let creating = $state<SessionInfo | null>(null);
  const splitBusy = $derived(busy || creating !== null);
  const desktop = new MediaQuery("(min-width: 1024px)");
  let mounted = false;
  const terminals = $state<Record<string, SessionTerminal | undefined>>({});
  let messagePanel = $state("");
  $effect(() => {
    const target = messagePanel && terminals[messagePanel];
    if (!target || !dock?.hasPanel(messagePanel)) return;
    untrack(() => target.revealChat());
    messagePanel = "";
  });
  let scrollStates = $state<Record<string, ChatScrollState>>({});
  const ids = $derived(layout ? paneIds(layout) : []);
  $effect(() => retainTerminalDrafts(drafts, ids));
  const documentScroll = $derived(
    resolveTerminalDocumentScroll({
      pageActive,
      ids,
      scrollStates,
      viewOf: (sid) => info[sid]?.view,
      isPanelVisible: (panel) => dock?.isVisible(panel) ?? false,
    }),
  );
  const viewport = useViewport(() => !documentScroll);
  const keyboardDismissed = () => hasTouchInput() && viewport.keyboardDismissed;
  onDestroy(() => {
    if (pageActive) resetBrowserChrome();
  });
  $effect(() => {
    if (!pageActive) return;
    const current = info[active];
    if (current)
      setBrowserChrome(
        `${paneTitle(active)} · WebMux`,
        current.status === "running" && connections[active] === "connected",
      );
  });
  const initial = $derived(
    layout
      ? sessionDockTree(
          layout,
          (availableFeatures?.code?.codePanelState.size() ?? 50) / 100,
        )
      : dockGroup(),
  );
  $effect(() => store(key, layout ? JSON.stringify(layout) : ""));
  $effect(() => store(`${key}.active`, active));
  $effect(() => store(`${key}.mobile-panel`, mobilePanel || null));
  let forwarding = $state(false);
  const feed = useTerminalPageFeed({
    ids: () => ids,
    source: shell?.sessions,
    forward: () => availableFeatures?.forward,
    error: (message) => (error = message),
  });
  const paneTitle = (sessionId: string) =>
    feed.byId.get(sessionId)?.title ||
    info[sessionId]?.title ||
    m.s4_tab_terminal();
  const dockSessions = $derived(dockSessionLabels(ids, paneTitle));
  function createSession(target: string) {
    const source = info[target];
    if (splitBusy || ids.length >= MAX_PANES || !source) return;
    active = target;
    creating = source;
  }
  $effect(() => {
    if (!pageActive) {
      prefix = false;
      return;
    }
    const target = openTarget;
    untrack(() => {
      if (target) {
        onOpenTargetHandled?.();
        if (!activateSession(target)) reopenSession(target);
      }
      void revealMessage();
    });
  });

  async function focus(sessionId: string, keyboard = true) {
    active = sessionId;
    prefix = false;
    await tick();
    terminals[sessionId]?.showInputPanel();
    if (mounted && keyboard && allowAutomaticInputFocus())
      terminals[sessionId]?.focus();
  }
  // Removes a pane; the last one returns home.
  export function activateSession(sessionId: string) {
    if (!ids.includes(sessionId)) return false;
    void focus(sessionId);
    return true;
  }
  export function reopenSession(sessionId: string) {
    void ensureExistingPane({
      sessionId,
      activate: activateSession,
      getIds: () => ids,
      add: (sid, dir) => add(sid, dir),
      getError: () => error,
      clearError: () => (error = ""),
    }).catch((cause: unknown) => (error = errorMessage(cause)));
  }
  async function revealMessage() {
    await ensureMessagePane({
      search: location.search,
      getIds: () => ids,
      add: (sid, dir) => add(sid, dir),
      onLimitError: (msg) => (error = msg),
      onRevealed: (target) => {
        error = "";
        active = target;
        prefix = false;
        messagePanel = target;
      },
    });
  }
  function close(sessionId: string) {
    if (!layout || !ids.includes(sessionId)) return;
    const next = removePane(layout, sessionId);
    const previous = ids.indexOf(sessionId);
    for (const panel of dock?.ids() ?? [])
      if (ownsSessionPanel(sessionId, panel)) dock?.remove(panel);
    layout = next;
    if (!next) {
      active = "";
      onEvict?.();
      if (pageActive) navigate("/");
      return;
    }
    if (active === sessionId)
      void focus(paneIds(next)[Math.max(0, previous - 1)]!);
  }
  function removeSession(sessionId: string) {
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
    onJoinSession?.(sessionId);
    const panels = sessionPanelIds(target);
    const chat = info[target]?.view === "chat";
    dock?.add(
      sessionId,
      chat ? panels.reader : panels.terminal,
      chat || direction === "vertical" ? "bottom" : "right",
      true,
    );
    layout = layout
      ? splitPane(layout, target, sessionId, direction)
      : { kind: "pane", sessionId };
    await focus(sessionId);
  }
  function split(direction: SplitDirection, target: string) {
    const source = info[target];
    if (
      splitBusy ||
      ids.length >= MAX_PANES ||
      !ids.includes(target) ||
      !source
    )
      return;
    void runTerminalSplit(source, direction, target, {
      isMounted: () => mounted,
      onStart: () => {
        active = target;
        busy = true;
        error = "";
      },
      onCreated: async (s, d, t) => {
        info[s.id] = s;
        await add(s.id, d, t);
      },
      onError: (msg) => (error = msg),
      onFinally: () => (busy = false),
    });
  }
  function keydown(event: KeyboardEvent) {
    if (!pageActive) return;
    handleTerminalPageKeydown(event, {
      getPrefix: () => prefix,
      setPrefix: (value) => (prefix = value),
      getActive: () => active,
      getIds: () => ids,
      isBusy: () => busy,
      sendPrefix: (id) => terminals[id]?.send("\x02"),
      split: (dir, id) => void split(dir, id),
      focus: (id) => void focus(id),
      toggleZoom: (id) => dock?.toggleZoom(id),
      close,
      paneRect: (id) => dock?.rect(id),
    });
  }
</script>

<svelte:window
  onpopstate={() => {
    if (pageActive) void revealMessage();
  }}
/>

{#if features && availableFeatures}
  {#key features}
    <div
      class="terminal-page"
      class:ios={isIOS()}
      role={pageActive ? "main" : undefined}
      data-document-scroll={documentScroll ? "true" : undefined}
      data-cache-hidden={pageActive ? undefined : "true"}
      inert={!pageActive}
      aria-hidden={!pageActive}
      style:--viewport-height={`${viewport.height}px`}
      onkeydowncapture={keydown}
    >
      {#if pageActive}
        <TerminalPageToolbar
          {dock}
          closePane={(panel) =>
            ids.includes(panel) ? close(panel) : dock?.minimize(panel)}
          pickerOpen={showPicker}
          openPanels={() => dock?.openPicker()}
          fileLabel={`${m.s5_file_tree()} · ${paneTitle(active)}`}
          openFiles={availableFeatures.code
            ? () => terminals[active]?.openCode()
            : undefined}
          {prefix}
          featureErrors={features.errors}
          {error}
          onerror={(message) => (error = message)}
          dismissError={() => (error = "")}
        />
      {/if}
      <DockWorkspace
        bind:this={dock}
        storageKey={`session.v2.${id}`}
        migrate={() =>
          migrateSessionLayout(
            stored(`dock.session.${id}`),
            ids,
            (sid) => feed.byId.get(sid)?.view === "chat",
          )}
        {initial}
        flow={documentScroll}
        compact={compact.current}
        bind:mobilePanel
        bind:showPicker
        edgeRails
        pickerTrigger={false}
        onSelectPanel={(panel) => {
          const s = panelSession(ids, panel);
          if (s) active = s;
          if (s === panel) lastInput = panel;
        }}
      >
        {#snippet pickerActions(closePanel, panel, hide)}
          {@const target = panel ? (panelSession(ids, panel.id) ?? "") : active}
          <TerminalSplitPanel
            compact={compact.current}
            busy={splitBusy}
            limit={ids.length >= MAX_PANES}
            ready={!!info[target]}
            oncommand={closePanel}
            onsplit={(direction) => void split(direction, target)}
            oncreate={() => createSession(target)}
            onchoose={ids.includes(target)
              ? () => ((joinTarget = target), (choosing = true))
              : undefined}
            onclose={target && (!compact.current || panel?.id === target)
              ? () => close(target)
              : undefined}
            hideLabel={panel && m.dock_minimize({ title: panel.title })}
            onhide={hide}
          />
        {/snippet}
        {#each ids as sessionId (sessionId)}
          {@const panels = sessionPanelIds(sessionId)}
          {@const dockSession = dockSessions.get(sessionId)}
          {#key `${sessionId}:${revisions[sessionId] ?? 0}`}
            <SessionTerminal
              bind:this={terminals[sessionId]}
              {features}
              id={sessionId}
              {panels}
              referenceTarget={terminals[lastInput]?.referenceInput}
              dockSessionName={dockSession?.name}
              dockSessionNumber={dockSession?.number}
              desktop={desktop.current}
              onactivate={() => ((active = sessionId), (prefix = false))}
              {drafts}
              {onUploadNotice}
              visible={pageActive &&
                (dock?.isAnyVisible((panel) =>
                  ownsSessionPanel(sessionId, panel),
                ) ??
                  false)}
              {keyboardDismissed}
              keyboardVisibility={() => viewport.keyboard}
              viewportHeight={() => viewport.height}
              active={pageActive && active === sessionId}
              {cacheTier}
              reopen={(session) => {
                info[sessionId] = session;
                dock?.show(panels.terminal);
                revisions[sessionId] = (revisions[sessionId] ?? 0) + 1;
              }}
              onremoved={() => removeSession(sessionId)}
              onarchived={() => removeSession(sessionId)}
              onleave={ids.length > 1 ? () => close(sessionId) : undefined}
              showForwards={() => (forwarding = true)}
              feedSession={feed.byId.get(sessionId)}
              subscribeRemoved={feed.subscribeRemoved}
              {documentScroll}
              onscrollstate={(state) => (scrollStates[sessionId] = state)}
              oninfo={(value, conn) => {
                info[value.id] = value;
                connections[value.id] = conn;
              }}
            />
          {/key}
        {/each}
      </DockWorkspace>
    </div>
    {#if pageActive}
      <TerminalPageDialogs
        features={availableFeatures}
        bind:choosing
        bind:creating
        bind:forwarding
        excluded={ids}
        feedSessions={feed.sessions}
        oncancelchoose={() => void focus(active)}
        onselect={(sessionId, direction) => {
          choosing = false;
          void add(sessionId, direction, joinTarget);
        }}
        oncreated={(session, direction, targetId) => {
          info[session.id] = session;
          void add(session.id, direction, targetId);
        }}
      />
    {/if}
  {/key}
{/if}
