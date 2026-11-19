<script lang="ts">
  import IconPlus from "~icons/tabler/plus";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { errorMessage } from "../../platform/error";
  import { sessionPins } from "../../domains/sessions/session-pins.svelte";
  import type { Route } from "../../hooks/shared/route";
  import type { RelayReconnection } from "../../platform/relay-auth.svelte";
  import type {
    ArchivedSession,
    HybridRelease,
    Worktree,
    SessionInfo,
    AgentConnector,
  } from "../../contracts/schemas";
  import {
    getShell,
    openPage,
    openSession,
    shellUi,
  } from "../../hooks/shared/shell.svelte";
  import { resumesOnOpen } from "../../domains/sessions/archived";
  import { workbenchLayout } from "../../domains/workspace/workbench-layout.svelte";
  import { useLayoutRowsMotion } from "../../hooks/shared/layout-rows-motion.svelte";
  import { useSidebarScrollHandoff } from "../../hooks/shared/sidebar-scroll-handoff.svelte";
  import { paneEnter } from "../../hooks/shared/pane-enter";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import SidebarWorkspaceTree from "./SidebarWorkspaceTree.svelte";
  import WorkbenchSidebarSessions from "./WorkbenchSidebarSessions.svelte";
  import SidebarSessionRow from "./SidebarSessionRow.svelte";
  import SidebarPinnedSessions from "./SidebarPinnedSessions.svelte";
  import SidebarFooter from "./SidebarFooter.svelte";
  import SidebarHeader from "./SidebarHeader.svelte";
  import { featureContributions } from "../../features/registry.svelte";
  import LazyDialog from "../shared/LazyDialog.svelte";
  import { m } from "$lib/paraglide/messages.js";
  const workspace = featureContributions.entries.workspace;
  const chat = featureContributions.entries.chat;
  const notifications = featureContributions.entries.notifications;
  const recordTitle = (record: ArchivedSession) =>
    chat?.recordTitle(record) ?? record.title;
  const connectorMissingText = (connector: AgentConnector) =>
    chat?.connectorMissingText(connector) ?? "";
  const removeRecord = (id: string) => chat!.removeRecord(id);
  const resumeSession = (id: string, view: SessionInfo["view"]) =>
    chat!.resumeSession(id, view);

  // The sidebar body, the same on desktop and in the phone drawer: brand and
  // mode switch, the mode's primary action, its navigation, then the global
  // footer that replaced the home page header.
  let {
    route,
    collapsible = false,
    onclose,
    reconnection,
  }: {
    route: Extract<
      Route,
      { name: "sessions" | "quick-start" | "terminal" | "handoff" | "git" }
    >;
    /** Desktop only; the drawer closes on its own. */
    collapsible?: boolean;
    onclose?: () => void;
    reconnection?: RelayReconnection;
  } = $props();
  const shell = getShell();
  const current = $derived(route.name === "terminal" ? route.id : "");
  const live = $derived(
    shell.sessions.sessions.filter((session) => session.state === "active"),
  );
  const queue = $derived(
    Boolean(workspace && workbenchLayout.value === "queue"),
  );
  const queueLoading = $derived(
    !shell.sessions.ready ||
      shell.repositories?.initialSettled === false ||
      shell.archived?.initialSettled === false,
  );
  const chatHome = $derived(
    !!chat && (route.name === "quick-start" || route.name === "handoff"),
  );
  const navigationId = $props.id();
  let navigationRows = $state<HTMLDivElement | null>(null);
  useLayoutRowsMotion(
    () => navigationRows,
    () => (chatHome ? "" : workbenchLayout.value),
    "[data-sidebar-session-id], [data-layout-row]",
  );
  const showWorkspaces = $derived(
    Boolean(shell.repositories?.settled || shell.repositories?.list.length),
  );
  const pinned = $derived(sessionPins.ordered(live));
  const records = $derived(
    shell.archived?.records.filter((record) => !pinnedIDs.has(record.id)) ?? [],
  );
  const pinnedIDs = $derived(new Set(pinned.map((session) => session.id)));
  const unpinned = $derived(
    live.filter((session) => !pinnedIDs.has(session.id)),
  );
  const configs = $derived(
    featureContributions.entries.launch &&
      shell.launchConfigs?.state.status === "ready"
      ? shell.launchConfigs.state.configs
      : [],
  );
  const configIcons = $derived(
    new Map(configs.map((config) => [config.id, config.icon ?? ""])),
  );
  const setup = $derived(shell.setup.value);

  let resuming = $state("");
  // The last failed sidebar action (resume or record deletion), shown above
  // the list.
  let resumeErrorFn = $state<(() => string) | null>(null);
  const resumeError = $derived(resumeErrorFn?.() ?? "");
  let removing = $state<ArchivedSession | null>(null);
  let removeOpen = $state(false);
  function askRemove(record: ArchivedSession) {
    removing = record;
    removeOpen = true;
  }
  function askRemoveWithWorktree(record: ArchivedSession, worktree: Worktree) {
    shell.removeWorktree(worktree, {
      onremoved: () => void forgetAfterWorktree(record),
      forgetRecords: [record],
    });
  }
  // The worktree is already gone when this runs; a failed record deletion
  // leaves a released record behind, which the user can delete again.
  async function forgetAfterWorktree(record: ArchivedSession) {
    resumeErrorFn = null;
    try {
      await removeRecord(record.id);
      sessionPins.refreshEnded(record.id);
      shell.drafts.remove(record.id);
    } catch (err) {
      const cause = errorMessage(err);
      resumeErrorFn = () =>
        m.s1_worktree_removed_record_delete_failed({
          title: recordTitle(record),
          error: cause,
        });
    } finally {
      await shell.archived?.refresh();
    }
  }
  async function confirmRemove() {
    const record = removing;
    removeOpen = false;
    if (!record) return;
    resumeErrorFn = null;
    try {
      await removeRecord(record.id);
      sessionPins.refreshEnded(record.id);
      shell.drafts.remove(record.id);
      await shell.archived?.refresh();
    } catch (err) {
      const cause = errorMessage(err);
      resumeErrorFn = () =>
        m.s1_record_delete_failed({
          title: recordTitle(record),
          error: cause,
        });
    }
  }
  // Resume on the recorded view, independent of the sidebar's mode. Choices
  // that need the user are made on the record's own page. A released record's
  // worktree is rebuilt first, once the user confirms how.
  let rebuilding = $state<{
    record: ArchivedSession;
    released: HybridRelease;
  } | null>(null);
  function open(record: ArchivedSession) {
    if (resumesOnOpen(record)) {
      openSession(record);
      return;
    }
    if (record.released && !workspace)
      resumeErrorFn = () => m.s1_archive_requires_workspace_feature();
    else if (record.released)
      rebuilding = { record, released: record.released };
    else void resume(record);
  }
  function rebuilt() {
    const target = rebuilding;
    rebuilding = null;
    if (!target) return;
    // The worktree is back and the release cleared, whatever the resume does.
    void shell.repositories?.refresh();
    void shell.archived?.refresh();
    void resume(target.record);
  }
  async function resume(record: ArchivedSession) {
    resuming = record.id;
    resumeErrorFn = null;
    try {
      const outcome = await resumeSession(record.id, record.view);
      if (outcome.status === "resumed") {
        void shell.archived?.refresh();
        openSession(outcome.session);
      } else if (outcome.status === "connector_missing") {
        resumeErrorFn = () =>
          m.s1_connector_missing_combined({
            message: outcome.message,
            detail: connectorMissingText(outcome.connector),
          });
      } else openPage(`/s/${record.id}`);
    } catch (err) {
      const cause = errorMessage(err);
      resumeErrorFn = () =>
        m.s1_resume_session_failed({ title: record.title, error: cause });
    } finally {
      resuming = "";
    }
  }
  function revealNavigation(node: HTMLElement, _chatHome: boolean) {
    const animation = paneEnter(node, false);
    return {
      update() {
        animation.update(false);
        animation.update(true);
      },
      destroy: animation.destroy,
    };
  }
  let drawer = $state<HTMLElement | null>(null);
  let navigationViewport = $state<HTMLElement | null>(null);

  useSidebarScrollHandoff(
    () => (onclose ? drawer : null),
    () => navigationViewport,
  );

  function filterQueue(attention: boolean) {
    shell.repositories?.select("all");
    workbenchLayout.attentionOnly = attention;
    if (route.name !== "sessions") openPage("/");
  }
  function scope(next: string, worktreeId?: string) {
    workbenchLayout.attentionOnly = false;
    shellUi.setMode("workbench");
    shell.repositories?.select(next, worktreeId);
    openPage("/");
  }
</script>

{#snippet history()}
  {#if chat}<chat.History
      sessions={unpinned}
      {records}
      {configs}
      repositories={shell.repositories?.list ?? []}
      {current}
      {resuming}
      onopen={openSession}
      onresume={open}
      onrename={shell.rename}
      onclose={shell.close}
      onrelease={shell.releaseWorktree}
      onremove={askRemove}
      onremoveworktree={askRemoveWithWorktree}
    />{/if}
{/snippet}

{#snippet workspaces()}
  <SidebarWorkspaceTree
    sessions={unpinned}
    {records}
    showScope={route.name === "sessions"}
    {current}
    {configIcons}
    onscope={scope}
    {resuming}
    onresume={open}
    onremove={askRemove}
  />
{/snippet}

<nav
  bind:this={drawer}
  class="flex h-full min-h-0 flex-col"
  class:sidebar-drawer={Boolean(onclose)}
  aria-label={m.s1_navigation()}
>
  <SidebarHeader
    oncollapse={collapsible ? () => shellUi.setCollapsed(true) : undefined}
    {onclose}
    showSessionSwitcher={collapsible &&
      route.name === "sessions" &&
      !!workspace}
    controls={navigationId}
    byBranch={!queue}
    onGroupingChange={workspace
      ? (checked) => {
          workbenchLayout.value = checked ? "workspaces" : "queue";
          if (chatHome) openPage("/");
        }
      : undefined}
  />
  <div class="sidebar-actions flex flex-col gap-1.5 pt-1">
    <div class="flex items-center gap-1.5">
      <Button
        variant="outline"
        size="sm-touch"
        class="min-w-0 flex-1"
        aria-current={route.name === "quick-start" ? "page" : undefined}
        onclick={() => (chat ? openPage("/new") : shell.start())}
      >
        <IconPlus aria-hidden="true" />{m.s1_new_session()}
      </Button>
      {#if !chatHome && workspace && shell.repositories}
        <Button
          variant="outline"
          size="icon-touch"
          aria-label={m.s1_refresh_workspaces()}
          title={shell.repositories.refreshing
            ? m.s1_refreshing_workspaces()
            : m.s1_refresh_workspaces()}
          aria-busy={shell.repositories.refreshing}
          onclick={() => void shell.repositories?.refresh(true)}
        >
          <IconRefresh
            class={shell.repositories.refreshing
              ? "animate-spin motion-reduce:animate-none"
              : ""}
            aria-hidden="true"
          />
        </Button>
      {/if}
    </div>
  </div>
  {#if !workspace}
    <div class="px-2 pt-2">
      <Button
        variant="ghost"
        size="sm-touch"
        class="w-full"
        aria-current={route.name === "sessions" ? "page" : undefined}
        onclick={() => openPage("/")}>{m.s1_sessions()}</Button
      >
    </div>
  {/if}
  {#if notifications}<notifications.Inbox
      sessions={live}
      triggerLabel={workspace ? m.s8_queue_notifications() : undefined}
    />{/if}
  <ScrollArea
    bind:viewportRef={navigationViewport}
    type="hover"
    class="min-h-0 flex-1 [&_[data-slot=scroll-area-viewport]]:overscroll-contain [&_[data-slot=scroll-area-viewport]>div]:!block"
  >
    <div id={navigationId} class="px-2 py-2" bind:this={navigationRows}>
      {#if pinned.length && (chatHome || !queue)}
        <SidebarPinnedSessions
          sessions={pinned}
          {current}
          {configIcons}
          showLocation={chatHome}
        />
      {/if}

      {#if resumeError || sessionPins.error}<p
          role="alert"
          class="mb-2 rounded-md border border-destructive/40 px-3 py-2 text-xs text-destructive"
        >
          {resumeError || sessionPins.error}
        </p>{/if}
      <div use:revealNavigation={chatHome}>
        {#if chatHome}
          {@render history()}
        {:else if queue}
          <WorkbenchSidebarSessions
            loading={queueLoading}
            sessions={live}
            {records}
            {pinned}
            repositories={shell.repositories?.list ?? []}
            {configIcons}
            {current}
            allCurrent={route.name === "sessions" &&
              !workbenchLayout.attentionOnly &&
              (!shell.repositories?.choice ||
                shell.repositories.choice === "all")}
            attentionCurrent={route.name === "sessions" &&
              workbenchLayout.attentionOnly}
            onfilter={filterQueue}
            oncreate={(repositoryId) =>
              shell.start(
                repositoryId
                  ? { kind: "repository", repositoryId }
                  : { kind: "directory", cwd: "", workspace: null },
              )}
            onopen={openSession}
            onrename={shell.rename}
            onclose={shell.close}
            onremove={askRemove}
          />
        {:else if workspace && shell.repositories && !shell.sessions.loading && showWorkspaces}
          {@render workspaces()}
        {:else if !workspace || !shell.repositories}
          <ul class="space-y-0.5">
            {#each unpinned as session (session.id)}
              <SidebarSessionRow
                {session}
                {current}
                icon={session.agent?.icon ??
                  configIcons.get(session.configId ?? "") ??
                  ""}
                onopen={openSession}
                onrename={shell.rename}
                onclose={shell.close}
              />
            {/each}
          </ul>
        {/if}
      </div>
    </div>
  </ScrollArea>
  <SidebarFooter
    connection={shell.sessions.connection}
    hostname={setup.hostname}
    setupSettled={shell.setup.settled}
    {reconnection}
    onaddrepository={workspace && shell.repositories
      ? shell.addRepository
      : undefined}
    canAddRepository={setup.gitReady && setup.registryReady}
    onforward={shell.forward}
  />
</nav>

<ConfirmDialog
  bind:open={removeOpen}
  title={m.s1_delete_record_confirm_title({
    title: removing ? recordTitle(removing) : "",
  })}
  onconfirm={() => void confirmRemove()}
>
  {m.s1_delete_record_confirm_body()}
</ConfirmDialog>
{#if rebuilding && workspace && chat}
  {@const { record, released } = rebuilding}
  <LazyDialog
    load={chat.rebuild}
    label={m.s1_rebuild_worktree()}
    onclose={() => (rebuilding = null)}
  >
    {#snippet children(page)}
      <page.default
        id={record.id}
        title={recordTitle(record)}
        {released}
        onclose={() => (rebuilding = null)}
        onrebuilt={rebuilt}
      />
    {/snippet}
  </LazyDialog>
{/if}

<style>
  nav {
    --sidebar-gutter: 8px;
  }

  .sidebar-actions {
    padding-inline: var(--sidebar-gutter);
  }

  /* Short drawers need one scroller for both the header and its navigation. */
  @media (width < 40rem) and (height <= 600px) {
    .sidebar-drawer {
      overflow-y: auto;
      overscroll-behavior: contain;
    }

    .sidebar-drawer > :global([data-slot="scroll-area"]) {
      flex: none;
    }

    .sidebar-drawer
      > :global([data-slot="scroll-area"] [data-slot="scroll-area-viewport"]) {
      height: auto;
    }
  }
</style>
