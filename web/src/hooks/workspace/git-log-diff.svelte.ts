import { gitCompare } from "../../platform/git-api";
import type {
  GitCommitDetail,
  GitCommitFile,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import {
  createGitSnapshotDiffReader,
  type GitDiffPanelReader,
} from "./git-snapshot-diff-reader.svelte";

export type { GitDiffPanelReader };

export interface GitLogDiffControllerDeps {
  getRepoId: () => string;
  getEffectiveWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  persistFileSelection: (path: string | null) => void;
}

export function createGitLogDiffController(deps: GitLogDiffControllerDeps) {
  const reader = createGitSnapshotDiffReader({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getEffectiveWorktreeId,
    onStatusBanner: (statusBanner) => deps.patchView({ statusBanner }),
  });

  async function loadPairFileDiff(options: {
    baseOid: string;
    targetOid: string;
    leftLabel: string;
    rightLabel: string;
    file: GitCommitFile | null;
  }) {
    if (!options.targetOid || !options.file) {
      reader.clearDiff();
      return;
    }
    const filePath = options.file.newPath || options.file.oldPath;
    await reader.loadFetchedDiff({
      baseOid: options.baseOid,
      targetOid: options.targetOid,
      leftLabel: options.leftLabel,
      rightLabel: options.rightLabel,
      file: options.file,
      missingReason: "未找到该路径对应的历史对象快照。",
      fetchDiff: async (cid, signal) => {
        const res = await gitCompare(
          deps.getRepoId(),
          {
            contextId: cid,
            worktreeId: deps.getEffectiveWorktreeId(),
            baseOid: options.baseOid || undefined,
            targetOid: options.targetOid,
            path: options.file?.changeId ? undefined : filePath,
            changeId: options.file?.changeId,
          },
          signal,
        );
        return res.diff ?? null;
      },
    });
  }

  function syncForCommitDetail(
    detail: GitCommitDetail | undefined,
    preferredPath?: string | null,
  ) {
    const view = deps.getView();
    if (view.compareMode) return;
    if (!detail) {
      reader.clearDiff();
      return;
    }
    const parentKey =
      view.selectedParentByCommit[detail.oid] ??
      detail.selectedParent ??
      detail.parents[0]?.oid ??
      "";
    const files = detail.filesByParent[parentKey] ?? [];
    const targetPath = preferredPath ?? view.selectedCommitFilePath;
    const chosen =
      (targetPath
        ? files.find((f) => (f.newPath || f.oldPath) === targetPath)
        : undefined) ??
      files[0] ??
      null;
    const chosenPath = chosen ? chosen.newPath || chosen.oldPath : null;
    if (chosenPath !== view.selectedCommitFilePath) {
      deps.patchView({ selectedCommitFilePath: chosenPath });
      deps.persistFileSelection(chosenPath);
    }
    void loadPairFileDiff({
      baseOid: parentKey,
      targetOid: detail.oid,
      leftLabel: parentKey ? parentKey.slice(0, 7) : "空树",
      rightLabel: detail.shortOid || detail.oid.slice(0, 7),
      file: chosen,
    });
  }

  function selectActiveFile(path: string) {
    const view = deps.getView();
    deps.patchView({ selectedCommitFilePath: path });
    deps.persistFileSelection(path);

    if (view.compareMode) {
      const file =
        view.compareMode.files.find((f) => (f.newPath || f.oldPath) === path) ??
        null;
      void loadPairFileDiff({
        baseOid: view.compareMode.baseOid,
        targetOid: view.compareMode.targetOid,
        leftLabel: view.compareMode.baseOid.slice(0, 7),
        rightLabel: view.compareMode.targetOid.slice(0, 7),
        file,
      });
      return;
    }

    const oid = view.selectedCommitOid;
    syncForCommitDetail(oid ? view.commitDetails[oid] : undefined, path);
  }

  return {
    get state() {
      return reader.state;
    },
    dispose: reader.dispose,
    resetTarget: reader.resetTarget,
    clearDiff: reader.clearDiff,
    seedContextId: reader.seedContextId,
    withContext: reader.withContext,
    loadPairFileDiff,
    syncForCommitDetail,
    selectActiveFile,
    retryActiveDiff() {
      const view = deps.getView();
      if (view.compareMode) {
        selectActiveFile(view.selectedCommitFilePath ?? "");
      } else {
        const oid = view.selectedCommitOid;
        syncForCommitDetail(oid ? view.commitDetails[oid] : undefined);
      }
    },
    openSnapshotSide: reader.openSnapshotSide,
    closeSnapshotSide: reader.closeSnapshotSide,
    downloadSnapshotSide: reader.downloadSnapshotSide,
  };
}
export type GitLogDiffController = ReturnType<
  typeof createGitLogDiffController
>;
