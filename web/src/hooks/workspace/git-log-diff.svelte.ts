import type { FileDiffMetadata } from "@pierre/diffs";
import { APIError } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import {
  gitCompare,
  gitContext,
  gitSnapshotFile,
  gitSnapshotRaw,
} from "../../platform/git-api";
import type {
  GitCommitDetail,
  GitCommitFile,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import type {
  GitDiff,
  GitFile,
  GitSnapshotSide,
} from "../../contracts/schemas";
import { resolveCalculatedDiff } from "../../domains/code/code-diff-client";
import { LatestRequest } from "../shared/remote-request.svelte";

type GitLogDiffPhase =
  | "idle"
  | "loading"
  | "ready"
  | "binary"
  | "limited"
  | "error";

interface GitSnapshotSidePreview {
  side: GitSnapshotSide;
  loading: boolean;
  file: GitFile | null;
  error: string;
}

export interface GitLogDiffViewState {
  phase: GitLogDiffPhase;
  leftLabel: string;
  rightLabel: string;
  baseOid: string;
  targetOid: string;
  path: string;
  oldPath: string;
  status: GitCommitFile["status"] | "";
  snapshotId: string;
  fileDiff: FileDiffMetadata | null;
  rawDiff: GitDiff | null;
  reason: string;
  snapshotSideDoc: GitSnapshotSidePreview | null;
  downloadingSide: GitSnapshotSide | null;
}

function emptyLogDiffState(): GitLogDiffViewState {
  return {
    phase: "idle",
    leftLabel: "",
    rightLabel: "",
    baseOid: "",
    targetOid: "",
    path: "",
    oldPath: "",
    status: "",
    snapshotId: "",
    fileDiff: null,
    rawDiff: null,
    reason: "",
    snapshotSideDoc: null,
    downloadingSide: null,
  };
}

function isBinaryDiff(file: GitCommitFile, diff: GitDiff): boolean {
  const binaryKinds = new Set(["binary", "image", "pdf"]);
  return (
    file.binary ||
    diff.kind === "image" ||
    binaryKinds.has(diff.old.kind) ||
    binaryKinds.has(diff.new.kind)
  );
}

export interface GitLogDiffControllerDeps {
  getRepoId: () => string;
  getEffectiveWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  persistFileSelection: (path: string | null) => void;
}

export function createGitLogDiffController(deps: GitLogDiffControllerDeps) {
  let state = $state.raw<GitLogDiffViewState>(emptyLogDiffState());
  let cachedContext: {
    repoId: string;
    worktreeId?: string;
    contextId: string;
  } | null = null;

  const diffReq = new LatestRequest();
  const sideReq = new LatestRequest();

  function dispose() {
    diffReq.dispose();
    sideReq.dispose();
  }

  function resetTarget() {
    diffReq.cancel();
    sideReq.cancel();
    cachedContext = null;
    state = emptyLogDiffState();
  }

  function clearDiff() {
    diffReq.cancel();
    sideReq.cancel();
    state = emptyLogDiffState();
  }

  function seedContextId(
    repoId: string,
    worktreeId: string | undefined,
    contextId: string,
  ) {
    if (!repoId || !contextId) return;
    cachedContext = { repoId, worktreeId, contextId };
  }

  async function ensureContextId(
    repoId: string,
    worktreeId: string | undefined,
    signal: AbortSignal,
    force = false,
  ): Promise<string> {
    if (
      !force &&
      cachedContext &&
      cachedContext.repoId === repoId &&
      cachedContext.worktreeId === worktreeId &&
      cachedContext.contextId
    ) {
      return cachedContext.contextId;
    }
    const ctx = await gitContext(repoId, worktreeId, signal);
    cachedContext = { repoId, worktreeId, contextId: ctx.contextId };
    return ctx.contextId;
  }

  async function withContext<T>(
    repoId: string,
    worktreeId: string | undefined,
    signal: AbortSignal,
    run: (contextId: string) => Promise<T>,
  ): Promise<T> {
    const firstId = await ensureContextId(repoId, worktreeId, signal, false);
    try {
      return await run(firstId);
    } catch (err) {
      if (
        !signal.aborted &&
        err instanceof APIError &&
        (err.status === 409 || err.status === 410)
      ) {
        const nextId = await ensureContextId(repoId, worktreeId, signal, true);
        return await run(nextId);
      }
      throw err;
    }
  }

  async function loadPairFileDiff(options: {
    baseOid: string;
    targetOid: string;
    leftLabel: string;
    rightLabel: string;
    file: GitCommitFile | null;
  }) {
    const repoId = deps.getRepoId();
    const worktreeId = deps.getEffectiveWorktreeId();
    sideReq.cancel();
    if (!repoId || !options.targetOid || !options.file) {
      diffReq.cancel();
      state = emptyLogDiffState();
      return;
    }

    const { file, baseOid, targetOid, leftLabel, rightLabel } = options;
    const filePath = file.newPath || file.oldPath;
    const req = diffReq.begin();
    state = {
      phase: "loading",
      leftLabel,
      rightLabel,
      baseOid,
      targetOid,
      path: filePath,
      oldPath: file.oldPath,
      status: file.status,
      snapshotId: "",
      fileDiff: null,
      rawDiff: null,
      reason: "",
      snapshotSideDoc: null,
      downloadingSide: null,
    };

    try {
      const res = await withContext(repoId, worktreeId, req.signal, (cid) =>
        gitCompare(
          repoId,
          {
            contextId: cid,
            worktreeId,
            baseOid: baseOid || undefined,
            targetOid,
            path: file.changeId ? undefined : filePath,
            changeId: file.changeId,
          },
          req.signal,
        ),
      );
      if (!req.current() || repoId !== deps.getRepoId()) return;

      const diff = res.diff ?? null;
      if (!diff) {
        state = {
          ...state,
          phase: "limited",
          reason: "未找到该路径对应的历史对象快照。",
        };
        return;
      }

      if (isBinaryDiff(file, diff)) {
        state = {
          ...state,
          phase: "binary",
          snapshotId: diff.snapshotId,
          rawDiff: diff,
          reason: diff.reason || "二进制文件不直接展开文本行级差异。",
        };
        return;
      }

      if (diff.kind !== "text") {
        state = {
          ...state,
          phase: "limited",
          snapshotId: diff.snapshotId,
          rawDiff: diff,
          reason:
            diff.reason ||
            "该文件过大或仅包含元数据变更，可查看两侧完整文件或下载原始对象。",
        };
        return;
      }

      const calculated = await resolveCalculatedDiff(diff, req.signal);
      if (!req.current() || repoId !== deps.getRepoId()) return;

      if (calculated.calculation === "ready" && calculated.fileDiff) {
        state = {
          ...state,
          phase: "ready",
          snapshotId: diff.snapshotId,
          rawDiff: diff,
          fileDiff: calculated.fileDiff,
          reason: "",
        };
      } else {
        state = {
          ...state,
          phase: calculated.calculation === "error" ? "error" : "limited",
          snapshotId: diff.snapshotId,
          rawDiff: diff,
          reason: calculated.calculationReason,
        };
      }
    } catch (err) {
      if (!req.reportable()) return;
      state = { ...state, phase: "error", reason: errorMessage(err) };
    }
  }

  function syncForCommitDetail(
    detail: GitCommitDetail | undefined,
    preferredPath?: string | null,
  ) {
    const view = deps.getView();
    if (view.compareMode) return;
    if (!detail) {
      clearDiff();
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

  async function openSnapshotSide(side: GitSnapshotSide) {
    const repoId = deps.getRepoId();
    const worktreeId = deps.getEffectiveWorktreeId();
    const snapshotId = state.snapshotId;
    if (!repoId || !snapshotId) return;

    const req = sideReq.begin();
    state = {
      ...state,
      snapshotSideDoc: { side, loading: true, file: null, error: "" },
    };
    try {
      const file = await withContext(repoId, worktreeId, req.signal, (cid) =>
        gitSnapshotFile(
          repoId,
          snapshotId,
          { side, contextId: cid, worktreeId },
          req.signal,
        ),
      );
      if (!req.current() || state.snapshotId !== snapshotId) return;
      state = {
        ...state,
        snapshotSideDoc: { side, loading: false, file, error: "" },
      };
    } catch (err) {
      if (!req.reportable()) return;
      state = {
        ...state,
        snapshotSideDoc: {
          side,
          loading: false,
          file: null,
          error: errorMessage(err),
        },
      };
    }
  }

  function closeSnapshotSide() {
    sideReq.cancel();
    state = { ...state, snapshotSideDoc: null };
  }

  async function downloadSnapshotSide(side: GitSnapshotSide) {
    const repoId = deps.getRepoId();
    const worktreeId = deps.getEffectiveWorktreeId();
    const snapshotId = state.snapshotId;
    if (!repoId || !snapshotId || state.downloadingSide) return;

    state = { ...state, downloadingSide: side };
    const ctrl = new AbortController();
    try {
      const blob = await withContext(repoId, worktreeId, ctrl.signal, (cid) =>
        gitSnapshotRaw(
          repoId,
          snapshotId,
          { side, contextId: cid, worktreeId, download: true },
          ctrl.signal,
        ),
      );
      const baseName =
        (side === "before" ? state.oldPath || state.path : state.path)
          .split("/")
          .pop() || `snapshot-${side}`;
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = baseName;
      anchor.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (err) {
      deps.patchView({ statusBanner: `快照下载失败：${errorMessage(err)}` });
    } finally {
      if (state.snapshotId === snapshotId) {
        state = { ...state, downloadingSide: null };
      }
    }
  }

  return {
    get state() {
      return state;
    },
    dispose,
    resetTarget,
    clearDiff,
    seedContextId,
    withContext,
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
    openSnapshotSide,
    closeSnapshotSide,
    downloadSnapshotSide,
  };
}
export type GitLogDiffController = ReturnType<
  typeof createGitLogDiffController
>;
