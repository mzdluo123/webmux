<script lang="ts">
  import { findShell, shellUi } from "../hooks/shared/shell.svelte";
  import { onMount, onDestroy, tick, untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { dockGroup } from "$lib/dock-layout";
  import SessionWorkspaceHeader from "../components/terminal/SessionWorkspaceHeader.svelte";
  import { MediaQuery } from "svelte/reactivity";
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
  import TerminalSplitPanel from "../components/terminal/TerminalSplitPanel.svelte";
  import DockWorkspace from "../components/shared/DockWorkspace.svelte";
  import TerminalPageDialogs from "../components/terminal/TerminalPageDialogs.svelte";
  import TerminalPageNotices from "../components/terminal/TerminalPageNotices.svelte";
  import { useViewport } from "../hooks/shared/viewport.svelte";
  import type { TerminalDrafts } from "../domains/terminal/terminal-draft.svelte";
  import SessionTerminal from "../components/terminal/SessionTerminal.svelte";
  import {
    dockSessionLabels,
    sessionDockTree,
    sessionPanelIds,
  } from "../domains/terminal/session-dock";
  import { migrateSessionLayout } from "../domains/terminal/session-layout-migration";
  import {
    MAX_PANES,
    createSplitShellSession,
    directionalPane,
    paneIds,
    parsePrefixKeydown,
    removePane,
    restoreLayout,
    splitPane,
    type SplitDirection,
    type PaneLayout,
  } from "../domains/terminal/terminal-layout";
  import { useTerminalPageFeed } from "../hooks/terminal/terminal-page-feed.svelte";
  import { retainTerminalDrafts } from "../domains/terminal/terminal-draft-owners";
  import {
    loadTerminalFeatures,
    availableTerminalFeatures,
    type TerminalFeatures,
  } from "../features/terminal";
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
  let dock = $state<DockWorkspace>();
  let mobilePanel = $state(
    untrack(() => stored(`${key}.mobile-panel`).replace(/^chat:/, "")),
  );
  let showPicker = $state(false);
  const compact = new MediaQuery("(max-width: 639px)");
  let prefix = $state(false);
  let busy = $state(false);
  let error = $state("");
  let features = $state<TerminalFeatures>();
  const availableFeatures = $derived(
    features ? availableTerminalFeatures(features) : undefined,
  );
  onMount(() => {
    let disposed = false;
    void loadTerminalFeatures()
      .then((value) => {
        if (!disposed) features = value;
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
  const documentScroll = $derived.by(() => {
    const firstId = ids[0];
    const current = firstId ? scrollStates[firstId] : undefined;
    if (!firstId || !current || ids.length !== 1 || !dock) return false;
    const panels = sessionPanelIds(firstId);
    if (!dock.isVisible(panels.terminal) || info[firstId]?.view !== "chat")
      return false;
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
        `${paneTitle(active)} · WebMux`,
        current.status === "running" && connections[active] === "connected",
      );
  });
  const initial = $derived(
    layout
      ? sessionDockTree(
          layout,
          (sid) => (info[sid] ?? feed.byId.get(sid))?.view === "chat",
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
      throw new Error(m.s1_max_panes_reopen({ max: String(MAX_PANES) }));
    error = "";
    await add(sessionId, "horizontal");
    if (!ids.includes(sessionId))
      throw new Error(error || m.s1_open_session_failed_unmodified());
  }

  async function revealMessage() {
    const query = new URLSearchParams(location.search);
    const target = query.get("messageSession");
    if (!query.get("message") || !target) return;
    if (!ids.includes(target)) {
      if (ids.length >= MAX_PANES) {
        error = m.s1_max_panes_reveal_message({ max: String(MAX_PANES) });
        return;
      }
      await add(target, "horizontal");
      if (!ids.includes(target)) return;
    }
    error = "";
    active = target;
    prefix = false;
    messagePanel = target;
  }

  function close(sessionId: string) {
    if (!layout || !ids.includes(sessionId)) return;
    const next = removePane(layout, sessionId);
    const previous = ids.indexOf(sessionId);
    for (const panel of Object.values(sessionPanelIds(sessionId)))
      dock?.remove(panel);
    layout = next;
    if (!next) {
      active = "";
      navigate(shellUi.home);
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
    const panels = sessionPanelIds(target);
    const chat = info[target]?.view === "chat";
    const dockTarget = chat ? panels.reader : panels.terminal;
    dock?.add(
      sessionId,
      dockTarget,
      chat || direction === "vertical" ? "bottom" : "right",
      true,
    );
    layout = layout
      ? splitPane(layout, target, sessionId, direction)
      : { kind: "pane", sessionId };
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
      if (mounted) error = m.s1_split_pane_failed({ error: errorMessage(err) });
    } finally {
      if (mounted) busy = false;
    }
  }
  function cycle() {
    void focus(ids[(ids.indexOf(active) + 1) % ids.length]!);
  }
  function zoom(sessionId: string) {
    dock?.toggleZoom(sessionId);
  }
  function keydown(event: KeyboardEvent) {
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
      const panes = ids.flatMap((sessionId) => {
        const rect = dock?.rect(sessionId);
        return rect ? [{ ...rect, sessionId }] : [];
      });
      const next = directionalPane(panes, active, cmd.key);
      if (next) void focus(next);
    }
  }
</script>

<svelte:window onpopstate={revealMessage} />

{#if features && availableFeatures}
  {#key features}
    <main
      class="terminal-page"
      class:ios={isIOS()}
      data-document-scroll={documentScroll ? "true" : undefined}
      style:--viewport-height={`${viewport.height}px`}
      onkeydowncapture={keydown}
    >
      <div class="session-toolbar shrink-0">
        <SessionWorkspaceHeader
          pickerOpen={showPicker}
          openPanels={() => dock?.openPicker()}
          fileLabel={`${m.s5_file_tree()} · ${paneTitle(active)}`}
          openFiles={availableFeatures.code
            ? () => terminals[active]?.openCode()
            : undefined}
          onerror={(message) => (error = message)}
        />
      </div>
      {#if prefix}
        <p
          class="shrink-0 border-b px-2 py-1 text-xs text-muted-foreground"
          role="status"
        >
          {m.s4_prefix_shortcut_bar()}
        </p>
      {/if}
      <TerminalPageNotices
        featureErrors={features.errors}
        {error}
        reloadPage={() => location.reload()}
        dismissError={() => (error = "")}
      />
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
        pickerTrigger={false}
        onSelectPanel={(panel) => {
          const sessionId = ids.find(
            (sid) =>
              Object.values(sessionPanelIds(sid)).includes(panel) ||
              panel.startsWith(`reader:${sid}:`),
          );
          if (sessionId) active = sessionId;
        }}
      >
        {#snippet pickerActions(closePanel)}
          <TerminalSplitPanel
            busy={splitBusy}
            limit={ids.length >= MAX_PANES}
            ready={!!info[active]}
            onsplit={(direction) => {
              closePanel();
              void split(direction, active);
            }}
            oncreate={() => {
              closePanel();
              createSession(active);
            }}
            onchoose={() => {
              closePanel();
              choosing = true;
            }}
            onclose={() => {
              closePanel();
              close(active);
            }}
          />
        {/snippet}
        {#each ids as sessionId (sessionId)}
          {@const panels = sessionPanelIds(sessionId)}
          {@const dockSession = dockSessions.get(sessionId)}
          {@const visible =
            !!dock &&
            Object.values(panels).some((panel) => dock?.isVisible(panel))}
          {#key `${sessionId}:${revisions[sessionId] ?? 0}`}
            <SessionTerminal
              bind:this={terminals[sessionId]}
              {features}
              id={sessionId}
              {panels}
              dockSessionName={dockSession?.name}
              dockSessionNumber={dockSession?.number}
              desktop={desktop.current}
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
              active={active === sessionId}
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
              onscrollstate={(state) => {
                scrollStates[sessionId] = state;
              }}
              oninfo={(value, connection) => {
                info[value.id] = value;
                connections[value.id] = connection;
              }}
            />
          {/key}
        {/each}
      </DockWorkspace>
    </main>
    <TerminalPageDialogs
      features={availableFeatures}
      bind:choosing
      bind:creating
      bind:forwarding
      excluded={ids}
      feedSessions={feed.sessions}
      oncancelchoose={() => void focus(active)}
      onselect={(sessionId, direction) => {
        void add(sessionId, direction);
      }}
      oncreated={(session, direction, targetId) => {
        info[session.id] = session;
        void add(session.id, direction, targetId);
      }}
    />
  {/key}
{/if}

<style>
  .terminal-page[data-document-scroll="true"] .session-toolbar {
    position: sticky;
    top: 0;
    z-index: 20;
  }
</style>
