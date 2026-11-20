<script lang="ts">
  import { onDestroy, untrack, type Snippet } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import IconLayoutSidebarLeftExpand from "~icons/tabler/layout-sidebar-left-expand";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import * as Drawer from "$lib/components/ui/drawer";
  import type { BranchOperation } from "../../domains/workspace/branch-actions";
  import { createArchiveSignal } from "../../domains/sessions/archive-signal";
  import {
    resolveLaunchScope,
    type LaunchEntry,
    type LaunchScope,
  } from "../../domains/sessions/new-session-defaults";
  import type { Route } from "../../hooks/shared/route";
  import type {
    ArchivedSession,
    Repository,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
  import {
    openPage,
    openSession,
    setShell,
    shellUi,
    sidebarWidthMax,
    sidebarWidthMin,
    type RepositoryTab,
    type Shell,
  } from "../../hooks/shared/shell.svelte";
  import { dragWidth, keyWidth } from "../../hooks/shared/splitter";
  import { provideSidebarProjectAvatars } from "../../hooks/shared/sidebar-project-avatars";
  import type { TerminalDrafts } from "../../domains/terminal/terminal-draft.svelte";
  import type { RelayReconnection } from "../../platform/relay-auth.svelte";
  import CoreShellDialogs from "./CoreShellDialogs.svelte";
  import { featureContributions } from "../../features/registry.svelte";
  import { createShellSessionResources } from "../../features/shell-resources";
  import AppSidebar from "./AppSidebar.svelte";
  import RelayReconnectIndicator from "./RelayReconnectIndicator.svelte";
  import SidebarFooter from "./SidebarFooter.svelte";
  import SidebarRailSessions from "./SidebarRailSessions.svelte";
  import SessionSwitcherTrigger from "./SessionSwitcherTrigger.svelte";
  import { m } from "$lib/paraglide/messages.js";

  // The layout around the home page, quick start and session pages: the
  // sidebar (a rail when collapsed on desktop, a left drawer on phones) and
  // the page. It owns the data those pages share — one session-list
  // connection, the repositories, archived records, server setup and launch
  // configs — and the dialogs any of them can open. Navigating between its
  // pages keeps this instance, so the sidebar's scroll and folds survive.
  let {
    route,
    drafts,
    reconnection,
    children,
  }: {
    route: Extract<
      Route,
      { name: "sessions" | "quick-start" | "terminal" | "handoff" | "git" }
    >;
    drafts: TerminalDrafts;
    reconnection?: RelayReconnection;
    children: Snippet;
  } = $props();

  // The drafts store lives as long as the app; the shell never swaps it.
  const ownDrafts = untrack(() => drafts);
  const contributions = featureContributions.entries;
  const workspace = contributions.workspace;
  const resources = createShellSessionResources(ownDrafts, () =>
    route.name === "terminal" ? route.id : undefined,
  );
  const {
    sessions,
    setup,
    archived,
    repositories,
    launchConfigs,
    refreshWorktrees,
  } = resources;
  const archiveSignal = createArchiveSignal();
  export function sessionSwitcherSource() {
    return { sessions, repositories, archived };
  }

  let creating = $state<{
    entry?: LaunchEntry;
    scope?: LaunchScope;
  } | null>(null);
  let renaming = $state<SessionInfo | null>(null);
  let closing = $state<SessionInfo | null>(null);
  let removingWorktree = $state<{
    worktree: Worktree;
    onremoved?: () => void;
    forgetRecords?: readonly ArchivedSession[];
  } | null>(null);
  let choosingHome = $state<Repository | null>(null);
  let viewingRepository = $state<{
    repository: Repository;
    tab: RepositoryTab;
  } | null>(null);
  let viewingWorktree = $state<Worktree | null>(null);
  let switchingBranch = $state<{
    worktree: Worktree;
    initialBranch?: string;
    initialMode?: "existing" | "new";
    initialStartPoint?: string;
  } | null>(null);
  let releasingWorktree = $state<Worktree | null>(null);
  let operating = $state<{
    repositoryId: string;
    operation: BranchOperation;
  } | null>(null);
  let forwarding = $state(false);
  let adding = $state(false);
  let configuring = $state<Repository | null>(null);
  // The home view's repository; elsewhere the toolbar has no settings button.
  const settingsRepository = $derived(
    route.name === "sessions" && setup.value.registryReady
      ? (repositories?.list.find((repo) => repo.id === repositories.choice) ??
          ((repositories?.choice ?? "all") === "all" &&
          repositories?.list.length === 1
            ? repositories.list[0]
            : undefined))
      : undefined,
  );

  let activePane = $state("");
  const activeChat = $derived(
    route.name === "terminal"
      ? sessions.sessions.find(
          (session) =>
            session.id === activePane &&
            session.view === "chat" &&
            session.state === "active" &&
            !session.transition,
        )
      : undefined,
  );
  const shell: Shell = {
    drafts: ownDrafts,
    get activeChat() {
      return activeChat;
    },
    setActivePane(id) {
      activePane = id;
    },
    sessions,
    repositories,
    archived,
    archiveSignal,
    setup,
    launchConfigs,
    start(entry) {
      shellUi.drawerOpen = false;
      creating = {
        entry,
        scope: repositories
          ? resolveLaunchScope({
              entry,
              home: route.name === "sessions",
              choice: repositories.choice,
              worktreeId: repositories.worktree,
              repositories: repositories.list,
            })
          : undefined,
      };
      // Pick up configs and worktrees changed elsewhere since the page loaded.
      if (launchConfigs) void launchConfigs.load();
      if (repositories) void repositories.refresh();
    },
    rename(session) {
      shellUi.drawerOpen = false;
      renaming = session;
    },
    close(session) {
      shellUi.drawerOpen = false;
      closing = session;
    },
    removeWorktree(worktree, options) {
      shellUi.drawerOpen = false;
      removingWorktree = { worktree, ...options };
    },
    chooseHomeWorktrees(repository) {
      shellUi.drawerOpen = false;
      choosingHome = repository;
      if (repositories) void repositories.refresh();
    },
    repositoryDetails(repository, tab = "overview") {
      shellUi.drawerOpen = false;
      viewingRepository = { repository, tab };
      // The panel shows the last snapshot while Git is read again.
      if (repositories) void repositories.refresh();
    },
    worktreeDetails(worktree) {
      shellUi.drawerOpen = false;
      viewingWorktree = worktree;
      if (repositories) void repositories.refresh();
    },
    switchBranch(worktree, initial) {
      shellUi.drawerOpen = false;
      if (typeof initial === "string" || initial === undefined) {
        switchingBranch = { worktree, initialBranch: initial };
      } else {
        switchingBranch = {
          worktree,
          initialBranch: initial.branch,
          initialMode: initial.mode,
          initialStartPoint: initial.startPoint,
        };
      }
    },
    releaseWorktree(worktree) {
      shellUi.drawerOpen = false;
      releasingWorktree = worktree;
    },
    branchOperation(repositoryId, operation) {
      shellUi.drawerOpen = false;
      operating = { repositoryId, operation };
    },
    forward() {
      shellUi.drawerOpen = false;
      forwarding = true;
    },
    addRepository() {
      shellUi.drawerOpen = false;
      adding = true;
    },
    get settingsRepository() {
      return settingsRepository;
    },
    openRepositorySettings(repository) {
      shellUi.drawerOpen = false;
      configuring = repository;
    },
  };
  setShell(shell);
  provideSidebarProjectAvatars();
  onDestroy(() => {
    resources.dispose();
    // Leaving the shell (settings) must not reopen the drawer on return.
    shellUi.drawerOpen = false;
  });

  // The new-session dialog belongs to the page it was opened on: a created
  // session navigates away, and the dialog must not follow it there.
  const page = $derived(
    route.name === "terminal"
      ? route.id
      : route.name === "git"
        ? `git:${route.repositoryId}:${route.worktreeId ?? ""}`
        : route.name,
  );
  $effect(() => {
    void page;
    untrack(() => (creating = null));
  });

  // A session started elsewhere names a worktree this page has not read yet.
  // Read the repositories once per unknown worktree: if it is still missing
  // afterwards (for example a released one), "已不可用工作区" is correct and
  // asking again would only repeat the Git discovery.
  const askedFor = new SvelteSet<string>();
  $effect(() => {
    if (!repositories?.settled || !workspace) return;
    const unknown = workspace
      .unknownWorkspaces(sessions.sessions, repositories.list)
      .filter((key) => !askedFor.has(key));
    if (!unknown.length) return;
    for (const key of unknown) askedFor.add(key);
    untrack(() => void repositories.refresh(true));
  });

  let drawerOpener: HTMLElement | null = null;
  let sidebarDrawer = $state<HTMLElement | null>(null);

  // The drawer takes focus when it opens and gives it back to the button
  // that opened it when it closes in place.
  $effect(() => {
    if (!shellUi.drawerOpen) return;
    drawerOpener =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    const timer = setTimeout(() => {
      if (!sidebarDrawer?.contains(document.activeElement))
        sidebarDrawer
          ?.querySelector<HTMLButtonElement>("[data-sidebar-close]")
          ?.focus();
    });
    return () => clearTimeout(timer);
  });

  function restoreDrawerFocus(event: Event) {
    if (shellUi.drawerOpen) return;
    if (drawerOpener?.isConnected) {
      event.preventDefault();
      drawerOpener.focus();
    }
  }

  // Explicit quick-start pages retain a return path for sessions launched there.
  $effect(() => {
    if (route.name === "sessions" && shellUi.mode !== "workbench") {
      shellUi.setMode("workbench");
      void repositories?.refresh();
    } else if (
      (route.name === "quick-start" || route.name === "handoff") &&
      shellUi.mode !== "chat"
    )
      shellUi.setMode("chat");
  });
  // Dragging or arrow keys on the sidebar's edge set its width.
  const setWidth = (width: number) => shellUi.setWidth(width);
  function resizeStart(event: PointerEvent) {
    dragWidth(event, shellUi.width, setWidth);
  }
  function resizeKey(event: KeyboardEvent) {
    keyWidth(
      event,
      shellUi.width,
      { min: sidebarWidthMin, max: sidebarWidthMax },
      setWidth,
    );
  }
</script>

<div
  class="app-shell group/shell"
  data-collapsed={shellUi.collapsed}
  style:--app-sidebar-full-width={`${shellUi.width}px`}
>
  <nav class="app-sidebar app-sidebar-rail" aria-label={m.s1_rail_navigation()}>
    <Button
      variant="ghost"
      size="icon-touch"
      aria-label={m.s1_expand_sidebar()}
      title={m.s1_expand_sidebar()}
      onclick={() => {
        // Tablets keep the rail and show the full sidebar as a drawer.
        if (matchMedia("(max-width: 1023px)").matches)
          shellUi.drawerOpen = true;
        else shellUi.setCollapsed(false);
      }}><IconLayoutSidebarLeftExpand aria-hidden="true" /></Button
    >
    <Button
      variant="ghost"
      size="icon-touch"
      aria-label={m.s1_new_session()}
      title={m.s1_new_session()}
      aria-current={route.name === "quick-start" ? "page" : undefined}
      onclick={() => (contributions.chat ? openPage("/new") : shell.start())}
      ><IconPlus aria-hidden="true" /></Button
    >

    <SessionSwitcherTrigger size="icon-touch" />
    <SidebarRailSessions
      sessions={sessions.sessions.filter(
        (item) => item.state === "active" && item.status !== "exited",
      )}
      current={route.name === "terminal" ? route.id : ""}
      onopen={openSession}
    />
    <div class="mt-auto w-full">
      <SidebarFooter
        rail
        connection={sessions.connection}
        hostname={setup.value.hostname}
        setupSettled={setup.settled}
        {reconnection}
        onaddrepository={workspace && repositories
          ? shell.addRepository
          : undefined}
        canAddRepository={setup.value.gitReady && setup.value.registryReady}
        onforward={shell.forward}
      />
    </div>
  </nav>
  <aside class="app-sidebar app-sidebar-full" aria-label={m.s1_sidebar()}>
    <AppSidebar {route} {reconnection} collapsible />
    <!-- A focusable separator with aria-valuenow is an interactive widget (WAI-ARIA window splitter). -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
    <div
      class="app-sidebar-divider"
      role="separator"
      aria-orientation="vertical"
      aria-label={m.s1_resize_sidebar()}
      title={m.s1_drag_to_resize_sidebar()}
      aria-valuemin={sidebarWidthMin}
      aria-valuemax={sidebarWidthMax}
      aria-valuenow={shellUi.width}
      tabindex="0"
      onpointerdown={resizeStart}
      onkeydown={resizeKey}
    ></div>
  </aside>
  <div class="app-shell-main">
    {@render children()}
  </div>
  {#if reconnection}
    <!-- The content area's lower left, always past the sidebar and its own
         status footer; --app-sidebar-width lives on .app-shell. Where the
         expanded sidebar already shows the reconnecting label, its footer
         carries the details instead of a second indicator. -->
    <RelayReconnectIndicator
      checking={reconnection.checking}
      returnTo={reconnection.returnTo}
      onretry={reconnection.onretry}
      class="left-[calc(var(--app-sidebar-width)+0.5rem)] lg:group-data-[collapsed=false]/shell:hidden"
    />
  {/if}
</div>
{#if contributions.notifications}<contributions.notifications.Toasts />{/if}

<!-- noBodyStyles: the page under it keeps its scroll position and layout. -->
<Drawer.Root direction="left" noBodyStyles bind:open={shellUi.drawerOpen}>
  <Drawer.Content
    bind:ref={sidebarDrawer}
    class="w-80 max-w-[85vw] lg:hidden"
    onOpenAutoFocus={(event) => {
      const close = sidebarDrawer?.querySelector<HTMLButtonElement>(
        "[data-sidebar-close]",
      );
      if (close) {
        event.preventDefault();
        close.focus();
      }
    }}
    onCloseAutoFocus={restoreDrawerFocus}
  >
    <Drawer.Title class="sr-only">{m.s1_navigation()}</Drawer.Title>
    <AppSidebar
      {route}
      {reconnection}
      onclose={() => (shellUi.drawerOpen = false)}
    />
  </Drawer.Content>
</Drawer.Root>

<CoreShellDialogs
  {shell}
  bind:creating
  bind:renaming
  bind:closing
  bind:forwarding
  {repositories}
  {launchConfigs}
  {archived}
  onrefreshworktrees={refreshWorktrees}
/>
{#if workspace}
  <workspace.Dialogs
    {shell}
    bind:removingWorktree
    bind:choosingHome
    bind:viewingRepository
    bind:viewingWorktree
    bind:switchingBranch
    bind:releasingWorktree
    bind:operating
    bind:adding
    bind:configuring
    onrefreshworktrees={refreshWorktrees}
  />
{/if}
