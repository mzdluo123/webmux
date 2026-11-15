import { errorMessage } from "../../platform/error";
import { gitCompare } from "../../platform/git-api";
import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import { mapGitCompareChanges } from "../../domains/workspace/git-workbench-adapter";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitLogDiffController } from "./git-log-diff.svelte";

export interface GitLogCompareControllerDeps {
  getRepoId: () => string;
  getEffectiveWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  logDiff: GitLogDiffController;
}

export function createGitLogCompareController(
  deps: GitLogCompareControllerDeps,
) {
  const compareReq = new LatestRequest();
  const compareMoreReq = new LatestRequest();

  function dispose() {
    compareReq.dispose();
    compareMoreReq.dispose();
  }

  function resetTarget() {
    compareReq.cancel();
    compareMoreReq.cancel();
  }

  function markCompareBase(oid: string | null) {
    deps.patchView({ compareBaseMarkOid: oid });
  }

  async function startCompare(baseOid: string, targetOid: string) {
    const repoId = deps.getRepoId();
    const worktreeId = deps.getEffectiveWorktreeId();
    if (!repoId || !baseOid || !targetOid) return;
    if (baseOid === targetOid) {
      deps.patchView({
        statusBanner: "请选择两个不同的提交版本进行比较。",
      });
      return;
    }

    compareMoreReq.cancel();
    deps.logDiff.clearDiff();
    const req = compareReq.begin();
    deps.patchView({
      compareBaseMarkOid: null,
      compareMode: {
        baseOid,
        targetOid,
        files: [],
        nextCursor: "",
        loading: true,
        loadingMore: false,
        error: "",
      },
    });

    try {
      const res = await deps.logDiff.withContext(
        repoId,
        worktreeId,
        req.signal,
        (cid) =>
          gitCompare(
            repoId,
            { contextId: cid, worktreeId, baseOid, targetOid },
            req.signal,
          ),
      );
      if (!req.current() || repoId !== deps.getRepoId()) return;

      const files = mapGitCompareChanges(res.changes.entries);
      const prevPath = deps.getView().selectedCommitFilePath;
      const chosen =
        (prevPath
          ? files.find((f) => (f.newPath || f.oldPath) === prevPath)
          : undefined) ??
        files[0] ??
        null;
      const chosenPath = chosen ? chosen.newPath || chosen.oldPath : null;

      deps.patchView({
        selectedCommitFilePath: chosenPath,
        compareMode: {
          baseOid,
          targetOid,
          files,
          nextCursor: res.changes.nextCursor,
          loading: false,
          loadingMore: false,
          error: "",
        },
      });
      void deps.logDiff.loadPairFileDiff({
        baseOid,
        targetOid,
        leftLabel: baseOid.slice(0, 7),
        rightLabel: targetOid.slice(0, 7),
        file: chosen,
      });
    } catch (err) {
      if (!req.reportable()) return;
      deps.patchView({
        compareMode: {
          baseOid,
          targetOid,
          files: [],
          nextCursor: "",
          loading: false,
          loadingMore: false,
          error: errorMessage(err),
        },
      });
    }
  }

  async function loadMoreCompareFiles() {
    const repoId = deps.getRepoId();
    const worktreeId = deps.getEffectiveWorktreeId();
    const cmp = deps.getView().compareMode;
    if (!repoId || !cmp || cmp.loading || cmp.loadingMore || !cmp.nextCursor) {
      return;
    }

    const req = compareMoreReq.begin();
    deps.patchView({
      compareMode: { ...cmp, loadingMore: true, error: "" },
    });
    try {
      const res = await deps.logDiff.withContext(
        repoId,
        worktreeId,
        req.signal,
        (cid) =>
          gitCompare(
            repoId,
            {
              contextId: cid,
              worktreeId,
              baseOid: cmp.baseOid,
              targetOid: cmp.targetOid,
              cursor: cmp.nextCursor,
            },
            req.signal,
          ),
      );
      if (!req.current()) return;
      const latest = deps.getView().compareMode;
      if (
        !latest ||
        latest.baseOid !== cmp.baseOid ||
        latest.targetOid !== cmp.targetOid
      ) {
        return;
      }
      const extra = mapGitCompareChanges(res.changes.entries);
      deps.patchView({
        compareMode: {
          ...latest,
          files: [...latest.files, ...extra],
          nextCursor: res.changes.nextCursor,
          loadingMore: false,
        },
      });
    } catch (err) {
      if (!req.reportable()) return;
      const latest = deps.getView().compareMode;
      if (!latest) return;
      deps.patchView({
        compareMode: {
          ...latest,
          loadingMore: false,
          error: errorMessage(err),
        },
      });
    }
  }

  function swapCompareDirection() {
    const cmp = deps.getView().compareMode;
    if (!cmp) return;
    void startCompare(cmp.targetOid, cmp.baseOid);
  }

  function exitCompare() {
    compareReq.cancel();
    compareMoreReq.cancel();
    const view = deps.getView();
    if (!view.compareMode) return;
    deps.patchView({ compareMode: null });
    const oid = view.selectedCommitOid;
    deps.logDiff.syncForCommitDetail(oid ? view.commitDetails[oid] : undefined);
  }

  return {
    dispose,
    resetTarget,
    markCompareBase,
    startCompare,
    loadMoreCompareFiles,
    swapCompareDirection,
    exitCompare,
  };
}
