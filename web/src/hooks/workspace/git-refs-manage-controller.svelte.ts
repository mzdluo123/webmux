import { m } from "$lib/paraglide/messages.js";
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
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import {
  canConfirmBranchRename,
  formatTagCreatedBanner,
  formatUpstreamUpdatedBanner,
  refreshRepositoryRefsAndRemotes,
  resolveInitialUpstreamSelection,
} from "./git-refs-manage-actions";
import {
  createGitRemotesController,
  executeTrackedGitMutation,
  runLatestGitPreview,
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
  let renameTimer: ReturnType<typeof setTimeout> | null = null;
  let disposed = false;

  function clearRenameTimer() {
    if (renameTimer !== null) {
      clearTimeout(renameTimer);
      renameTimer = null;
    }
  }

  function isSameTarget(repoId: string, wtId: string | undefined): boolean {
    return (
      !disposed && repoId === deps.getRepoId() && wtId === deps.getWorktreeId()
    );
  }

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
    clearRenameTimer();
    previewReq.dispose();
  }

  function resetTarget() {
    clearRenameTimer();
    previewReq.cancel();
    remotes.resetTarget();
    renameOpen = false;
    renamePreview = null;
    renameAckSessions = false;
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
    clearRenameTimer();
    previewReq.cancel();
    remotes.closeDialogs();
    loadingPreview = false;
    renameOpen = false;
    renamePreview = null;
    renameAckSessions = false;
    upstreamOpen = false;
    tagCreateOpen = false;
    tagDeleteOpen = false;
    dialogError = "";
  }

  async function refreshRefsAndRemotes() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    await refreshRepositoryRefsAndRemotes({
      repoId,
      isCurrentTarget: () => isSameTarget(repoId, wtId),
      getRepository: deps.getRepository,
      getSessions: deps.getSessions,
      getView: deps.getView,
      patchView: deps.patchView,
      onRefreshRepositories: deps.onRefreshRepositories,
      onAfterRefsChanged: deps.onAfterRefsChanged,
    });
  }

  async function refreshRenamePreview() {
    clearRenameTimer();
    previewReq.cancel();
    renamePreview = null;
    renameAckSessions = false;
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const oldName = renameOldName;
    const cleanNew = renameNewName.trim();
    if (!repoId || !oldName || !cleanNew || cleanNew === oldName) {
      loadingPreview = false;
      return;
    }
    await runLatestGitPreview({
      tracker: previewReq,
      isActive: () => renameOpen && isSameTarget(repoId, wtId),
      setLoading: (busy) => (loadingPreview = busy),
      setDialogError: (msg) => (dialogError = msg),
      load: (signal) =>
        gitBranchRenamePreview(repoId, { oldName, newName: cleanNew }, signal),
      onLoaded: (res) => {
        renamePreview = res;
      },
      onCleared: () => {
        renamePreview = null;
        renameAckSessions = false;
      },
    });
  }

  function openRenameDialog(branchName: string) {
    clearRenameTimer();
    previewReq.cancel();
    renameOldName = branchName;
    renameNewName = branchName;
    renameAckSessions = false;
    renamePreview = null;
    loadingPreview = false;
    dialogError = "";
    renameOpen = true;
  }

  async function runRefMutation<T>(
    opKind: "branch_rename" | "branch_upstream" | "tag_create" | "tag_delete",
    progressText: string,
    exec: (repoId: string, clientRequestId: string) => Promise<T>,
    onSuccess: (res: T) => void,
    onFailure?: () => void,
  ) {
    if (submitting) return;
    await executeTrackedGitMutation({
      getRepoId: deps.getRepoId,
      getWorktreeId: deps.getWorktreeId,
      isAlive: () => !disposed,
      operations: deps.operations,
      opKind,
      progressText,
      setSubmitting: (busy) => (submitting = busy),
      setDialogError: (msg) => (dialogError = msg),
      exec,
      afterSuccess: async (res) => {
        onSuccess(res);
        await refreshRefsAndRemotes();
      },
      onFailure,
    });
  }

  async function confirmRename() {
    const check = {
      renameOpen,
      loadingPreview,
      submitting,
      hasPendingDebounce: renameTimer !== null,
      oldName: renameOldName,
      newName: renameNewName,
      acknowledgeSessions: renameAckSessions,
      preview: renamePreview,
    };
    if (!canConfirmBranchRename(check)) return;
    const oldBranch = renameOldName;
    const ack = renameAckSessions;
    const confirmationId = check.preview.confirmationId;
    await runRefMutation(
      "branch_rename",
      m.s7_refs_renaming_phase(),
      (repoId, clientRequestId) =>
        gitBranchRename(repoId, {
          confirmationId,
          clientRequestId,
          acknowledgeSessions: ack,
        }),
      (res) => {
        renameOpen = false;
        renamePreview = null;
        renameAckSessions = false;
        const curView = deps.getView();
        const checkedOutUpdated =
          curView.context.checkedOutBranch === oldBranch
            ? { ...curView.context, checkedOutBranch: res.branch }
            : curView.context;
        deps.patchView({
          context: checkedOutUpdated,
          statusBanner: m.s7_refs_renamed_banner({
            oldName: oldBranch,
            newName: res.branch,
          }),
        });
      },
      () => {
        renamePreview = null;
        renameAckSessions = false;
      },
    );
  }

  function openUpstreamDialog(item: GitBranchItem) {
    upstreamBranch = item.name;
    upstreamCurrent = item.upstream ?? "";
    upstreamSelection = resolveInitialUpstreamSelection(
      item,
      deps.getView().branches,
    );
    dialogError = "";
    upstreamOpen = true;
  }

  async function confirmSetUpstream(unset = false) {
    if (!upstreamBranch) return;
    const targetUpstream = unset ? "" : upstreamSelection.trim();
    if (!unset && !targetUpstream) {
      dialogError = m.s7_refs_upstream_empty_error();
      return;
    }
    await runRefMutation(
      "branch_upstream",
      m.s7_refs_upstream_updating_phase(),
      (repoId, clientRequestId) =>
        gitBranchSetUpstream(repoId, {
          branch: upstreamBranch,
          upstream: targetUpstream || undefined,
          clientRequestId,
        }),
      (res) => {
        upstreamOpen = false;
        deps.patchView({
          statusBanner: formatUpstreamUpdatedBanner(res.branch, res.upstream),
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
      dialogError = m.s7_refs_tag_annotated_msg_req();
      return;
    }
    await runRefMutation(
      "tag_create",
      m.s7_refs_tag_creating_phase(),
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
          statusBanner: formatTagCreatedBanner(res.tag),
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
      m.s7_refs_tag_deleting_phase(),
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
          statusBanner: m.s7_refs_tag_deleted_banner({
            name: res.name,
            cmd: res.recoveryCommand,
          }),
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
      if (submitting || val === renameNewName) return;
      renameNewName = val;
      clearRenameTimer();
      previewReq.cancel();
      renamePreview = null;
      renameAckSessions = false;
      dialogError = "";
      const cleanNew = val.trim();
      if (!cleanNew || cleanNew === renameOldName || !deps.getRepoId()) {
        loadingPreview = false;
        return;
      }
      loadingPreview = true;
      renameTimer = setTimeout(() => {
        renameTimer = null;
        void refreshRenamePreview();
      }, 250);
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
