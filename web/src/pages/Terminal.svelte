<script lang="ts">
  import { findShell, shellUi } from "../hooks/shared/shell.svelte";
  import { onMount, onDestroy, tick, untrack } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
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
  import InlineNotice from "../components/shared/InlineNotice.svelte";
  import { useViewport } from "../hooks/shared/viewport.svelte";
  import type { TerminalDrafts } from "../domains/terminal/terminal-draft.svelte";
  import { Button } from "$lib/components/ui/button";
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
  import { connectSessions } from "../domains/workspace/forwards.svelte";
  import { applySessionEvent } from "../hooks/sessions/session-list";
  import { createXrSessionWorkspace } from "../hooks/terminal/xr-session.svelte";
  import { createXrSessionMembership } from "../hooks/terminal/xr-session-membership.svelte";
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
  const membership = createXrSessionMembership({
    live: () => shell?.sessions.sessions ?? feedSessions,
    ready: () => shell?.sessions.ready ?? feedReady,
    panes: () => ids,
    info: (sessionId) => info[sessionId],
  });
  const xr = createXrSessionWorkspace({
    available: () => membership.available,
    ready: () => shell?.sessions.ready ?? feedReady,
    observe: async (sessionId) => {
      if (!xr.active) throw new Error("VR 工作台已退出。");
      await membership.observe(sessionId);
    },
    unobserve: (sessionId) => membership.unobserve(sessionId),
    reveal: openExistingPane,
    exited: () => {
      membership.clear();
      if (mounted && !layout) navigate(shellUi.home);
    },
    error: (message) => (error = message),
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
  let workspace: HTMLDivElement;
  let mounted = false;
  const terminals = $state<Record<string, TerminalSessionHost | undefined>>({});
  let scrollStates = $state<Record<string, ChatScrollState>>({});
  const ids = $derived(layout ? paneIds(layout) : []);
  const owners = $derived([...new Set([...ids, ...membership.observed])]);
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
  // Every session, for the forwarding dialog's detected ports.
  let feedSessions = $state.raw<SessionInfo[]>([]);
  let feedReady = $state(false);
  const removedListeners = new SvelteSet<(id: string) => void>();
  function subscribeRemoved(listener: (id: string) => void) {
    removedListeners.add(listener);
    return () => {
      removedListeners.delete(listener);
    };
  }
  // The shell already owns a feed. Standalone pages keep one while any
  // owner exists, including recovery before a chat stream ever attaches,
  // so the archive commit's final removed event cannot fall into a gap.
  const standaloneFeed = $derived(!shell && owners.length > 0);
  const feedById = $derived(
    new Map(
      (shell?.sessions.sessions ?? feedSessions).map((session) => [
        session.id,
        session,
      ]),
    ),
  );
  $effect(() => {
    if (!standaloneFeed) return;
    const socket = connectSessions(
      (event) => {
        if (event.type === "snapshot") feedReady = true;
        if (event.type === "removed")
          for (const listener of removedListeners) listener(event.id);
        feedSessions = applySessionEvent(feedSessions, event);
      },
      () => {},
      (message) => {
        error = message;
      },
    );
    return () => socket.close();
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
      if (!xr.active) navigate(shellUi.home);
      return;
    }
    if (active === sessionId)
      void focus(paneIds(next)[Math.max(0, previous - 1)]!);
    if (paneIds(next).length === 1) zoomed = false;
  }
  function removeSession(sessionId: string) {
    membership.unobserve(sessionId);
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
    if (xr.active) return;
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
  {#if error}<InlineNotice tone="danger"
      ><div class="flex items-center justify-between gap-2">
        <span>{error}</span><Button
          variant="ghost"
          size="sm"
          onclick={() => (error = "")}>关闭</Button
        >
      </div></InlineNotice
    >{/if}
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
          id={sessionId}
          {pane}
          paneIndex={ids.indexOf(sessionId)}
          full={single || ids.length === 1}
          multiple={ids.length > 1}
          immersive={xr.active}
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
          onleave={pane && ids.length > 1 ? () => close(sessionId) : undefined}
          showForwards={() => (forwarding = true)}
          feedSession={feedById.get(sessionId)}
          subscribeRemoved={shell?.sessions.subscribeRemoved ??
            subscribeRemoved}
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
                onzoom={ids.length > 1 ? () => zoom(sessionId) : undefined}
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
          getWorkspaceBounds={() => workspace.getBoundingClientRect()}
          onresize={(dividerId, ratio) => {
            if (layout) layout = resizeSplit(layout, dividerId, ratio);
          }}
        />
      {/each}
    {/if}
  </div>
</main>
<TerminalPageDialogs
  bind:choosing
  bind:creating
  bind:forwarding
  excluded={ids}
  {feedSessions}
  onselect={(sessionId, direction) => {
    void add(sessionId, direction);
  }}
  oncreated={(session, direction, targetId) => {
    info[session.id] = session;
    void add(session.id, direction, targetId);
  }}
/>

<style>
  .split-workspace {
    position: relative;
    flex: 1;
    min-inline-size: 0;
    min-block-size: 0;
  }
</style>
