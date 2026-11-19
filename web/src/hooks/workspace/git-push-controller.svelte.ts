import { m } from "$lib/paraglide/messages.js";
import { SvelteMap } from "svelte/reactivity";
import { errorMessage } from "../../platform/error";
import { gitCommitDetail } from "../../platform/git-api";
import type {
  GitCommitDetail,
  GitPrecedingCommit,
} from "../../contracts/git-workbench-schemas";
import type {
  Commit,
  PushPreview,
  PushResult,
  Remote,
  Repository,
} from "../../contracts/schemas";
import { branchOperations } from "../../domains/workspace/branch-operations";
import { mapGitCommitDetailResponse } from "../../domains/workspace/git-workbench-adapter";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import {
  createGitPushDiffController,
  type GitPushFileDiffState,
} from "./git-push-diff.svelte";
import {
  canConfirmPushPreview,
  runConfirmPushFlow,
} from "./git-push-execution";

export type { GitPushFileDiffState };

export interface GitPushControllerDeps {
  getRepoId: () => string;
  getRepoName: () => string;
  getWorktreeId: () => string | undefined;
  operations?: GitOperationsController;
  onRepositoryUpdated?: (repo: Repository) => void;
  onRefreshAfterPush?: () => void;
  onStatusBanner: (banner: string) => void;
}

export function createGitPushController(deps: GitPushControllerDeps) {
  let open = $state(false);
  let branch = $state("");
  let precedingCommit = $state<GitPrecedingCommit | null>(null);
  let publishing = $state(false);
  let remote = $state("");
  let target = $state("");
  let remotes = $state<Remote[]>([]);
  let remotesLoading = $state(false);
  let remotesError = $state("");
  let preview = $state<PushPreview | null>(null);
  let loading = $state(false);
  let running = $state(false);
  let loadError = $state("");
  let rejected = $state<PushResult | null>(null);
  let conflictError = $state("");
  let selectedCommitOid = $state<string | null>(null);
  let selectedParentOid = $state<string>("");
  let selectedFilePath = $state<string | null>(null);
  let detailLoading = $state(false);

  const detailsByOid = new SvelteMap<string, GitCommitDetail>();
  const previewReq = new LatestRequest();
  const remotesReq = new LatestRequest();
  const detailReq = new LatestRequest();
  let targetTimer: ReturnType<typeof setTimeout> | null = null;
  let disposed = false;

  const diffCtrl = createGitPushDiffController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
    isOpen: () => open,
  });

  function isSameTarget(repoId: string, wtId: string | undefined): boolean {
    return (
      !disposed && repoId === deps.getRepoId() && wtId === deps.getWorktreeId()
    );
  }

  function clearTimer() {
    if (targetTimer !== null) {
      clearTimeout(targetTimer);
      targetTimer = null;
    }
  }

  function dispose() {
    disposed = true;
    clearTimer();
    previewReq.dispose();
    remotesReq.dispose();
    detailReq.dispose();
    diffCtrl.dispose();
  }

  function resetDialogState() {
    clearTimer();
    previewReq.cancel();
    remotesReq.cancel();
    detailReq.cancel();
    diffCtrl.reset();
    open = false;
    branch = "";
    precedingCommit = null;
    publishing = false;
    remote = "";
    target = "";
    remotes = [];
    remotesLoading = false;
    remotesError = "";
    preview = null;
    loading = false;
    running = false;
    loadError = "";
    rejected = null;
    conflictError = "";
    selectedCommitOid = null;
    selectedParentOid = "";
    selectedFilePath = null;
    detailLoading = false;
    detailsByOid.clear();
  }

  function resetTarget() {
    resetDialogState();
  }

  async function selectCommit(oid: string, parentOverride?: string) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !oid) return;
    detailReq.cancel();
    detailLoading = false;
    selectedCommitOid = oid;
    selectedFilePath = null;
    const existing = detailsByOid.get(oid);
    if (
      existing &&
      (!parentOverride || Boolean(existing.filesByParent[parentOverride]))
    ) {
      const pKey = parentOverride ?? existing.selectedParent;
      selectedParentOid = pKey;
      const files = existing.filesByParent[pKey] ?? [];
      const firstFile = files[0] ?? null;
      selectedFilePath = firstFile
        ? firstFile.newPath || firstFile.oldPath
        : null;
      void diffCtrl.loadFileDiff(oid, pKey, firstFile);
      return;
    }

    const req = detailReq.begin();
    selectedParentOid = parentOverride ?? "";
    void diffCtrl.loadFileDiff(oid, selectedParentOid, null);
    detailLoading = true;
    try {
      const raw = await gitCommitDetail(
        repoId,
        oid,
        parentOverride,
        req.signal,
      );
      if (!req.current() || !open || !isSameTarget(repoId, wtId)) return;
      const mapped = mapGitCommitDetailResponse(raw, existing);
      detailsByOid.set(oid, mapped);
      const pKey = parentOverride ?? mapped.selectedParent;
      selectedParentOid = pKey;
      const files = mapped.filesByParent[pKey] ?? [];
      const firstFile = files[0] ?? null;
      selectedFilePath = firstFile
        ? firstFile.newPath || firstFile.oldPath
        : null;
      void diffCtrl.loadFileDiff(oid, pKey, firstFile);
    } catch {
      /* Commit summary stays visible even if detail fails */
    } finally {
      if (req.current() && isSameTarget(repoId, wtId)) detailLoading = false;
    }
  }

  function selectFile(path: string) {
    const oid = selectedCommitOid;
    if (!oid || detailLoading) return;
    const detail = detailsByOid.get(oid);
    if (!detail) return;
    const pKey = selectedParentOid || detail.selectedParent;
    const files = detail.filesByParent[pKey] ?? [];
    const chosen = files.find((f) => (f.newPath || f.oldPath) === path) ?? null;
    selectedFilePath = path;
    void diffCtrl.loadFileDiff(oid, pKey, chosen);
  }

  async function refreshPreview() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !branch) return;
    const req = previewReq.begin();
    loading = true;
    loadError = "";
    try {
      const targetTrimmed = target.trim();
      const res = await branchOperations.pushPreview(
        repoId,
        publishing
          ? {
              branch,
              remote: remote || undefined,
              target: targetTrimmed || undefined,
            }
          : { branch },
        req.signal,
      );
      if (!req.current() || !open || !isSameTarget(repoId, wtId)) return;
      if (res.publish && !publishing) {
        publishing = true;
        remote = res.remote;
        target = res.target;
      }
      preview = res;
      const items: Commit[] = res.commits.items;
      const chosenOid =
        (selectedCommitOid && items.some((c) => c.hash === selectedCommitOid)
          ? selectedCommitOid
          : items[0]?.hash) ?? null;
      if (chosenOid) {
        void selectCommit(chosenOid);
      } else {
        detailReq.cancel();
        detailLoading = false;
        selectedCommitOid = null;
        selectedFilePath = null;
        diffCtrl.reset();
      }
    } catch (err) {
      if (!req.reportable() || !open || !isSameTarget(repoId, wtId)) return;
      preview = null;
      loadError = errorMessage(err);
    } finally {
      if (req.current() && isSameTarget(repoId, wtId)) loading = false;
    }
  }

  function openDialog(
    targetBranch: string,
    preceding?: GitPrecedingCommit | null,
  ) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !targetBranch) return;
    resetDialogState();
    open = true;
    branch = targetBranch;
    precedingCommit = preceding ?? null;

    const rReq = remotesReq.begin();
    remotesLoading = true;
    branchOperations
      .remotes(repoId, rReq.signal)
      .then((res) => {
        if (rReq.current() && open && isSameTarget(repoId, wtId)) {
          remotes = res.remotes;
        }
      })
      .catch((err) => {
        if (rReq.reportable() && open && isSameTarget(repoId, wtId)) {
          remotesError = errorMessage(err);
        }
      })
      .finally(() => {
        if (rReq.current() && isSameTarget(repoId, wtId)) {
          remotesLoading = false;
        }
      });

    void refreshPreview();
  }

  function closeDialog() {
    if (running) return;
    const hadPreceding = precedingCommit;
    resetDialogState();
    if (hadPreceding) {
      deps.onStatusBanner(
        m.s7_push_cancelled_after_commit({ oid: hadPreceding.shortOid }),
      );
    }
  }

  function invalidateDestination() {
    clearTimer();
    previewReq.cancel();
    detailReq.cancel();
    diffCtrl.reset();
    preview = null;
    rejected = null;
    conflictError = "";
    loadError = "";
    selectedCommitOid = null;
    selectedParentOid = "";
    selectedFilePath = null;
    detailLoading = false;
    loading = Boolean(target.trim());
  }

  function setPublishRemote(nextRemote: string) {
    if (running || remote === nextRemote) return;
    remote = nextRemote;
    invalidateDestination();
    if (target.trim()) void refreshPreview();
  }

  function setPublishTarget(nextTarget: string) {
    if (running) return;
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const changed = target.trim() !== nextTarget.trim();
    target = nextTarget;
    if (!changed) return;
    invalidateDestination();
    if (!target.trim()) return;
    targetTimer = setTimeout(() => {
      targetTimer = null;
      if (!open || !isSameTarget(repoId, wtId)) return;
      void refreshPreview();
    }, 280);
  }

  async function confirmPush() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const check = {
      open,
      repoId,
      preview,
      running,
      loading,
      hasPendingDebounce: targetTimer !== null,
      publishing,
      remote,
      target,
    };
    if (!canConfirmPushPreview(check)) return;

    await runConfirmPushFlow({
      repoId,
      wtId,
      preview: check.preview,
      precedingCommit,
      isOpen: () => open,
      isSameTarget: () => isSameTarget(repoId, wtId),
      operations: deps.operations,
      onRepositoryUpdated: deps.onRepositoryUpdated,
      onRefreshAfterPush: deps.onRefreshAfterPush,
      onStatusBanner: deps.onStatusBanner,
      setRunning: (next) => (running = next),
      setRejected: (next) => (rejected = next),
      setConflictError: (next) => (conflictError = next),
      onInvalidatePreview: () => {
        preview = null;
      },
      onResetDialog: resetDialogState,
      onRefreshPreview: () => void refreshPreview(),
    });
  }

  return {
    get open() {
      return open;
    },
    get repositoryName() {
      return deps.getRepoName();
    },
    get branch() {
      return branch;
    },
    get precedingCommit() {
      return precedingCommit;
    },
    get publishing() {
      return publishing;
    },
    get remote() {
      return remote;
    },
    get target() {
      return target;
    },
    get remotes() {
      return remotes;
    },
    get remotesLoading() {
      return remotesLoading;
    },
    get remotesError() {
      return remotesError;
    },
    get preview() {
      return preview;
    },
    get loading() {
      return loading;
    },
    get running() {
      return running;
    },
    get loadError() {
      return loadError;
    },
    get rejected() {
      return rejected;
    },
    get conflictError() {
      return conflictError;
    },
    get selectedCommitOid() {
      return selectedCommitOid;
    },
    get selectedParentOid() {
      return selectedParentOid;
    },
    get selectedFilePath() {
      return selectedFilePath;
    },
    get detailLoading() {
      return detailLoading;
    },
    get activeDetail() {
      return selectedCommitOid
        ? (detailsByOid.get(selectedCommitOid) ?? null)
        : null;
    },
    get fileDiffState() {
      return diffCtrl.state;
    },
    dispose,
    resetTarget,
    openDialog,
    closeDialog,
    refreshPreview,
    setPublishRemote,
    setPublishTarget,
    selectCommit,
    selectParent(parentOid: string) {
      if (selectedCommitOid) void selectCommit(selectedCommitOid, parentOid);
    },
    selectFile,
    confirmPush,
  };
}
export type GitPushController = ReturnType<typeof createGitPushController>;
