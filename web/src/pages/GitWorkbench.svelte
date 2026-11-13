<script lang="ts">
  import { onDestroy } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { replacePath } from "../platform/api";
  import {
    resetBrowserChrome,
    setBrowserChrome,
  } from "../platform/browser-chrome";
  import type {
    GitPrecedingCommit,
    GitWorkbenchMode,
  } from "../contracts/git-workbench-schemas";
  import {
    readGitDeviceLayout,
    resetGitDeviceLayout,
    resolveGitDeviceClass,
    writeGitDeviceLayout,
    type GitDeviceLayout,
  } from "../domains/workspace/git-workbench-memory";
  import { filterActiveWorktreeSessions } from "../domains/workspace/git-rework-model";
  import {
    GIT_CONFLICT_HANDOFF_EVENT,
    type GitConflictHandoff,
  } from "../domains/workspace/git-conflict-handoff";
  import { buildGitRouteHref, parseGitRouteQuery } from "../hooks/shared/route";
  import {
    getShell,
    openGitWorkbench,
    openSession,
  } from "../hooks/shared/shell.svelte";
  import { dispatchGitPageBranchAction } from "../hooks/workspace/git-branch-action-dispatch";
  import { createGitWorkbenchController } from "../hooks/workspace/git-workbench-controller.svelte";
  import InlineNotice from "../components/shared/InlineNotice.svelte";
  import GitCommitOutcomeBanner from "../components/workspace/GitCommitOutcomeBanner.svelte";
  import GitWorkbenchPageDialogs from "../components/workspace/GitWorkbenchPageDialogs.svelte";
  import GitWorkbenchStatusBar from "../components/workspace/GitWorkbenchStatusBar.svelte";
  import GitWorkbenchTopBar from "../components/workspace/GitWorkbenchTopBar.svelte";
  import GitWorkbenchWorkspace from "../components/workspace/GitWorkbenchWorkspace.svelte";
  import GitWorktreeGuardState from "../components/workspace/GitWorktreeGuardState.svelte";

  let {
    repositoryId,
    worktreeId,
    search = "",
  }: {
    repositoryId: string;
    worktreeId?: string;
    search?: string;
  } = $props();

  const shell = getShell();
  let viewportWidth = $state(
    typeof window !== "undefined" ? window.innerWidth : 1280,
  );
  let layoutRevision = $state(0);

  const deviceClass = $derived(resolveGitDeviceClass(viewportWidth));
  const deviceLayout = $derived.by(() => {
    void layoutRevision;
    return readGitDeviceLayout(deviceClass);
  });

  function updateDeviceLayout(patch: Partial<GitDeviceLayout>) {
    writeGitDeviceLayout(deviceClass, patch);
    layoutRevision += 1;
  }

  function handleResetDeviceLayout() {
    resetGitDeviceLayout(deviceClass);
    layoutRevision += 1;
  }

  const controller = createGitWorkbenchController({
    onRepositoryUpdated: (repo) => shell.repositories.replace(repo),
    onRefreshRepositories: () => void shell.repositories.refresh(true),
  });
  onDestroy(() => {
    controller.dispose();
    resetBrowserChrome();
  });

  const repository = $derived(
    repositoryId
      ? shell.repositories.list.find((r) => r.id === repositoryId)
      : undefined,
  );
  const worktree = $derived(
    worktreeId
      ? repository?.worktrees.find((w) => w.id === worktreeId)
      : undefined,
  );
  const activeWorktreeSessions = $derived(
    filterActiveWorktreeSessions(
      shell.sessions.sessions,
      worktreeId,
      worktree?.path,
    ),
  );

  // Visiting `/git` directly picks the remembered or first registered repository.
  $effect(() => {
    if (repositoryId || !shell.repositories.settled) return;
    if (shell.repositories.list.length === 0) return;
    const preferred =
      shell.repositories.list.find((r) => r.id === shell.repositories.choice) ??
      shell.repositories.list[0];
    if (!preferred) return;
    const query = parseGitRouteQuery(search);
    replacePath(buildGitRouteHref({ repositoryId: preferred.id, ...query }));
  });

  $effect(() => {
    controller.syncRoute({
      repositoryId,
      worktreeId,
      search,
      repository,
      worktree,
      sessions: shell.sessions.sessions,
    });
  });

  $effect(() => {
    const title = repository?.name
      ? `${repository.name} · Git 工作台 · WebMux`
      : "Git 工作台 · WebMux";
    const running = shell.sessions.sessions.some((s) => s.status === "running");
    setBrowserChrome(title, running);
  });

  $effect(() => {
    if (typeof window === "undefined" || typeof document === "undefined") {
      return;
    }
    const onFocus = () => {
      if (document.visibilityState === "visible") {
        void controller.checkExternalUpdates();
      }
    };
    const onConflictHandoff = (e: Event) => {
      const detail = (e as CustomEvent<GitConflictHandoff>).detail;
      if (detail?.repositoryId === repositoryId) {
        void controller.openConflictHandoff(detail.path);
      }
    };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onFocus);
    window.addEventListener(GIT_CONFLICT_HANDOFF_EVENT, onConflictHandoff);
    return () => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onFocus);
      window.removeEventListener(GIT_CONFLICT_HANDOFF_EVENT, onConflictHandoff);
    };
  });

  const returnSession = $derived(
    controller.returnSessionId
      ? shell.sessions.sessions.find((s) => s.id === controller.returnSessionId)
      : undefined,
  );
  const returnSessionArchived = $derived(
    Boolean(
      controller.returnSessionId &&
      shell.archived.records.some((r) => r.id === controller.returnSessionId),
    ),
  );
  const canAddRepository = $derived(
    shell.setup.value.gitReady && shell.setup.value.registryReady,
  );

  const pageGuard = $derived.by(() => {
    if (!shell.repositories.settled && shell.repositories.list.length === 0) {
      return null;
    }
    if (shell.repositories.list.length === 0) return "empty-registry" as const;
    if (!repositoryId) return null;
    if (!repository) return "missing-repository" as const;
    if (repository.availability !== "available") {
      return "unavailable-repository" as const;
    }
    return null;
  });

  const commitGuard = $derived.by(() => {
    if (!repository || repository.availability !== "available") return null;
    if (!worktreeId || worktree?.kind === "bare") {
      return "pick-worktree" as const;
    }
    if (
      !worktree ||
      !worktree.available ||
      worktree.prunable ||
      worktree.locked
    ) {
      return "broken-worktree" as const;
    }
    return null;
  });

  const writableWorktree = $derived(
    worktree &&
      worktree.available &&
      !worktree.prunable &&
      !worktree.locked &&
      worktree.kind !== "bare"
      ? worktree
      : undefined,
  );

  function switchRepository(nextRepoId: string) {
    openGitWorkbench({
      repositoryId: nextRepoId,
      returnSession: controller.returnSessionId,
    });
  }

  function switchWorktree(
    nextWorktreeId?: string,
    forceMode?: GitWorkbenchMode,
  ) {
    if (!repositoryId) return;
    openGitWorkbench({
      repositoryId,
      worktreeId: nextWorktreeId,
      view: forceMode ?? controller.view.mode,
      returnSession: controller.returnSessionId,
    });
  }

  const openNewWorktree = () =>
    repository &&
    shell.start({ kind: "new-worktree", repositoryId: repository.id });
  const openManageWorktrees = () =>
    repository && shell.repositoryDetails(repository, "worktrees");

  function handleUpdate(branchOverride?: string) {
    const branch = branchOverride ?? controller.view.context.checkedOutBranch;
    if (repository && branch) {
      shell.branchOperation(repository.id, { kind: "fast-forward", branch });
    }
  }

  function handlePush(
    branchOverride?: string,
    preceding?: GitPrecedingCommit | null,
  ) {
    const branch = branchOverride ?? controller.view.context.checkedOutBranch;
    if (repository && branch) controller.push.openDialog(branch, preceding);
  }

  function handleCheckoutRequest(branch: string) {
    if (!repository) return;
    if (writableWorktree) shell.switchBranch(writableWorktree, branch);
    else {
      shell.start({
        kind: "new-worktree",
        repositoryId: repository.id,
        preset: { mode: "existing", branch },
      });
    }
  }

  function handleCreateBranchFromHead() {
    if (!repository) return;
    const headPoint =
      controller.view.context.checkedOutBranch ||
      controller.view.context.headOid ||
      "HEAD";
    if (writableWorktree) {
      shell.switchBranch(writableWorktree, {
        mode: "new",
        startPoint: headPoint,
      });
    } else {
      shell.start({
        kind: "new-worktree",
        repositoryId: repository.id,
        preset: { mode: "new", startBranch: headPoint },
      });
    }
  }
</script>

<svelte:window bind:innerWidth={viewportWidth} />

<main
  class="git-workbench flex h-dvh min-h-0 min-w-0 flex-col bg-background text-foreground"
  data-testid="git-workbench-page"
  data-device-class={deviceClass}
>
  <GitWorkbenchTopBar
    repositories={shell.repositories.list}
    {repository}
    {worktreeId}
    {worktree}
    context={controller.view.context}
    branches={controller.view.branches}
    branchFilter={controller.view.branchFilter}
    fetching={controller.fetching}
    {canAddRepository}
    returnSessionId={controller.returnSessionId}
    {returnSession}
    {returnSessionArchived}
    sessionsReady={shell.sessions.connection === "connected"}
    reworkCount={controller.rework.items.length}
    onOpenRework={writableWorktree
      ? () => controller.rework.openReworkSheet(activeWorktreeSessions)
      : undefined}
    onSelectRepository={switchRepository}
    onSelectWorktree={(wtId) => switchWorktree(wtId)}
    onAddRepository={() => shell.addRepository()}
    onNewWorktree={openNewWorktree}
    onManageWorktrees={openManageWorktrees}
    onBranchFilterChange={(branch) =>
      controller.setBranchFilter(
        branch,
        repository,
        worktree,
        shell.sessions.sessions,
        true,
      )}
    onBranchAction={(kind, item) =>
      dispatchGitPageBranchAction({
        kind,
        item,
        repository,
        worktree,
        writableWorktree,
        sessions: shell.sessions.sessions,
        shell,
        controller,
        switchWorktree,
        openPush: handlePush,
      })}
    onFetch={() => void controller.runFetch(worktree, shell.sessions.sessions)}
    onUpdate={() => handleUpdate()}
    onPush={() => handlePush()}
    onReturnSession={(id) => openSession({ id })}
  />

  {#if pageGuard}
    <GitWorktreeGuardState
      kind={pageGuard}
      repositories={shell.repositories.list}
      {repository}
      {worktree}
      {canAddRepository}
      onAddRepository={() => shell.addRepository()}
      onSelectRepository={switchRepository}
      onSelectWorktree={(wtId) => switchWorktree(wtId)}
      onNewWorktree={openNewWorktree}
      onManageWorktrees={openManageWorktrees}
    />
  {:else if !repository}
    <div
      class="flex flex-1 items-center justify-center p-6 text-xs text-muted-foreground"
      role="status"
    >
      正在加载仓库工作台…
    </div>
  {:else}
    {#if controller.error}
      <div class="border-b border-border bg-background px-3 py-2">
        <InlineNotice tone="danger">{controller.error}</InlineNotice>
      </div>
    {/if}

    {#if worktreeId && commitGuard === "broken-worktree" && controller.view.mode !== "commit"}
      <div class="border-b border-border bg-background px-3 py-2">
        <InlineNotice tone="warning">
          目标工作树不可用或已锁定，写操作已禁用；仓库历史仍可只读浏览。
          <Button
            size="xs"
            variant="outline"
            class="ml-2"
            onclick={openManageWorktrees}
          >
            管理工作树…
          </Button>
        </InlineNotice>
      </div>
    {/if}

    {#if controller.view.lastCommitOutcome && controller.view.lastCommitOutcome.status !== "matched" && !controller.divergence.inProgress?.active}
      <GitCommitOutcomeBanner
        outcome={controller.view.lastCommitOutcome}
        onInspectCommit={(oid) => controller.inspectCommitInLog(oid)}
        onReloadVerify={() =>
          void controller.reloadAndVerifyUncertain(
            repository,
            worktree,
            shell.sessions.sessions,
          )}
        onDismiss={() => controller.dismissLastCommitOutcome()}
      />
    {/if}

    <GitWorkbenchWorkspace
      {controller}
      repositories={shell.repositories.list}
      {repository}
      {worktree}
      sessions={shell.sessions.sessions}
      archivedRecords={shell.archived.records}
      {returnSession}
      {commitGuard}
      {canAddRepository}
      {deviceLayout}
      hasActiveWorktreeSession={activeWorktreeSessions.length > 0}
      onUpdateDeviceLayout={updateDeviceLayout}
      onResetDeviceLayout={handleResetDeviceLayout}
      onAddRepository={() => shell.addRepository()}
      onSelectRepository={switchRepository}
      onSwitchWorktree={switchWorktree}
      onNewWorktree={openNewWorktree}
      onManageWorktrees={openManageWorktrees}
      onRequestCheckout={handleCheckoutRequest}
      onOpenPush={(branch) => handlePush(branch)}
      onBranchAction={(kind, item) =>
        dispatchGitPageBranchAction({
          kind,
          item,
          repository,
          worktree,
          writableWorktree,
          sessions: shell.sessions.sessions,
          shell,
          controller,
          switchWorktree,
          openPush: handlePush,
        })}
      onCreateBranchFromHead={handleCreateBranchFromHead}
      onOpenWorktreeTerminal={() => {
        const s = activeWorktreeSessions[0];
        if (s) openSession(s);
      }}
      onNewWorktreeTerminal={() =>
        worktreeId &&
        shell.start({
          kind: "worktree",
          repositoryId: repository.id,
          worktreeId,
        })}
      onOpenSession={(s) => openSession(s)}
      onArchiveSession={(s) => shell.close(s)}
      onRemoveWorktree={writableWorktree && writableWorktree.kind === "linked"
        ? () =>
            shell.removeWorktree(writableWorktree, {
              onremoved: () => switchWorktree(undefined, "log"),
            })
        : undefined}
    />

    <GitWorkbenchStatusBar
      worktreePath={controller.view.context.worktreePath}
      statusBanner={controller.view.statusBanner}
      lastFetchedAt={controller.view.lastFetchedAt ?? repository.lastFetchedAt}
      activeOperation={controller.operations.active}
      operations={controller.operations.operations}
      onInspectCommit={(oid) => controller.inspectCommitInLog(oid)}
      onRefreshOperations={() => void controller.operations.refreshOperations()}
      onReloadVerify={() =>
        void controller.reloadAndVerifyUncertain(
          repository,
          worktree,
          shell.sessions.sessions,
        )}
    />

    <GitWorkbenchPageDialogs
      {controller}
      {shell}
      {repositoryId}
      {worktreeId}
      {activeWorktreeSessions}
      onCreateBranchFromHead={handleCreateBranchFromHead}
      onPush={handlePush}
      onFetchRemote={() =>
        void controller.runFetch(worktree, shell.sessions.sessions)}
      onFastForwardBranch={(branch) => handleUpdate(branch)}
    />
  {/if}
</main>
