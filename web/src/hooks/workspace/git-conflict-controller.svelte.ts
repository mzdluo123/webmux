import { APIError } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import {
  gitConflictDetail,
  gitResolveConflict,
  gitSaveConflict,
} from "../../platform/git-conflict-api";
import {
  GitConflictSchema,
  type GitConflictFileDetail,
  type GitInProgressOperation,
} from "../../contracts/schemas";
import {
  applyAllNonConflictingSegments,
  applyConflictChunkChoice,
  createInitialConflictDraftState,
  markAllUnresolvedAsManual,
  summarizeConflictDraftProgress,
  toggleNonConflictingSegment,
  type GitBaseCompareMode,
  type GitConflictChunkChoice,
  type GitConflictDraftState,
} from "../../domains/workspace/git-conflict-model";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface GitConflictControllerDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getInProgress: () => GitInProgressOperation | null;
  onInProgressUpdated: (next: GitInProgressOperation) => void;
  onRefreshStaging: () => Promise<void> | void;
  onStatusBanner: (banner: string) => void;
}

function parseConflictFailure(err: unknown): {
  stale: boolean;
  message: string;
} {
  if (err instanceof APIError && err.status === 409) {
    const parsed = GitConflictSchema.safeParse(err.body);
    if (parsed.success) {
      return {
        stale:
          parsed.data.reason === "stale" ||
          parsed.data.reason === "context_mismatch",
        message: parsed.data.error,
      };
    }
  }
  return { stale: false, message: errorMessage(err) };
}

export function createGitConflictController(deps: GitConflictControllerDeps) {
  let open = $state(false);
  let activePath = $state<string | null>(null);
  let detail = $state<GitConflictFileDetail | null>(null);
  let draft = $state<GitConflictDraftState | null>(null);
  let baseCompareMode = $state<GitBaseCompareMode>("off");
  let activeConflictIndex = $state(0);
  let loading = $state(false);
  let saving = $state(false);
  let resolving = $state(false);
  let error = $state("");
  let stale = $state(false);

  const loadReq = new LatestRequest();
  let disposed = false;

  const progress = $derived.by(() =>
    detail && draft ? summarizeConflictDraftProgress(detail, draft) : null,
  );

  const conflictSegments = $derived.by(() =>
    detail ? detail.segments.filter((s) => s.kind === "conflict") : [],
  );

  function dispose() {
    disposed = true;
    loadReq.dispose();
  }

  function resetTarget() {
    loadReq.cancel();
    open = false;
    activePath = null;
    detail = null;
    draft = null;
    baseCompareMode = "off";
    activeConflictIndex = 0;
    loading = false;
    saving = false;
    resolving = false;
    error = "";
    stale = false;
  }

  async function selectFile(path: string) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId || !path || disposed) return;

    const req = loadReq.begin();
    activePath = path;
    loading = true;
    error = "";
    stale = false;
    activeConflictIndex = 0;
    try {
      const res = await gitConflictDetail(repoId, wtId, path, req.signal);
      if (
        !req.current() ||
        disposed ||
        repoId !== deps.getRepoId() ||
        wtId !== deps.getWorktreeId()
      ) {
        return;
      }
      detail = res;
      draft = createInitialConflictDraftState(res);
    } catch (err) {
      if (req.reportable()) {
        const parsed = parseConflictFailure(err);
        error = parsed.message;
        stale = parsed.stale;
        detail = null;
        draft = null;
      }
    } finally {
      if (req.current()) loading = false;
    }
  }

  async function openResolver(preferredPath?: string) {
    open = true;
    error = "";
    stale = false;
    const targetPath =
      preferredPath ||
      activePath ||
      deps.getInProgress()?.conflictedFiles[0]?.path ||
      null;
    if (targetPath) {
      await selectFile(targetPath);
    } else {
      activePath = null;
      detail = null;
      draft = null;
    }
  }

  function closeResolver() {
    if (saving || resolving) return;
    loadReq.cancel();
    open = false;
    error = "";
    stale = false;
  }

  function chooseChunk(segmentId: string, choice: GitConflictChunkChoice) {
    if (!detail || !draft) return;
    draft = applyConflictChunkChoice(detail, draft, segmentId, choice);
  }

  function toggleNonConflicting(segmentId: string, applied?: boolean) {
    if (!detail || !draft) return;
    draft = toggleNonConflictingSegment(detail, draft, segmentId, applied);
  }

  function applyAllNonConflicting() {
    if (!detail || !draft) return;
    draft = applyAllNonConflictingSegments(detail, draft);
  }

  function confirmManualEditsResolved() {
    if (!draft) return;
    draft = markAllUnresolvedAsManual(draft);
  }

  function setResultText(nextText: string) {
    if (!draft) return;
    draft = {
      ...draft,
      manualEdited: true,
      resultText: nextText,
    };
  }

  function setBaseCompareMode(mode: GitBaseCompareMode) {
    baseCompareMode = mode;
  }

  function navigateConflict(delta: -1 | 1) {
    const total = conflictSegments.length;
    if (total === 0) return;
    activeConflictIndex = (activeConflictIndex + delta + total) % total;
  }

  async function saveResult() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId || !detail || !draft || saving || resolving) return;

    saving = true;
    error = "";
    stale = false;
    const textToSave = draft.resultText;
    try {
      const res = await gitSaveConflict(repoId, wtId, {
        identity: detail.identity,
        resultText: textToSave,
      });
      detail = { ...detail, identity: res.identity };
      draft = { ...draft, savedResultText: textToSave };
      deps.onStatusBanner(
        `已保存合并结果到工作树文件 ${res.path}（尚未写入暂存区，核对无误后请点击「标记已解决」）`,
      );
      void deps.onRefreshStaging();
    } catch (err) {
      const parsed = parseConflictFailure(err);
      error = parsed.message;
      stale = parsed.stale;
    } finally {
      saving = false;
    }
  }

  async function handleAfterResolved(
    resolvedPath: string,
    nextInProgress: GitInProgressOperation,
  ) {
    deps.onInProgressUpdated(nextInProgress);
    await deps.onRefreshStaging();
    const remaining = nextInProgress.conflictedFiles;
    if (remaining.length > 0) {
      const nextTarget =
        remaining.find((f) => f.path !== resolvedPath) ?? remaining[0]!;
      deps.onStatusBanner(
        `已标记 ${resolvedPath} 解决，剩余 ${remaining.length} 个冲突文件`,
      );
      await selectFile(nextTarget.path);
    } else {
      detail = null;
      draft = null;
      activePath = null;
      deps.onStatusBanner(
        `已标记 ${resolvedPath} 解决；全部冲突已清零，可点击「继续操作」完成流程`,
      );
    }
  }

  async function executeResolution(
    resolution: "text" | "choose_left" | "choose_right" | "delete",
    resultText?: string,
  ) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId || !detail || saving || resolving) return;

    resolving = true;
    error = "";
    stale = false;
    try {
      const res = await gitResolveConflict(repoId, wtId, {
        identity: detail.identity,
        resolution,
        resultText,
      });
      await handleAfterResolved(res.path, res.inProgress);
    } catch (err) {
      const parsed = parseConflictFailure(err);
      error = parsed.message;
      stale = parsed.stale;
    } finally {
      resolving = false;
    }
  }

  async function markAsResolved() {
    if (!draft || !progress?.canMarkResolved) return;
    await executeResolution("text", draft.resultText);
  }

  async function resolveVariant(
    resolution: "choose_left" | "choose_right" | "delete",
  ) {
    await executeResolution(resolution);
  }

  return {
    get open() {
      return open;
    },
    get activePath() {
      return activePath;
    },
    get detail() {
      return detail;
    },
    get draft() {
      return draft;
    },
    get progress() {
      return progress;
    },
    get conflictSegments() {
      return conflictSegments;
    },
    get baseCompareMode() {
      return baseCompareMode;
    },
    get activeConflictIndex() {
      return activeConflictIndex;
    },
    get loading() {
      return loading;
    },
    get saving() {
      return saving;
    },
    get resolving() {
      return resolving;
    },
    get error() {
      return error;
    },
    get stale() {
      return stale;
    },
    dispose,
    resetTarget,
    openResolver,
    closeResolver,
    selectFile,
    reloadCurrent: () =>
      activePath ? selectFile(activePath) : Promise.resolve(),
    chooseChunk,
    toggleNonConflicting,
    applyAllNonConflicting,
    confirmManualEditsResolved,
    setResultText,
    setBaseCompareMode,
    navigateConflict,
    saveResult,
    markAsResolved,
    resolveVariant,
  };
}

export type GitConflictController = ReturnType<
  typeof createGitConflictController
>;
