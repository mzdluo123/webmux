import { SvelteMap } from "svelte/reactivity";
import { errorMessage } from "../../platform/error";
import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import type {
  GitStagingEntry,
  GitStagingFileDiff,
} from "../../contracts/schemas";
import { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import { LatestRequest } from "../shared/remote-request.svelte";
import {
  fetchGitStagingDiffForEntry,
  fetchGitStagingState,
  performGitCommitExecution,
  performStageMutation,
  requestCommitPreviewModel,
} from "./git-workbench-actions";

export interface GitStagingBindings {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  onRefreshRepositories?: () => void;
}

export function createGitStagingController(bindings: GitStagingBindings) {
  let stagingBusy = $state(false);
  let commitConfirmationId = $state<string | null>(null);
  const entryById = new SvelteMap<string, GitStagingEntry>();
  const diffCache = new SvelteMap<string, GitStagingFileDiff>();

  const diffReq = new LatestRequest();
  const actionReq = new LatestRequest();

  function dispose() {
    diffReq.dispose();
    actionReq.dispose();
  }

  function resetTarget() {
    commitConfirmationId = null;
    entryById.clear();
    diffCache.clear();
  }

  async function loadActiveDiff(
    repoId: string,
    wtId: string,
    changeId: string | null,
  ) {
    if (!repoId || !wtId || !changeId) return;
    const entry = entryById.get(changeId);
    if (!entry || diffCache.has(`${entry.area}:${entry.path}`)) return;
    const req = diffReq.begin();
    try {
      const out = await fetchGitStagingDiffForEntry({
        repositoryId: repoId,
        worktreeId: wtId,
        entry,
        signal: req.signal,
      });
      if (
        !out ||
        !req.current() ||
        repoId !== bindings.getRepoId() ||
        wtId !== bindings.getWorktreeId()
      ) {
        return;
      }
      diffCache.set(out.diffKey, out.diff);
      const current = bindings.getView();
      bindings.patchView({
        workingChanges: current.workingChanges.map((item) =>
          item.id === changeId
            ? {
                ...item,
                binary: out.diff.binary,
                hunkStageAvailable: out.diff.hunkStageAvailable,
                degradeReason: out.diff.degradeReason,
                hunks: out.diff.hunks,
              }
            : item,
        ),
      });
    } catch {
      /* Whole-file stage remains available */
    }
  }

  async function refresh(repoId: string, wtId: string, signal?: AbortSignal) {
    const out = await fetchGitStagingState({
      repositoryId: repoId,
      worktreeId: wtId,
      diffCache,
      signal,
    });
    if (repoId !== bindings.getRepoId() || wtId !== bindings.getWorktreeId()) {
      return;
    }
    entryById.clear();
    for (const [id, entry] of out.entryById) entryById.set(id, entry);
    const current = bindings.getView();
    const nextActive =
      out.changes.find((c) => c.id === current.activeChangeId)?.id ??
      out.changes[0]?.id ??
      null;
    bindings.patchView({
      workingChanges: out.changes,
      activeChangeId: nextActive,
    });
    writeGitTargetMemory(repoId, wtId, { activeChangeId: nextActive });
    if (nextActive) void loadActiveDiff(repoId, wtId, nextActive);
  }

  async function stageOrUnstage(
    kind: "stage" | "unstage",
    changeId: string,
    hunkId?: string,
  ) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId || stagingBusy) return;
    const entry = entryById.get(changeId);
    if (!entry) return;
    stagingBusy = true;
    const req = actionReq.begin();
    try {
      const out = await performStageMutation({
        kind,
        repositoryId: repoId,
        worktreeId: wtId,
        entry,
        hunkId,
        signal: req.signal,
      });
      diffCache.clear();
      await refresh(repoId, wtId, req.signal).catch(() => undefined);
      if (out.ok) bindings.onRefreshRepositories?.();
      else if (out.busyOutcome)
        bindings.patchView({ lastCommitOutcome: out.busyOutcome });
      else if (out.banner) bindings.patchView({ statusBanner: out.banner });
    } finally {
      stagingBusy = false;
    }
  }

  async function openCommitPreview(followUpPush: boolean) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) return;
    const req = actionReq.begin();
    const current = bindings.getView();
    try {
      const res = await requestCommitPreviewModel({
        repositoryId: repoId,
        worktreeId: wtId,
        repositoryName: current.context.repositoryName,
        workingChanges: current.workingChanges,
        followUpPush,
        signal: req.signal,
      });
      if (!req.current()) return;
      commitConfirmationId = res.confirmationId;
      const latest = bindings.getView();
      bindings.patchView({
        context: {
          ...latest.context,
          authorName: res.authorName,
          authorEmail: res.authorEmail,
          signEnabled: res.signEnabled,
          executableHooks: res.executableHooks,
        },
        commitPreview: res.model,
      });
    } catch (err) {
      bindings.patchView({
        statusBanner: `提交预览读取失败：${errorMessage(err)}`,
      });
    }
  }

  function closeCommitPreview() {
    commitConfirmationId = null;
    bindings.patchView({ commitPreview: null });
  }

  async function confirmCommit(onFollowUpPush?: (branch: string) => void) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    const current = bindings.getView();
    const preview = current.commitPreview;
    if (!preview || !commitConfirmationId || !repoId || !wtId) return;
    const req = actionReq.begin();
    const out = await performGitCommitExecution({
      repositoryId: repoId,
      worktreeId: wtId,
      confirmationId: commitConfirmationId,
      message: current.commitMessage,
      signal: req.signal,
    });
    commitConfirmationId = null;
    if (out.ok) {
      const nextMsg =
        out.outcome.status === "matched" ? "" : current.commitMessage;
      writeGitTargetMemory(repoId, wtId, { commitMessage: nextMsg });
      bindings.patchView({
        commitMessage: nextMsg,
        commitPreview: null,
        lastCommitOutcome: out.outcome,
        statusBanner: out.outcome.summary,
      });
      diffCache.clear();
      await refresh(repoId, wtId).catch(() => undefined);
      bindings.onRefreshRepositories?.();
      if (preview.followUpPush && out.outcome.status === "matched") {
        onFollowUpPush?.(preview.branch);
      }
    } else if (out.busyOutcome) {
      bindings.patchView({
        commitPreview: null,
        lastCommitOutcome: out.busyOutcome,
      });
    } else if (out.banner) {
      bindings.patchView({ statusBanner: out.banner });
    }
  }

  return {
    get stagingBusy() {
      return stagingBusy;
    },
    dispose,
    resetTarget,
    loadActiveDiff,
    refresh,
    stageOrUnstage,
    openCommitPreview,
    closeCommitPreview,
    confirmCommit,
  };
}
