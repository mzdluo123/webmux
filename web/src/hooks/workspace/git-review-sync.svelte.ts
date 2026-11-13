import { SvelteMap } from "svelte/reactivity";
import type {
  GitCommitFile,
  GitReviewScope,
  GitWorkbenchViewState,
  GitWorkingChange,
} from "../../contracts/git-workbench-schemas";
import type {
  GitReviewArea,
  GitReviewFileState,
  GitReviewMarkIdentity,
  GitReviewsResponse,
} from "../../contracts/schemas";
import { toggleReviewedChange } from "../../domains/workspace/git-workbench-state";
import { errorMessage } from "../../platform/error";
import { gitPutReviews, gitReviews } from "../../platform/git-collab-api";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface GitReviewProgressSummary {
  total: number;
  reviewed: number;
  remaining: number;
}

function slotKey(area: string, path: string): string {
  return `${area}\x00${path}`;
}

function workingChangeArea(change: GitWorkingChange): GitReviewArea {
  return change.group === "staged" ? "staged" : "unstaged";
}

function parseStageIdentityKey(identityKey: string): {
  oldPath?: string;
  baseMode?: string;
  baseIdentity?: string;
  targetMode?: string;
  targetIdentity?: string;
} {
  if (!identityKey) return {};
  const parts = identityKey.split("|");
  if (parts.length < 10) return {};
  const [
    ,
    oldPath,
    ,
    ,
    ,
    baseOid,
    baseMode,
    targetOid,
    targetSha256,
    targetMode,
  ] = parts;
  return {
    ...(oldPath ? { oldPath } : {}),
    ...(baseMode ? { baseMode } : {}),
    ...(baseOid ? { baseIdentity: baseOid } : {}),
    ...(targetMode ? { targetMode } : {}),
    ...(targetOid || targetSha256
      ? { targetIdentity: targetOid || targetSha256 }
      : {}),
  };
}

export function findNextUnreviewedWorkingChange(
  changes: readonly GitWorkingChange[],
  activeChangeId: string | null,
  isReviewed: (change: GitWorkingChange) => boolean,
): GitWorkingChange | null {
  const candidates = changes.filter((c) => c.group !== "conflicted");
  if (candidates.length === 0) return null;
  const startIdx = candidates.findIndex((c) => c.id === activeChangeId);
  for (let offset = 1; offset <= candidates.length; offset++) {
    const idx =
      startIdx >= 0 ? (startIdx + offset) % candidates.length : offset - 1;
    const item = candidates[idx]!;
    if (!isReviewed(item)) return item;
  }
  return null;
}

export function findNextUnreviewedTaskFile(
  files: readonly GitCommitFile[],
  selectedPath: string | null,
  isReviewed: (path: string) => boolean,
): string | null {
  if (files.length === 0) return null;
  const startIdx = files.findIndex(
    (f) => (f.newPath || f.oldPath) === selectedPath,
  );
  for (let offset = 1; offset <= files.length; offset++) {
    const idx = startIdx >= 0 ? (startIdx + offset) % files.length : offset - 1;
    const path = files[idx]!.newPath || files[idx]!.oldPath;
    if (!isReviewed(path)) return path;
  }
  return null;
}

export function computeLocalReviewProgress(
  scope: GitReviewScope,
  changes: readonly GitWorkingChange[],
  taskFiles: readonly GitCommitFile[],
  reviewedPaths: readonly string[],
  isWorkingReviewed: (
    paths: readonly string[],
    change: GitWorkingChange,
  ) => boolean,
): GitReviewProgressSummary {
  if (scope === "task") {
    const reviewed = taskFiles.filter((f) => {
      const p = f.newPath || f.oldPath;
      return reviewedPaths.includes(`task:${p}`) || reviewedPaths.includes(p);
    }).length;
    return {
      total: taskFiles.length,
      reviewed,
      remaining: Math.max(0, taskFiles.length - reviewed),
    };
  }
  const candidates = changes.filter((c) => c.group !== "conflicted");
  const reviewed = candidates.filter((c) =>
    isWorkingReviewed(reviewedPaths, c),
  ).length;
  return {
    total: candidates.length,
    reviewed,
    remaining: Math.max(0, candidates.length - reviewed),
  };
}

export interface GitReviewSyncBindings {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  persistReviewedPaths: (paths: string[]) => void;
  onSelectChange: (changeId: string) => void;
  onSelectTaskFile: (path: string) => void;
  onStaleRejectedRefresh?: () => void;
}

export function createGitReviewSyncController(bindings: GitReviewSyncBindings) {
  let serverReadyUncommitted = $state(false);
  let serverReadyTask = $state(false);
  let mutating = $state(false);

  const uncommittedBySlot = new SvelteMap<string, GitReviewFileState>();
  const taskByPath = new SvelteMap<string, GitReviewFileState>();

  const uncomReq = new LatestRequest();
  const taskReq = new LatestRequest();
  const mutateReq = new LatestRequest();

  function dispose() {
    uncomReq.dispose();
    taskReq.dispose();
    mutateReq.dispose();
  }

  function resetTarget() {
    uncomReq.cancel();
    taskReq.cancel();
    mutateReq.cancel();
    serverReadyUncommitted = false;
    serverReadyTask = false;
    mutating = false;
    uncommittedBySlot.clear();
    taskByPath.clear();
  }

  function applyServerResponse(res: GitReviewsResponse) {
    if (res.scope === "uncommitted") {
      uncommittedBySlot.clear();
      for (const file of res.files) {
        uncommittedBySlot.set(slotKey(file.area, file.path), file);
      }
      serverReadyUncommitted = true;
    } else {
      taskByPath.clear();
      for (const file of res.files) {
        taskByPath.set(file.path, file);
      }
      serverReadyTask = true;
    }
    syncMirroredReviewedPaths();
  }

  function syncMirroredReviewedPaths() {
    const view = bindings.getView();
    const tokens: string[] = [];
    for (const change of view.workingChanges) {
      if (isUncommittedReviewed(change)) {
        const ver = change.identityKey?.trim();
        tokens.push(ver ? `${change.id}::v=${ver}` : change.id);
      }
    }
    for (const [path, file] of taskByPath) {
      if (file.reviewed) tokens.push(`task:${path}`);
    }
    bindings.patchView({ reviewedPaths: tokens });
    bindings.persistReviewedPaths(tokens);
  }

  async function refreshScope(
    scope: GitReviewScope,
    repoId = bindings.getRepoId(),
    wtId = bindings.getWorktreeId(),
    parentSignal?: AbortSignal,
  ) {
    if (!repoId || !wtId) return;
    const tracker = scope === "uncommitted" ? uncomReq : taskReq;
    const req = tracker.begin();
    const signal = parentSignal
      ? AbortSignal.any([req.signal, parentSignal])
      : req.signal;
    try {
      const res = await gitReviews(repoId, wtId, scope, signal);
      if (
        !req.current() ||
        parentSignal?.aborted ||
        repoId !== bindings.getRepoId() ||
        wtId !== bindings.getWorktreeId()
      ) {
        return;
      }
      applyServerResponse(res);
    } catch {
      /* Readonly or transient worktree state keeps existing local state */
    }
  }

  async function refreshAll(
    repoId = bindings.getRepoId(),
    wtId = bindings.getWorktreeId(),
    parentSignal?: AbortSignal,
  ) {
    if (!repoId || !wtId) return;
    await Promise.all([
      refreshScope("uncommitted", repoId, wtId, parentSignal),
      refreshScope("task", repoId, wtId, parentSignal),
    ]);
  }
  function isUncommittedReviewed(change: GitWorkingChange): boolean {
    if (change.group === "conflicted") return false;
    if (serverReadyUncommitted) {
      const item = uncommittedBySlot.get(
        slotKey(workingChangeArea(change), change.path),
      );
      if (!item || !item.reviewed) return false;
      const parsed = parseStageIdentityKey(change.identityKey);
      if (
        parsed.targetIdentity &&
        item.targetIdentity &&
        parsed.targetIdentity !== item.targetIdentity
      ) {
        return false;
      }
      if (
        parsed.baseIdentity &&
        item.baseIdentity &&
        parsed.baseIdentity !== item.baseIdentity
      ) {
        return false;
      }
      return true;
    }
    const paths = bindings.getView().reviewedPaths;
    const ver = change.identityKey?.trim();
    const token = ver ? `${change.id}::v=${ver}` : change.id;
    return paths.includes(token) || (!ver && paths.includes(change.path));
  }

  function isTaskFileReviewed(path: string): boolean {
    if (serverReadyTask) {
      return Boolean(taskByPath.get(path)?.reviewed);
    }
    const paths = bindings.getView().reviewedPaths;
    return paths.includes(`task:${path}`) || paths.includes(path);
  }

  function liveWorktreeIdentityForPath(path: string): string | undefined {
    const uncom = uncommittedBySlot.get(slotKey("unstaged", path));
    if (uncom?.targetIdentity) return uncom.targetIdentity;
    const task = taskByPath.get(path);
    if (task?.targetIdentity) return task.targetIdentity;
    return undefined;
  }

  async function toggleUncommittedReview(change: GitWorkingChange) {
    if (change.group === "conflicted") return;
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) {
      const next = toggleReviewedChange(
        bindings.getView().reviewedPaths,
        change,
      );
      bindings.patchView({ reviewedPaths: next });
      bindings.persistReviewedPaths(next);
      return;
    }

    const area = workingChangeArea(change);
    const live = uncommittedBySlot.get(slotKey(area, change.path));
    const parsed = parseStageIdentityKey(change.identityKey);
    const currentlyReviewed = isUncommittedReviewed(change);
    const item: GitReviewMarkIdentity = {
      scope: "uncommitted",
      area,
      path: change.path,
      ...(change.oldPath || parsed.oldPath || live?.oldPath
        ? { oldPath: change.oldPath || parsed.oldPath || live?.oldPath }
        : {}),
      ...((parsed.baseMode ?? live?.baseMode)
        ? { baseMode: parsed.baseMode ?? live?.baseMode }
        : {}),
      ...((parsed.baseIdentity ?? live?.baseIdentity)
        ? { baseIdentity: parsed.baseIdentity ?? live?.baseIdentity }
        : {}),
      ...((parsed.targetMode ?? live?.targetMode)
        ? { targetMode: parsed.targetMode ?? live?.targetMode }
        : {}),
      ...((parsed.targetIdentity ?? live?.targetIdentity)
        ? { targetIdentity: parsed.targetIdentity ?? live?.targetIdentity }
        : {}),
    };

    await executeReviewMutation(
      repoId,
      wtId,
      "uncommitted",
      currentlyReviewed,
      item,
      "更新审阅进度失败",
    );
  }

  async function executeReviewMutation(
    repoId: string,
    wtId: string,
    scope: GitReviewScope,
    currentlyReviewed: boolean,
    item: GitReviewMarkIdentity,
    errorPrefix: string,
  ) {
    const req = mutateReq.begin();
    mutating = true;
    try {
      const res = await gitPutReviews(
        repoId,
        wtId,
        {
          scope,
          action: currentlyReviewed ? "unmark" : "mark",
          item,
        },
        req.signal,
      );
      if (!req.current()) return;
      applyServerResponse(res);
      if (res.staleRejected) {
        bindings.patchView({
          statusBanner:
            res.rejectedReason ||
            "文件内容已在服务端更新，过期审阅标记未覆盖新版本，已同步最新状态。",
        });
        bindings.onStaleRejectedRefresh?.();
      }
    } catch (err) {
      if (req.reportable()) {
        bindings.patchView({
          statusBanner: `${errorPrefix}：${errorMessage(err)}`,
        });
      }
    } finally {
      if (req.current()) mutating = false;
    }
  }

  async function toggleTaskFileReview(path: string) {
    const repoId = bindings.getRepoId();
    const wtId = bindings.getWorktreeId();
    if (!repoId || !wtId) {
      const token = `task:${path}`;
      const prev = bindings.getView().reviewedPaths;
      const next = prev.includes(token)
        ? prev.filter((p) => p !== token && p !== path)
        : [...prev, token];
      bindings.patchView({ reviewedPaths: next });
      bindings.persistReviewedPaths(next);
      return;
    }

    let live = taskByPath.get(path);
    if (!live) {
      await refreshScope("task", repoId, wtId);
      live = taskByPath.get(path);
    }
    if (!live) {
      bindings.patchView({
        statusBanner: "当前任务变更列表已变化，请刷新后再标记已审阅。",
      });
      return;
    }

    const currentlyReviewed = isTaskFileReviewed(path);
    const item: GitReviewMarkIdentity = {
      scope: "task",
      area: "task",
      path: live.path,
      ...(live.oldPath ? { oldPath: live.oldPath } : {}),
      ...(live.baseMode ? { baseMode: live.baseMode } : {}),
      ...(live.baseIdentity ? { baseIdentity: live.baseIdentity } : {}),
      ...(live.targetMode ? { targetMode: live.targetMode } : {}),
      ...(live.targetIdentity ? { targetIdentity: live.targetIdentity } : {}),
    };

    await executeReviewMutation(
      repoId,
      wtId,
      "task",
      currentlyReviewed,
      item,
      "更新任务审阅进度失败",
    );
  }

  const uncommittedProgress = $derived.by<GitReviewProgressSummary>(() => {
    const candidates = bindings
      .getView()
      .workingChanges.filter((c) => c.group !== "conflicted");
    const reviewed = candidates.filter((c) => isUncommittedReviewed(c)).length;
    return {
      total: candidates.length,
      reviewed,
      remaining: Math.max(0, candidates.length - reviewed),
    };
  });

  const taskProgress = $derived.by<GitReviewProgressSummary>(() => {
    const files = bindings.getView().taskReview?.files ?? [];
    const reviewed = files.filter((f) =>
      isTaskFileReviewed(f.newPath || f.oldPath),
    ).length;
    return {
      total: files.length,
      reviewed,
      remaining: Math.max(0, files.length - reviewed),
    };
  });

  function jumpToNextUnreviewed(scope: GitReviewScope): string | null {
    const view = bindings.getView();
    if (scope === "task") {
      const files = view.taskReview?.files ?? [];
      const nextPath = findNextUnreviewedTaskFile(
        files,
        view.taskReview?.selectedPath ?? null,
        isTaskFileReviewed,
      );
      if (nextPath) bindings.onSelectTaskFile(nextPath);
      return nextPath;
    }
    const nextChange = findNextUnreviewedWorkingChange(
      view.workingChanges,
      view.activeChangeId,
      isUncommittedReviewed,
    );
    if (nextChange) bindings.onSelectChange(nextChange.id);
    return nextChange?.id ?? null;
  }

  return {
    get mutating() {
      return mutating;
    },
    get uncommittedProgress() {
      return uncommittedProgress;
    },
    get taskProgress() {
      return taskProgress;
    },
    dispose,
    resetTarget,
    refreshScope,
    refreshAll,
    isUncommittedReviewed,
    isTaskFileReviewed,
    liveWorktreeIdentityForPath,
    toggleUncommittedReview,
    toggleTaskFileReview,
    jumpToNextUnreviewed,
  };
}
