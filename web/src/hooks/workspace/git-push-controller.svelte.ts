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
import {
  generateGitClientRequestId,
  isRecoverableNetworkFailure,
} from "../../domains/workspace/git-operations-model";
import { mapGitCommitDetailResponse } from "../../domains/workspace/git-workbench-adapter";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import {
  createGitPushDiffController,
  type GitPushFileDiffState,
} from "./git-push-diff.svelte";

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

  const diffCtrl = createGitPushDiffController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
    isOpen: () => open,
  });

  function clearTimer() {
    if (targetTimer !== null) {
      clearTimeout(targetTimer);
      targetTimer = null;
    }
  }

  function dispose() {
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

  async function selectCommit(oid: string, parentOverride?: string) {
    const repoId = deps.getRepoId();
    if (!repoId || !oid) return;
    selectedCommitOid = oid;
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
    detailLoading = true;
    try {
      const raw = await gitCommitDetail(
        repoId,
        oid,
        parentOverride,
        req.signal,
      );
      if (!req.current() || !open) return;
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
      if (req.current()) detailLoading = false;
    }
  }

  function selectFile(path: string) {
    const oid = selectedCommitOid;
    if (!oid) return;
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
      if (!req.current() || !open) return;
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
        selectedCommitOid = null;
        selectedFilePath = null;
        diffCtrl.reset();
      }
    } catch (err) {
      if (!req.reportable()) return;
      preview = null;
      loadError = errorMessage(err);
    } finally {
      if (req.current()) loading = false;
    }
  }

  function openDialog(
    targetBranch: string,
    preceding?: GitPrecedingCommit | null,
  ) {
    const repoId = deps.getRepoId();
    if (!repoId || !targetBranch) return;
    resetDialogState();
    open = true;
    branch = targetBranch;
    precedingCommit = preceding ?? null;

    const rReq = remotesReq.begin();
    branchOperations
      .remotes(repoId, rReq.signal)
      .then((res) => {
        if (rReq.current() && open) remotes = res.remotes;
      })
      .catch((err) => {
        if (rReq.reportable() && open) remotesError = errorMessage(err);
      });

    void refreshPreview();
  }

  function closeDialog() {
    if (running) return;
    const hadPreceding = precedingCommit;
    resetDialogState();
    if (hadPreceding) {
      deps.onStatusBanner(
        `本地已提交（${hadPreceding.shortOid}），已取消推送；本地提交已保留，可随时通过「推送…」继续推送。`,
      );
    }
  }

  function setPublishRemote(nextRemote: string) {
    remote = nextRemote;
    conflictError = "";
    void refreshPreview();
  }

  function setPublishTarget(nextTarget: string) {
    target = nextTarget;
    conflictError = "";
    clearTimer();
    if (!nextTarget.trim()) {
      preview = preview ? { ...preview, confirmationId: "" } : null;
      return;
    }
    targetTimer = setTimeout(() => {
      targetTimer = null;
      void refreshPreview();
    }, 280);
  }

  async function confirmPush() {
    const repoId = deps.getRepoId();
    const cur = preview;
    if (!repoId || !cur || !cur.confirmationId || running || loading) return;
    if (publishing && (cur.remote !== remote || cur.target !== target.trim())) {
      return;
    }

    running = true;
    rejected = null;
    conflictError = "";
    const wtId = deps.getWorktreeId();
    const clientRequestId = generateGitClientRequestId("push");
    deps.operations?.setPhase(
      "running",
      "push",
      `正在推送到 ${cur.remote}/${cur.target}…`,
      { clientRequestId },
    );
    try {
      const result = await branchOperations.push(repoId, cur.confirmationId, {
        clientRequestId,
        worktreeId: wtId,
      });
      deps.onRepositoryUpdated?.(result.repository);
      deps.onRefreshAfterPush?.();
      await deps.operations?.refreshOperations(repoId, wtId);

      if (result.status === "ok") {
        const dest = `${cur.remote}/${cur.target}`;
        const prefix = precedingCommit
          ? `本地已提交（${precedingCommit.shortOid}）并`
          : "已";
        const verb = cur.publish ? "发布" : "推送";
        resetDialogState();
        deps.onStatusBanner(`${prefix}${verb}到 ${dest}`);
        return;
      }

      rejected = result;
      const failMsg = result.message || "远端拒绝了本次推送";
      deps.onStatusBanner(
        precedingCommit
          ? `本地已提交（${precedingCommit.shortOid}），推送未完成：${failMsg}`
          : `推送未完成：${failMsg}`,
      );
      await refreshPreview();
    } catch (err) {
      if (deps.operations && isRecoverableNetworkFailure(err)) {
        const recovered = await deps.operations.recoverByClientRequest({
          repositoryId: repoId,
          worktreeId: wtId,
          kind: "push",
          clientRequestId,
        });
        if (recovered && recovered.status === "succeeded") {
          deps.onRefreshAfterPush?.();
          const dest = `${cur.remote}/${cur.target}`;
          resetDialogState();
          deps.onStatusBanner(recovered.summary || `已推送到 ${dest}`);
          return;
        }
      }
      await deps.operations?.refreshOperations(repoId, wtId);
      const msg = errorMessage(err);
      conflictError = msg;
      deps.onStatusBanner(
        precedingCommit
          ? `本地已提交（${precedingCommit.shortOid}），推送未完成：${msg}`
          : `推送未完成：${msg}`,
      );
      await refreshPreview();
    } finally {
      deps.operations?.clearActive();
      running = false;
    }
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
