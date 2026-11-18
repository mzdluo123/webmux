import { m } from "$lib/paraglide/messages.js";
import { SvelteMap, SvelteSet } from "svelte/reactivity";
import type {
  GitPrecedingCommit,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import type {
  GitLineSelection,
  GitStagingEntry,
  GitStagingFileDiff,
} from "../../contracts/schemas";
import { formatStageIdentityKey } from "../../domains/workspace/git-workbench-adapter";
import { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import { LatestRequest } from "../shared/remote-request.svelte";
import {
  runConfirmCommitFlow,
  runOpenCommitPreview,
  runSyncCommitPreflight,
} from "./git-commit-execution";
import type { GitCommitOptionsController } from "./git-commit-options.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import { createGitStagingDiffController } from "./git-staging-diff.svelte";
import {
  fetchGitStagingDiffForEntry,
  fetchGitStagingState,
} from "./git-workbench-actions";
import {
  executeGroupStageMutation,
  executeSingleStageMutation,
} from "./git-workbench-staging-mutations";

export interface GitStagingBindings {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  operations?: GitOperationsController;
  commitOptions?: GitCommitOptionsController;
  onRefreshRepositories?: () => void;
  onAfterCommitRefresh?: () => void;
  onStagingRefreshed?: (
    repoId: string,
    wtId: string,
    signal?: AbortSignal,
  ) => void;
  onBeforeOpenPreview?: () => Promise<boolean>;
  onCommitDraftCleared?: () => Promise<void> | void;
}

export function createGitStagingController(bindings: GitStagingBindings) {
  let stagingBusy = $state(false);
  let commitSubmitting = $state(false);
  let commitConfirmationId = $state<string | null>(null);
  let activeDiffStale = $state(false);
  let activeDiffStaleReason = $state("");

  const entryById = new SvelteMap<string, GitStagingEntry>();
  const diffCache = new SvelteMap<string, GitStagingFileDiff>();
  const webmuxStagedPaths = new SvelteSet<string>();

  const diffReq = new LatestRequest();
  const actionReq = new LatestRequest();
  const previewReq = new LatestRequest();
  const commitReq = new LatestRequest();
  const preflightReq = new LatestRequest();
  const requests = [diffReq, actionReq, previewReq, commitReq, preflightReq];
  let disposed = false;

  const reader = createGitStagingDiffController({
    getRepoId: bindings.getRepoId,
    getWorktreeId: bindings.getWorktreeId,
  });

  function isSameTarget(repoId: string, wtId: string | undefined): boolean {
    return (
      !disposed &&
      repoId === bindings.getRepoId() &&
      wtId === bindings.getWorktreeId()
    );
  }

  function dispose() {
    disposed = true;
    for (const r of requests) r.dispose();
    reader.dispose();
  }

  function resetTarget() {
    for (const r of requests) r.cancel();
    stagingBusy = false;
    commitConfirmationId = null;
    commitSubmitting = false;
    activeDiffStale = false;
    activeDiffStaleReason = "";
    entryById.clear();
    diffCache.clear();
    webmuxStagedPaths.clear();
    reader.resetTarget();
  }

  async function syncPreflightMetadata(
    repoId: string,
    wtId: string,
    signal?: AbortSignal,
  ) {
    const req = preflightReq.begin();
    const effectiveSignal = signal ?? req.signal;
    await runSyncCommitPreflight({
      repoId,
      wtId,
      signal: effectiveSignal,
      isCurrent: () => req.current() && isSameTarget(repoId, wtId),
      commitOptions: bindings.commitOptions,
      getLatestView: bindings.getView,
      patchView: bindings.patchView,
    });
  }

  async function loadActiveDiff(
    repoId: string,
    wtId: string,
    changeId: string | null,
    forceReload = false,
  ) {
    if (!repoId || !wtId || !changeId) {
      void reader.loadForChange(null);
      return;
    }
    const currentView = bindings.getView();
    const initialChange =
      currentView.workingChanges.find((c) => c.id === changeId) ?? null;
    if (forceReload) {
      activeDiffStale = false;
      activeDiffStaleReason = "";
    }
    void reader.loadForChange(initialChange);

    const entry = entryById.get(changeId);
    if (!entry) return;
    const cacheKey = `${entry.area}:${entry.path}`;
    if (!forceReload && diffCache.has(cacheKey)) return;

    const req = diffReq.begin();
    try {
      const out = await fetchGitStagingDiffForEntry({
        repositoryId: repoId,
        worktreeId: wtId,
        entry,
        signal: req.signal,
      });
      if (!out || !req.current() || !isSameTarget(repoId, wtId)) {
        return;
      }
      diffCache.set(out.diffKey, out.diff);
      entryById.set(changeId, { ...entry, identity: out.diff.identity });
      const nextIdentityKey = formatStageIdentityKey(out.diff.identity);
      const current = bindings.getView();
      const nextChanges = current.workingChanges.map((item) =>
        item.id === changeId
          ? {
              ...item,
              binary: out.diff.binary,
              hunkStageAvailable: out.diff.hunkStageAvailable,
              degradeReason: out.diff.degradeReason,
              identityKey: nextIdentityKey,
              hunks: out.diff.hunks,
            }
          : item,
      );
      bindings.patchView({ workingChanges: nextChanges });
      if (!reader.state.rawDiff && current.activeChangeId === changeId) {
        const updated = nextChanges.find((c) => c.id === changeId) ?? null;
        void reader.loadForChange(updated);
      }
    } catch {
      /* Whole-file stage remains available */
    }
  }

  async function refresh(
    repoId: string,
    wtId: string,
    signal?: AbortSignal,
    options?: { preserveReadingOnStale?: boolean },
  ) {
    const prevView = bindings.getView();
    const prevActive = prevView.workingChanges.find(
      (c) => c.id === prevView.activeChangeId,
    );

    const out = await fetchGitStagingState({
      repositoryId: repoId,
      worktreeId: wtId,
      diffCache,
      webmuxStagedPaths,
      signal,
    });
    if (signal?.aborted || !isSameTarget(repoId, wtId)) {
      return;
    }

    const nowStagedPaths = new SvelteSet(
      out.changes.filter((c) => c.group === "staged").map((c) => c.path),
    );
    for (const p of Array.from(webmuxStagedPaths)) {
      if (!nowStagedPaths.has(p)) webmuxStagedPaths.delete(p);
    }

    entryById.clear();
    for (const [id, entry] of out.entryById) entryById.set(id, entry);

    const current = bindings.getView();
    const nextActiveObj =
      out.changes.find((c) => c.id === current.activeChangeId) ??
      out.changes.find((c) => prevActive && c.path === prevActive.path) ??
      out.changes[0] ??
      null;
    const nextActive = nextActiveObj?.id ?? null;

    const becameStale =
      Boolean(options?.preserveReadingOnStale) &&
      Boolean(prevActive) &&
      (!nextActiveObj ||
        nextActiveObj.id !== prevActive!.id ||
        nextActiveObj.identityKey !== prevActive!.identityKey);

    bindings.patchView({
      workingChanges: out.changes,
      activeChangeId: becameStale ? prevActive!.id : nextActive,
    });
    writeGitTargetMemory(repoId, wtId, {
      activeChangeId: becameStale ? prevActive!.id : nextActive,
    });

    if (becameStale) {
      activeDiffStale = true;
      activeDiffStaleReason = m.s7_staging_external_stale_reading();
    } else {
      activeDiffStale = false;
      activeDiffStaleReason = "";
      if (nextActive) void loadActiveDiff(repoId, wtId, nextActive);
      else void reader.loadForChange(null);
    }

    void syncPreflightMetadata(repoId, wtId, signal);
    bindings.onStagingRefreshed?.(repoId, wtId, signal);
  }

  async function refreshActiveSnapshot() {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) return;
    diffCache.clear();
    activeDiffStale = false;
    activeDiffStaleReason = "";
    await refresh(repoId, wtId);
    if (!isSameTarget(repoId, wtId)) return;
    const activeId = bindings.getView().activeChangeId;
    if (activeId) await loadActiveDiff(repoId, wtId, activeId, true);
  }

  async function checkExternalUpdates() {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId || stagingBusy || commitSubmitting) return;
    diffCache.clear();
    await refresh(repoId, wtId, undefined, {
      preserveReadingOnStale: true,
    }).catch(() => undefined);
  }

  async function reloadStagingAfterMutation(
    repoId: string,
    wtId: string,
    signal: AbortSignal,
  ) {
    diffCache.clear();
    activeDiffStale = false;
    activeDiffStaleReason = "";
    await refresh(repoId, wtId, signal).catch(() => undefined);
  }

  async function stageOrUnstage(
    kind: "stage" | "unstage",
    changeId: string,
    hunkId?: string,
    lineSelections?: readonly GitLineSelection[],
  ) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId || stagingBusy || commitSubmitting) return;
    const entry = entryById.get(changeId);
    if (!entry) return;
    const req = actionReq.begin();
    await executeSingleStageMutation({
      kind,
      repoId,
      wtId,
      entry,
      hunkId,
      lineSelections,
      req,
      isSameTarget: () => isSameTarget(repoId, wtId),
      onRecordStagedPath: (path) => webmuxStagedPaths.add(path),
      onReloadAfterMutation: (sig) =>
        reloadStagingAfterMutation(repoId, wtId, sig),
      onMarkStale: (reason) => {
        activeDiffStale = true;
        activeDiffStaleReason = reason;
      },
      patchView: bindings.patchView,
      onRefreshRepositories: bindings.onRefreshRepositories,
      setStagingBusy: (busy) => (stagingBusy = busy),
    });
  }

  async function stageOrUnstageGroup(
    group: "staged" | "unstaged" | "untracked",
  ) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId || stagingBusy || commitSubmitting) return;
    const targets = bindings
      .getView()
      .workingChanges.filter((c) => c.group === group);
    if (targets.length === 0) return;
    const req = actionReq.begin();
    await executeGroupStageMutation({
      kind: group === "staged" ? "unstage" : "stage",
      repoId,
      wtId,
      targets,
      getEntry: (id) => entryById.get(id),
      req,
      isSameTarget: () => isSameTarget(repoId, wtId),
      onRecordStagedPath: (path) => webmuxStagedPaths.add(path),
      onReloadAfterMutation: (sig) =>
        reloadStagingAfterMutation(repoId, wtId, sig),
      patchView: bindings.patchView,
      onRefreshRepositories: bindings.onRefreshRepositories,
      setStagingBusy: (busy) => (stagingBusy = busy),
    });
  }

  const canApplyPreview = (
    req: { current(): boolean },
    repoId: string,
    wtId: string,
  ) => req.current() && isSameTarget(repoId, wtId) && !commitSubmitting;

  async function openCommitPreview(followUpPush: boolean) {
    if (commitSubmitting) return;
    commitConfirmationId = null;
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) return;
    const req = previewReq.begin();
    if (bindings.onBeforeOpenPreview) {
      const ok = await bindings.onBeforeOpenPreview();
      if (!ok || !canApplyPreview(req, repoId, wtId)) return;
    }
    await runOpenCommitPreview({
      repoId,
      wtId,
      view: bindings.getView(),
      followUpPush,
      previewRequest: bindings.commitOptions?.buildPreviewRequest(),
      signal: req.signal,
      isCurrent: () => canApplyPreview(req, repoId, wtId),
      isReportable: () =>
        req.reportable() && canApplyPreview(req, repoId, wtId),
      setConfirmationId: (id) => (commitConfirmationId = id),
      getLatestView: bindings.getView,
      patchView: bindings.patchView,
    });
  }

  async function refreshCommitPreview() {
    if (commitSubmitting || stagingBusy) return;
    const preview = bindings.getView().commitPreview;
    if (!preview) return;
    commitConfirmationId = null;
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) return;
    const req = previewReq.begin();
    diffCache.clear();
    await refresh(repoId, wtId).catch(() => undefined);
    if (
      !canApplyPreview(req, repoId, wtId) ||
      !bindings.getView().commitPreview
    )
      return;
    await openCommitPreview(preview.followUpPush);
  }

  function closeCommitPreview() {
    if (commitSubmitting) return;
    previewReq.cancel();
    commitConfirmationId = null;
    bindings.patchView({ commitPreview: null });
  }

  async function confirmCommit(
    onFollowUpPush?: (
      branch: string,
      precedingCommit: GitPrecedingCommit,
    ) => void,
  ) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    const current = bindings.getView();
    if (
      !current.commitPreview ||
      current.commitPreview.stale ||
      !commitConfirmationId ||
      !repoId ||
      !wtId ||
      commitSubmitting
    ) {
      return;
    }
    const confirmationId = commitConfirmationId;
    previewReq.cancel();
    commitConfirmationId = null;
    commitSubmitting = true;
    const req = commitReq.begin();
    try {
      await runConfirmCommitFlow({
        repoId,
        wtId,
        confirmationId,
        current,
        acknowledgePushedAmend: bindings.commitOptions?.acknowledgePushedAmend,
        signal: req.signal,
        isCurrentTarget: () => isSameTarget(repoId, wtId),
        operations: bindings.operations,
        clearWebmuxStaged: () => webmuxStagedPaths.clear(),
        refreshStaging: async () => {
          if (!isSameTarget(repoId, wtId)) return;
          diffCache.clear();
          await refresh(repoId, wtId).catch(() => undefined);
        },
        patchView: bindings.patchView,
        onRefreshRepositories: bindings.onRefreshRepositories,
        onAfterCommitRefresh: bindings.onAfterCommitRefresh,
        onCommitDraftCleared: bindings.onCommitDraftCleared,
        onCommitOptionsReset: () => bindings.commitOptions?.resetAfterCommit(),
        onFollowUpPush,
      });
    } finally {
      if (isSameTarget(repoId, wtId)) {
        commitSubmitting = false;
      }
    }
  }

  return {
    get stagingBusy() {
      return stagingBusy;
    },
    get commitSubmitting() {
      return commitSubmitting;
    },
    get activeDiffStale() {
      return activeDiffStale;
    },
    get activeDiffStaleReason() {
      return activeDiffStaleReason;
    },
    get reader() {
      return reader;
    },
    getEntry(changeId: string) {
      return entryById.get(changeId);
    },
    dispose,
    resetTarget,
    seedContextId: reader.seedContextId,
    loadActiveDiff,
    refresh,
    refreshActiveSnapshot,
    checkExternalUpdates,
    stageOrUnstage,
    stageOrUnstageGroup,
    openCommitPreview,
    refreshCommitPreview,
    closeCommitPreview,
    confirmCommit,
  };
}
