import { SvelteSet } from "svelte/reactivity";
import { repositoryBranches, repositoryRemotes } from "../../platform/api";
import {
  gitBranchRename,
  gitBranchRenamePreview,
  gitBranchSetUpstream,
  gitTagCreate,
  gitTagDelete,
} from "../../platform/git-v2-api";
import type {
  GitBranchRenamePreview,
  Repository,
  SessionInfo,
} from "../../contracts/schemas";
import type {
  GitBranchItem,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import { readFavoriteBranches } from "../../domains/workspace/git-favorites";
import { mapGitBranchesResponse } from "../../domains/workspace/git-workbench-adapter";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import {
  createGitRemotesController,
  executeTrackedGitMutation,
  parseGitManageError,
} from "./git-remotes-controller.svelte";

export interface GitRefsManageDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getRepository: () => Repository | undefined;
  getSessions: () => readonly SessionInfo[];
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  operations?: GitOperationsController;
  onRefreshRepositories?: () => void;
  onAfterRefsChanged?: () => void;
}

export function createGitRefsManageController(deps: GitRefsManageDeps) {
  let renameOpen = $state(false);
  let renameOldName = $state("");
  let renameNewName = $state("");
  let renameAckSessions = $state(false);
  let renamePreview = $state<GitBranchRenamePreview | null>(null);

  let upstreamOpen = $state(false);
  let upstreamBranch = $state("");
  let upstreamCurrent = $state("");
  let upstreamSelection = $state("");

  let tagCreateOpen = $state(false);
  let tagTargetOid = $state("");
  let tagTargetLabel = $state("");
  let tagName = $state("");
  let tagKind = $state<"lightweight" | "annotated">("annotated");
  let tagMessage = $state("");

  let tagDeleteOpen = $state(false);
  let tagDeleteTarget = $state<GitBranchItem | null>(null);

  let loadingPreview = $state(false);
  let submitting = $state(false);
  let dialogError = $state("");

  const previewReq = new LatestRequest();
  let disposed = false;

  const remotes = createGitRemotesController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
    getRemotes: () => deps.getView().remotes ?? [],
    onRemotesUpdated: (nextRemotes, summary) => {
      deps.patchView({
        remotes: nextRemotes,
        ...(summary ? { statusBanner: summary } : {}),
      });
      deps.onAfterRefsChanged?.();
    },
    operations: deps.operations,
    onRefreshRepositories: deps.onRefreshRepositories,
  });

  function dispose() {
    disposed = true;
    previewReq.dispose();
  }

  function resetTarget() {
    previewReq.cancel();
    remotes.resetTarget();
    renameOpen = false;
    renamePreview = null;
    upstreamOpen = false;
    tagCreateOpen = false;
    tagDeleteOpen = false;
    tagDeleteTarget = null;
    loadingPreview = false;
    submitting = false;
    dialogError = "";
  }

  function closeDialogs() {
    if (submitting) return;
    previewReq.cancel();
    remotes.closeDialogs();
    renameOpen = false;
    upstreamOpen = false;
    tagCreateOpen = false;
    tagDeleteOpen = false;
    dialogError = "";
  }

  async function refreshRefsAndRemotes() {
    const repoId = deps.getRepoId();
    if (!repoId || disposed) return;
    const [branchesRes, remotesRes] = await Promise.all([
      repositoryBranches(repoId).catch(() => null),
      repositoryRemotes(repoId).catch(() => null),
    ]);
    if (disposed || repoId !== deps.getRepoId()) return;
    const curView = deps.getView();
    const patch: Partial<GitWorkbenchViewState> = {};
    if (branchesRes) {
      const localNames = new SvelteSet(branchesRes.branches.map((b) => b.name));
      const favorites = readFavoriteBranches(
        repoId,
        branchesRes.defaultBranch,
        localNames,
      );
      patch.branches = mapGitBranchesResponse(
        deps.getRepository(),
        curView.context.worktreePath,
        deps.getSessions(),
        branchesRes,
        favorites,
      );
    }
    if (remotesRes) {
      patch.remotes = remotesRes.remotes;
    }
    if (Object.keys(patch).length > 0) {
      deps.patchView(patch);
    }
    deps.onRefreshRepositories?.();
    deps.onAfterRefsChanged?.();
  }

  async function refreshRenamePreview() {
    const repoId = deps.getRepoId();
    const cleanNew = renameNewName.trim();
    if (!repoId || !renameOldName || !cleanNew) {
      renamePreview = null;
      return;
    }
    loadingPreview = true;
    dialogError = "";
    const req = previewReq.begin();
    try {
      const res = await gitBranchRenamePreview(
        repoId,
        { oldName: renameOldName, newName: cleanNew },
        req.signal,
      );
      if (!req.current() || disposed) return;
      renamePreview = res;
    } catch (err) {
      if (req.reportable()) dialogError = parseGitManageError(err);
    } finally {
      if (req.current()) loadingPreview = false;
    }
  }

  function openRenameDialog(branchName: string) {
    renameOldName = branchName;
    renameNewName = branchName;
    renameAckSessions = false;
    renamePreview = null;
    dialogError = "";
    renameOpen = true;
    void refreshRenamePreview();
  }

  async function runRefMutation<T>(
    opKind: "branch_rename" | "branch_upstream" | "tag_create" | "tag_delete",
    progressText: string,
    exec: (repoId: string, clientRequestId: string) => Promise<T>,
    onSuccess: (res: T) => void,
  ) {
    const repoId = deps.getRepoId();
    if (!repoId || submitting) return;
    await executeTrackedGitMutation({
      repoId,
      worktreeId: deps.getWorktreeId(),
      operations: deps.operations,
      opKind,
      progressText,
      setSubmitting: (busy) => (submitting = busy),
      setDialogError: (msg) => (dialogError = msg),
      exec: (clientRequestId) => exec(repoId, clientRequestId),
      afterSuccess: async (res) => {
        onSuccess(res);
        await refreshRefsAndRemotes();
      },
    });
  }

  async function confirmRename() {
    const cur = renamePreview;
    if (!cur?.confirmationId) return;
    const oldBranch = renameOldName;
    await runRefMutation(
      "branch_rename",
      "正在重命名本地分支…",
      (repoId, clientRequestId) =>
        gitBranchRename(repoId, {
          confirmationId: cur.confirmationId,
          clientRequestId,
          acknowledgeSessions: renameAckSessions,
        }),
      (res) => {
        renameOpen = false;
        renamePreview = null;
        const curView = deps.getView();
        const checkedOutUpdated =
          curView.context.checkedOutBranch === oldBranch
            ? { ...curView.context, checkedOutBranch: res.branch }
            : curView.context;
        deps.patchView({
          context: checkedOutUpdated,
          statusBanner: `已将本地分支 ${oldBranch} 重命名为 ${res.branch}`,
        });
      },
    );
  }

  function openUpstreamDialog(item: GitBranchItem) {
    upstreamBranch = item.name;
    upstreamCurrent = item.upstream ?? "";
    const remoteItems = deps
      .getView()
      .branches.filter((b) => b.kind === "remote");
    upstreamSelection =
      item.upstream ||
      remoteItems.find((r) => r.name.endsWith(`/${item.name}`))?.name ||
      remoteItems[0]?.name ||
      "";
    dialogError = "";
    upstreamOpen = true;
  }

  async function confirmSetUpstream(unset = false) {
    if (!upstreamBranch) return;
    const targetUpstream = unset ? "" : upstreamSelection.trim();
    if (!unset && !targetUpstream) {
      dialogError = "请选择或输入远程跟踪引用（例如 origin/main）";
      return;
    }
    await runRefMutation(
      "branch_upstream",
      "正在更新分支上游跟踪…",
      (repoId, clientRequestId) =>
        gitBranchSetUpstream(repoId, {
          branch: upstreamBranch,
          upstream: targetUpstream || undefined,
          clientRequestId,
        }),
      (res) => {
        upstreamOpen = false;
        deps.patchView({
          statusBanner: res.upstream
            ? `已将分支 ${res.branch} 上游设置为 ${res.upstream}`
            : `已取消分支 ${res.branch} 的上游跟踪`,
        });
      },
    );
  }

  function openCreateTagDialog(targetOid: string, label = "") {
    tagTargetOid = targetOid;
    tagTargetLabel = label;
    tagName = "";
    tagKind = "annotated";
    tagMessage = "";
    dialogError = "";
    tagCreateOpen = true;
  }

  async function confirmCreateTag() {
    const cleanName = tagName.trim();
    if (!cleanName || !tagTargetOid) return;
    if (tagKind === "annotated" && !tagMessage.trim()) {
      dialogError = "附注标签 (Annotated Tag) 必须填写标签说明";
      return;
    }
    await runRefMutation(
      "tag_create",
      "正在创建本地标签…",
      (repoId, clientRequestId) =>
        gitTagCreate(repoId, {
          name: cleanName,
          targetOid: tagTargetOid,
          annotated: tagKind === "annotated",
          message: tagKind === "annotated" ? tagMessage.trim() : undefined,
          clientRequestId,
        }),
      (res) => {
        tagCreateOpen = false;
        deps.patchView({
          statusBanner: `已创建${res.tag.kind === "annotated" ? "附注" : "轻量"}标签 ${res.tag.name} (${res.tag.head.slice(0, 7)})`,
        });
      },
    );
  }

  function openDeleteTagDialog(item: GitBranchItem) {
    tagDeleteTarget = item;
    dialogError = "";
    tagDeleteOpen = true;
  }

  async function confirmDeleteTag() {
    const target = tagDeleteTarget;
    if (!target) return;
    await runRefMutation(
      "tag_delete",
      "正在删除本地标签…",
      (repoId, clientRequestId) =>
        gitTagDelete(repoId, {
          name: target.name,
          expectedTargetOid: target.head,
          clientRequestId,
        }),
      (res) => {
        tagDeleteOpen = false;
        tagDeleteTarget = null;
        deps.patchView({
          statusBanner: `已删除本地标签 ${res.name}（恢复命令：${res.recoveryCommand}）`,
        });
      },
    );
  }

  return {
    remotes,
    get renameOpen() {
      return renameOpen;
    },
    get renameOldName() {
      return renameOldName;
    },
    get renameNewName() {
      return renameNewName;
    },
    setRenameNewName(val: string) {
      renameNewName = val;
      void refreshRenamePreview();
    },
    get renameAckSessions() {
      return renameAckSessions;
    },
    setRenameAckSessions(val: boolean) {
      renameAckSessions = val;
    },
    get renamePreview() {
      return renamePreview;
    },
    get upstreamOpen() {
      return upstreamOpen;
    },
    get upstreamBranch() {
      return upstreamBranch;
    },
    get upstreamCurrent() {
      return upstreamCurrent;
    },
    get upstreamSelection() {
      return upstreamSelection;
    },
    setUpstreamSelection(val: string) {
      upstreamSelection = val;
    },
    get tagCreateOpen() {
      return tagCreateOpen;
    },
    get tagTargetOid() {
      return tagTargetOid;
    },
    get tagTargetLabel() {
      return tagTargetLabel;
    },
    get tagName() {
      return tagName;
    },
    setTagName(val: string) {
      tagName = val;
    },
    get tagKind() {
      return tagKind;
    },
    setTagKind(val: "lightweight" | "annotated") {
      tagKind = val;
    },
    get tagMessage() {
      return tagMessage;
    },
    setTagMessage(val: string) {
      tagMessage = val;
    },
    get tagDeleteOpen() {
      return tagDeleteOpen;
    },
    get tagDeleteTarget() {
      return tagDeleteTarget;
    },
    get loadingPreview() {
      return loadingPreview;
    },
    get submitting() {
      return submitting;
    },
    get dialogError() {
      return dialogError;
    },
    dispose,
    resetTarget,
    closeDialogs,
    refreshRefsAndRemotes,
    openRenameDialog,
    refreshRenamePreview,
    confirmRename,
    openUpstreamDialog,
    confirmSetUpstream,
    openCreateTagDialog,
    confirmCreateTag,
    openDeleteTagDialog,
    confirmDeleteTag,
  };
}
