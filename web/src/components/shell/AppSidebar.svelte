<script lang="ts">
  import IconLayoutSidebarLeftCollapse from "~icons/tabler/layout-sidebar-left-collapse";
  import IconMessagePlus from "~icons/tabler/message-plus";
  import IconPlus from "~icons/tabler/plus";
  import IconX from "~icons/tabler/x";
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
    openGitWorkbench,
    openPage,
    openSession,
    shellUi,
    type ShellMode,
  } from "../../hooks/shared/shell.svelte";
  import { workbenchLayout } from "../../domains/workspace/workbench-layout.svelte";
  import { queueGroup } from "../../domains/workspace/session-queue";
  import { paneEnter } from "../../hooks/shared/pane-enter";
  import Brand from "./Brand.svelte";
  import ConfirmDialog from "../shared/ConfirmDialog.svelte";
  import SidebarLiveSessionRow from "./SidebarLiveSessionRow.svelte";
  import ModeSwitch from "./ModeSwitch.svelte";
  import SidebarFooter from "./SidebarFooter.svelte";
  import { featureContributions } from "../../features/registry.svelte";
  import { featureAvailable } from "../../platform/feature-host.svelte";
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
    onmode,
    onprimary,
  }: {
    route: Extract<
      Route,
      { name: "sessions" | "quick-start" | "terminal" | "handoff" | "git" }
    >;
    /** Desktop only; the drawer closes on its own. */
    collapsible?: boolean;
    onclose?: () => void;
    reconnection?: RelayReconnection;
    onmode: (mode: ShellMode) => void;
    onprimary: () => void;
  } = $props();
  const shell = getShell();
  const current = $derived(route.name === "terminal" ? route.id : "");
  const live = $derived(
    shell.sessions.sessions.filter((session) => session.state === "active"),
  );
  const queue = $derived(
    Boolean(
      workspace &&
      shellUi.mode === "workbench" &&
      workbenchLayout.value === "queue",
    ),
  );
  const attentionCount = $derived(
    queue
      ? live.filter((session) => queueGroup(session) === "attention").length
      : 0,
  );
  const pinned = $derived(sessionPins.ordered(live));
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
  // Runs a worktree action of the tree, by the IDs its rows carry.
  function withTree(action: (tree: Worktree) => void) {
    return (repositoryId: string, worktreeId: string) => {
      const tree = shell.repositories?.list
        .find((r) => r.id === repositoryId)
        ?.worktrees.find((w) => w.id === worktreeId);
      if (tree) action(tree);
    };
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
  function revealNavigation(node: HTMLElement, _mode: ShellMode) {
    const animation = paneEnter(node, false);
    return {
      update() {
        animation.update(false);
        animation.update(true);
      },
      destroy: animation.destroy,
    };
  }
  function scope(next: string, worktreeId?: string) {
    workbenchLayout.attentionOnly = false;
    shellUi.setMode("workbench");
    shell.repositories?.select(next, worktreeId);
    openPage("/");
  }
</script>

<nav class="flex h-full min-h-0 flex-col" aria-label={m.s1_navigation()}>
  <div class="flex items-center gap-1 px-2 pt-2 pb-1">
    {#if collapsible}
      <Button
        class="shrink-0"
        variant="ghost"
        size="icon-touch"
        aria-label={m.s1_collapse_sidebar()}
        title={m.s1_collapse_sidebar()}
        onclick={() => shellUi.setCollapsed(true)}
        ><IconLayoutSidebarLeftCollapse aria-hidden="true" /></Button
      >
    {/if}
    <span class="brand mr-auto min-w-0 overflow-hidden px-1 whitespace-nowrap"
      ><Brand /></span
    >
    {#if onclose}
      <Button
        class="shrink-0"
        variant="ghost"
        size="icon-touch"
        aria-label={m.s1_close_sidebar()}
        title={m.s1_close_sidebar()}
        onclick={onclose}><IconX aria-hidden="true" /></Button
      >
    {/if}
  </div>
  {#if chat}
    <div class="px-2 pb-1">
      <ModeSwitch
        mode={shellUi.mode}
        workspaces={!!workspace}
        onchange={onmode}
      />
    </div>
  {/if}
  <div class="px-2 pt-1">
    <Button
      variant="outline"
      size="sm-touch"
      class="w-full"
      onclick={onprimary}
    >
      {#if shellUi.mode === "chat"}<IconMessagePlus
          aria-hidden="true"
        />{m.s1_new_chat()}{:else}<IconPlus
          aria-hidden="true"
        />{m.s1_new_session()}{/if}
    </Button>
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
      triggerLabel={queue ? m.s8_queue_notifications() : undefined}
    />{/if}
  <ScrollArea
    type="hover"
    class="min-h-0 flex-1 [&_[data-slot=scroll-area-viewport]]:overscroll-contain [&_[data-slot=scroll-area-viewport]>div]:!block"
  >
    <div class="px-2 py-2">
      {#if pinned.length && !queue}
        <section class="mb-3 border-b pb-3" aria-label={m.s1_pinned_sessions()}>
          <h2 class="px-3 pb-1 text-xs text-muted-foreground">
            {m.s1_pinned()}
          </h2>
          <ul class="space-y-0.5">
            {#each pinned as session (session.id)}
              <SidebarLiveSessionRow
                {session}
                {current}
                icon={session.agent?.icon ??
                  configIcons.get(session.configId ?? "") ??
                  ""}
                pinned
                onopen={openSession}
                onrename={shell.rename}
                onclose={shell.close}
              />
            {/each}
          </ul>
        </section>
      {/if}

      {#if resumeError || sessionPins.error}<p
          role="alert"
          class="mb-2 rounded-md border border-destructive/40 px-3 py-2 text-xs text-destructive"
        >
          {resumeError || sessionPins.error}
        </p>{/if}
      <div use:revealNavigation={shellUi.mode}>
        {#if shellUi.mode === "chat" && chat}
          <chat.History
            sessions={unpinned}
            records={shell.archived?.records ?? []}
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
          />
        {:else if workspace && shell.repositories && !shell.sessions.loading && (shell.repositories.settled || shell.repositories.list.length)}
          {#if queue}
            <Button
              variant="ghost"
              size="sm-touch"
              class="mb-2 w-full justify-between"
              aria-current={route.name === "sessions" &&
              workbenchLayout.attentionOnly
                ? "page"
                : undefined}
              onclick={() => {
                scope("all");
                workbenchLayout.attentionOnly = true;
              }}
            >
              {m.s8_queue_attention_view()}
              <span class="font-mono tabular-nums">{attentionCount}</span>
            </Button>
          {/if}
          <workspace.Tree
            repositories={shell.repositories.list}
            scopesOnly={queue}
            sessions={queue ? live : unpinned}
            home={workspace.homeWorktrees.value}
            scope={route.name === "sessions" && !workbenchLayout.attentionOnly
              ? (shell.repositories?.choice ?? "all")
              : ""}
            worktree={route.name === "sessions"
              ? (shell.repositories?.worktree ?? "")
              : ""}
            {current}
            canCreate={setup.gitReady && setup.registryReady}
            {configIcons}
            onscope={scope}
            onopen={openSession}
            onnewworktree={(repositoryId) =>
              shell.start({ kind: "new-worktree", repositoryId })}
            onhomeworktrees={(repositoryId) => {
              const repo = shell.repositories?.list.find(
                (r) => r.id === repositoryId,
              );
              if (repo) shell.chooseHomeWorktrees(repo);
            }}
            onmanageworktrees={(repositoryId) => {
              const repo = shell.repositories?.list.find(
                (r) => r.id === repositoryId,
              );
              if (repo) shell.repositoryDetails(repo, "worktrees");
            }}
            onrepositorydetails={(repositoryId) => {
              const repo = shell.repositories?.list.find(
                (r) => r.id === repositoryId,
              );
              if (repo) shell.repositoryDetails(repo);
            }}
            onworktreedetails={withTree((tree) => shell.worktreeDetails(tree))}
            onremoveworktree={withTree((tree) => shell.removeWorktree(tree))}
            onswitchbranch={withTree((tree) => shell.switchBranch(tree))}
            onbranchoperation={shell.branchOperation}
            onopengit={featureAvailable("workspace.git-workbench")
              ? (repositoryId, worktreeId, view) =>
                  openGitWorkbench({ repositoryId, worktreeId, view })
              : undefined}
            onaddrepository={() => shell.addRepository()}
            onrename={shell.rename}
            onclose={shell.close}
            records={shell.archived?.records ?? []}
            {resuming}
            onresume={open}
            onremove={askRemove}
          />
        {:else if !workspace || !shell.repositories}
          <ul class="space-y-0.5">
            {#each unpinned as session (session.id)}
              <SidebarLiveSessionRow
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
