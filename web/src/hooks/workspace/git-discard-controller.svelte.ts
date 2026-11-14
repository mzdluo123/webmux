import { gitConflictReason } from "../../platform/git-api";
import { gitDiscard, gitDiscardPreview } from "../../platform/git-v3-api";
import type {
  GitDiscardPreview,
  GitDiscardPreviewRequest,
  GitLineSelection,
  GitStageFileIdentity,
} from "../../contracts/schemas";
import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import { generateGitClientRequestId } from "../../domains/workspace/git-operations-model";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import { parseGitManageError } from "./git-remotes-controller.svelte";

export interface GitDiscardControllerDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  getEntryIdentity: (changeId: string) => GitStageFileIdentity | undefined;
  operations?: GitOperationsController;
  onAfterDiscard: () => Promise<void>;
}

interface DiscardDialogState {
  open: boolean;
  lastChangeIds: readonly string[];
  lastRequest: GitDiscardPreviewRequest | null;
  preview: GitDiscardPreview | null;
  loadingPreview: boolean;
  submitting: boolean;
  staleRejected: boolean;
  acknowledgeSessions: boolean;
  dialogError: string;
}

function emptyDiscardDialogState(): DiscardDialogState {
  return {
    open: false,
    lastChangeIds: [],
    lastRequest: null,
    preview: null,
    loadingPreview: false,
    submitting: false,
    staleRejected: false,
    acknowledgeSessions: false,
    dialogError: "",
  };
}

export function createGitDiscardController(deps: GitDiscardControllerDeps) {
  let dialog = $state<DiscardDialogState>(emptyDiscardDialogState());
  const previewReq = new LatestRequest();
  let disposed = false;

  function dispose() {
    disposed = true;
    previewReq.dispose();
  }

  function resetTarget() {
    previewReq.cancel();
    dialog = emptyDiscardDialogState();
  }

  async function loadPreview(request: GitDiscardPreviewRequest) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId) {
      dialog.dialogError = "当前处于只读仓库视图，请先选择工作树再丢弃更改。";
      return;
    }
    dialog.lastRequest = request;
    dialog.loadingPreview = true;
    dialog.staleRejected = false;
    dialog.dialogError = "";
    const req = previewReq.begin();
    try {
      const res = await gitDiscardPreview(repoId, wtId, request, req.signal);
      if (!req.current() || disposed) return;
      dialog.preview = res;
    } catch (err) {
      if (req.reportable()) {
        dialog.staleRejected = gitConflictReason(err) === "stale";
        dialog.dialogError = parseGitManageError(err);
      }
    } finally {
      if (req.current()) dialog.loadingPreview = false;
    }
  }

  function beginOpen(changeIds: readonly string[]) {
    dialog.open = true;
    dialog.lastChangeIds = [...changeIds];
    dialog.preview = null;
    dialog.acknowledgeSessions = false;
  }

  function collectUnstagedIdentities(
    changeIds: readonly string[],
  ): GitStageFileIdentity[] {
    const identities: GitStageFileIdentity[] = [];
    for (const id of changeIds) {
      const ident = deps.getEntryIdentity(id);
      if (ident && ident.area === "unstaged") identities.push(ident);
    }
    return identities;
  }

  function openDiscardFiles(
    changeIds: readonly string[],
    paths: readonly string[],
  ) {
    if (paths.length === 0) return;
    const identities = collectUnstagedIdentities(changeIds);
    beginOpen(changeIds);
    void loadPreview({
      scope: "files",
      paths: [...paths],
      ...(identities.length > 0 ? { identities } : {}),
    });
  }

  function openDiscardHunk(changeId: string, hunkId: string) {
    const identity = deps.getEntryIdentity(changeId);
    if (!identity || identity.area !== "unstaged") {
      deps.patchView({
        statusBanner:
          "仅未暂存的已跟踪文件支持丢弃此变更块（已暂存区请使用取消暂存）。",
      });
      return;
    }
    beginOpen([changeId]);
    void loadPreview({
      scope: "hunks",
      identity,
      hunkIds: [hunkId],
    });
  }

  function openDiscardLines(
    changeId: string,
    lineSelections: readonly GitLineSelection[],
  ) {
    const identity = deps.getEntryIdentity(changeId);
    if (
      !identity ||
      identity.area !== "unstaged" ||
      lineSelections.length === 0
    ) {
      return;
    }
    beginOpen([changeId]);
    void loadPreview({
      scope: "lines",
      identity,
      lineSelections: lineSelections.map((s) => ({
        hunkId: s.hunkId,
        lineIndices: [...s.lineIndices],
      })),
    });
  }

  async function refreshPreviewAfterStale() {
    await deps.onAfterDiscard();
    const prev = dialog.lastRequest;
    if (!prev) return;
    if (prev.scope === "files") {
      const identities = collectUnstagedIdentities(dialog.lastChangeIds);
      await loadPreview({
        scope: "files",
        paths: prev.paths,
        ...(identities.length > 0 ? { identities } : {}),
      });
      return;
    }
    const freshIdentity = dialog.lastChangeIds[0]
      ? deps.getEntryIdentity(dialog.lastChangeIds[0])
      : undefined;
    if (!freshIdentity || freshIdentity.area !== "unstaged") {
      dialog.staleRejected = false;
      dialog.dialogError =
        "目标文件已无可丢弃的未暂存更改，请关闭弹窗核对最新工作区状态。";
      return;
    }
    await loadPreview({ ...prev, identity: freshIdentity });
  }

  async function confirmDiscard() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const cur = dialog.preview;
    if (!repoId || !wtId || !cur?.confirmationId || dialog.submitting) return;
    dialog.submitting = true;
    dialog.dialogError = "";
    dialog.staleRejected = false;
    const clientRequestId = generateGitClientRequestId("discard");
    deps.operations?.setPhase(
      "running",
      "discard",
      "正在丢弃所选更改并保存恢复快照…",
      { clientRequestId },
    );
    try {
      const res = await gitDiscard(repoId, wtId, {
        confirmationId: cur.confirmationId,
        clientRequestId,
        acknowledgeSessions: dialog.acknowledgeSessions,
      });
      dialog.open = false;
      dialog.preview = null;
      dialog.lastRequest = null;
      const recoverySuffix = res.recoveryOid
        ? `（恢复快照 ${res.recoveryOid.slice(0, 7)}：${res.recoveryCommand ?? ""}）`
        : "";
      deps.patchView({
        statusBanner: `${res.summary}${recoverySuffix}`,
      });
      await deps.onAfterDiscard();
      await deps.operations?.refreshOperations(repoId, wtId);
    } catch (err) {
      dialog.staleRejected = gitConflictReason(err) === "stale";
      dialog.dialogError = parseGitManageError(err);
    } finally {
      deps.operations?.clearActive();
      dialog.submitting = false;
    }
  }

  function closeDialog() {
    if (dialog.submitting) return;
    resetTarget();
  }

  return {
    get open() {
      return dialog.open;
    },
    get preview() {
      return dialog.preview;
    },
    get loadingPreview() {
      return dialog.loadingPreview;
    },
    get submitting() {
      return dialog.submitting;
    },
    get staleRejected() {
      return dialog.staleRejected;
    },
    get acknowledgeSessions() {
      return dialog.acknowledgeSessions;
    },
    setAcknowledgeSessions(val: boolean) {
      dialog.acknowledgeSessions = val;
    },
    get dialogError() {
      return dialog.dialogError;
    },
    dispose,
    resetTarget,
    openDiscardFiles,
    openDiscardHunk,
    openDiscardLines,
    refreshPreviewAfterStale,
    confirmDiscard,
    closeDialog,
  };
}
