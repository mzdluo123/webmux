import { m } from "$lib/paraglide/messages.js";
import { SvelteMap } from "svelte/reactivity";
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
  cloneConflictDraftState,
  createInitialConflictDraftState,
  markAllUnresolvedAsManual,
  normalizeConflictLineEndings,
  serializeConflictIdentity,
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
  const actionReq = new LatestRequest();
  const draftCache = new SvelteMap<
    string,
    { identityKey: string; draft: GitConflictDraftState }
  >();
  let disposed = false;

  const progress = $derived.by(() =>
    detail && draft ? summarizeConflictDraftProgress(detail, draft) : null,
  );

  const conflictSegments = $derived.by(() =>
    detail ? detail.segments.filter((s) => s.kind === "conflict") : [],
  );

  function matchesTarget(
    repoId: string,
    wtId: string | undefined,
    targetPath: string,
  ): boolean {
    return (
      !disposed &&
      repoId === deps.getRepoId() &&
      wtId === deps.getWorktreeId() &&
      activePath === targetPath
    );
  }

  function persistActiveDraft() {
    if (!detail || !draft) return;
    draftCache.set(detail.path, {
      identityKey: serializeConflictIdentity(detail.identity),
      draft: cloneConflictDraftState(draft),
    });
  }

  function reportFailureIfActive(
    err: unknown,
    reportable: boolean,
    repoId: string,
    wtId: string | undefined,
    targetPath: string,
  ): boolean {
    if (!reportable || !matchesTarget(repoId, wtId, targetPath)) return false;
    const parsed = parseConflictFailure(err);
    error = parsed.message;
    stale = parsed.stale;
    return true;
  }

  function dispose() {
    disposed = true;
    loadReq.dispose();
    actionReq.dispose();
    draftCache.clear();
  }

  function resetTarget() {
    loadReq.cancel();
    actionReq.cancel();
    draftCache.clear();
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

  async function selectFile(path: string, options?: { forceReload?: boolean }) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId || !path || disposed || saving || resolving) return;

    const forceReload = Boolean(options?.forceReload);
    if (detail && draft && (!forceReload || detail.path !== path)) {
      persistActiveDraft();
    }
    if (forceReload) {
      draftCache.delete(path);
    }
    if (detail && detail.path !== path) {
      detail = null;
      draft = null;
    }

    const req = loadReq.begin();
    activePath = path;
    loading = true;
    error = "";
    stale = false;
    activeConflictIndex = 0;
    try {
      const res = await gitConflictDetail(repoId, wtId, path, req.signal);
      if (!req.current() || !matchesTarget(repoId, wtId, path)) return;
      detail = res;
      const identityKey = serializeConflictIdentity(res.identity);
      const cached = !forceReload ? draftCache.get(res.path) : undefined;
      if (cached && cached.identityKey === identityKey) {
        draft = cloneConflictDraftState(cached.draft);
      } else {
        if (cached) draftCache.delete(res.path);
        draft = createInitialConflictDraftState(res);
      }
    } catch (err) {
      if (reportFailureIfActive(err, req.reportable(), repoId, wtId, path)) {
        detail = null;
        draft = null;
      }
    } finally {
      if (req.current()) loading = false;
    }
  }

  async function openResolver(preferredPath?: string) {
    open = true;
    if (saving || resolving) return;
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
    persistActiveDraft();
    loadReq.cancel();
    loading = false;
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
    const targetPath = activePath;
    if (
      !repoId ||
      !wtId ||
      !targetPath ||
      !detail ||
      !draft ||
      detail.path !== targetPath ||
      saving ||
      resolving ||
      disposed
    ) {
      return;
    }

    const req = actionReq.begin();
    saving = true;
    error = "";
    stale = false;
    const identityToSave = detail.identity;
    const textToSave = normalizeConflictLineEndings(draft.resultText, detail);
    try {
      const res = await gitSaveConflict(
        repoId,
        wtId,
        { identity: identityToSave, resultText: textToSave },
        req.signal,
      );
      if (
        !req.current() ||
        !matchesTarget(repoId, wtId, targetPath) ||
        detail === null ||
        draft === null ||
        detail.path !== targetPath
      ) {
        return;
      }
      detail = { ...detail, identity: res.identity };
      draft = { ...draft, resultText: textToSave, savedResultText: textToSave };
      persistActiveDraft();
      deps.onStatusBanner(m.s7_conflict_saved_to_wt({ path: res.path }));
      void deps.onRefreshStaging();
    } catch (err) {
      reportFailureIfActive(err, req.reportable(), repoId, wtId, targetPath);
    } finally {
      if (req.current()) saving = false;
    }
  }

  async function executeResolution(
    resolution: "text" | "choose_left" | "choose_right" | "delete",
    resultText?: string,
  ) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const targetPath = activePath;
    if (
      !repoId ||
      !wtId ||
      !targetPath ||
      !detail ||
      detail.path !== targetPath ||
      saving ||
      resolving ||
      disposed
    ) {
      return;
    }

    const req = actionReq.begin();
    resolving = true;
    error = "";
    stale = false;
    const identityToResolve = detail.identity;

    try {
      const res = await gitResolveConflict(
        repoId,
        wtId,
        { identity: identityToResolve, resolution, resultText },
        req.signal,
      );
      if (!req.current() || !matchesTarget(repoId, wtId, targetPath)) return;

      draftCache.delete(targetPath);
      draftCache.delete(res.path);
      deps.onInProgressUpdated(res.inProgress);
      await deps.onRefreshStaging();
      if (!req.current() || !matchesTarget(repoId, wtId, targetPath)) return;

      const remaining = res.inProgress.conflictedFiles;
      if (remaining.length > 0) {
        const nextTarget =
          remaining.find((f) => f.path !== res.path) ?? remaining[0]!;
        detail = null;
        draft = null;
        resolving = false;
        deps.onStatusBanner(
          m.s7_conflict_resolved_remaining({
            path: res.path,
            count: String(remaining.length),
          }),
        );
        await selectFile(nextTarget.path);
      } else {
        detail = null;
        draft = null;
        activePath = null;
        deps.onStatusBanner(
          m.s7_conflict_resolved_all_clear({ path: res.path }),
        );
      }
    } catch (err) {
      reportFailureIfActive(err, req.reportable(), repoId, wtId, targetPath);
    } finally {
      if (req.current()) resolving = false;
    }
  }

  async function markAsResolved() {
    if (!detail || !draft || !progress?.canMarkResolved) return;
    await executeResolution(
      "text",
      normalizeConflictLineEndings(draft.resultText, detail),
    );
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
      activePath
        ? selectFile(activePath, { forceReload: true })
        : Promise.resolve(),
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
