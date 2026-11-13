import { APIError, repositoryRemotes } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import {
  gitRemoteAdd,
  gitRemoteRemove,
  gitRemoteUpdate,
} from "../../platform/git-v2-api";
import { GitConflictSchema, type Remote } from "../../contracts/schemas";
import { generateGitClientRequestId } from "../../domains/workspace/git-operations-model";
import { containsRedactedRemoteMask } from "../../domains/workspace/git-v2-adapter";
import type { GitOperationsController } from "./git-operations-controller.svelte";

export function parseGitManageError(err: unknown): string {
  if (err instanceof APIError && err.status === 409) {
    const parsed = GitConflictSchema.safeParse(err.body);
    if (parsed.success) return parsed.data.error;
  }
  return errorMessage(err);
}

export async function executeTrackedGitMutation<T>(options: {
  repoId: string;
  worktreeId?: string;
  operations?: GitOperationsController;
  opKind:
    | "branch_rename"
    | "branch_upstream"
    | "tag_create"
    | "tag_delete"
    | "remote_add"
    | "remote_update"
    | "remote_remove";
  progressText: string;
  setSubmitting: (busy: boolean) => void;
  setDialogError: (msg: string) => void;
  exec: (clientRequestId: string) => Promise<T>;
  afterSuccess: (res: T) => Promise<void> | void;
}): Promise<void> {
  options.setSubmitting(true);
  options.setDialogError("");
  const clientRequestId = generateGitClientRequestId(options.opKind);
  options.operations?.setPhase(
    "running",
    options.opKind,
    options.progressText,
    {
      clientRequestId,
    },
  );
  try {
    const res = await options.exec(clientRequestId);
    await options.afterSuccess(res);
    await options.operations?.refreshOperations(
      options.repoId,
      options.worktreeId,
    );
  } catch (err) {
    options.setDialogError(parseGitManageError(err));
  } finally {
    options.operations?.clearActive();
    options.setSubmitting(false);
  }
}

export interface GitRemotesControllerDeps {
  getRepoId: () => string;
  getWorktreeId?: () => string | undefined;
  getRemotes: () => readonly Remote[];
  onRemotesUpdated: (remotes: Remote[], summary?: string) => void;
  operations?: GitOperationsController;
  onRefreshRepositories?: () => void;
}

export function createGitRemotesController(deps: GitRemotesControllerDeps) {
  let editOpen = $state(false);
  let editMode = $state<"add" | "edit">("add");
  let name = $state("");
  let fetchUrl = $state("");
  let pushUrl = $state("");
  let removeTarget = $state<Remote | null>(null);
  let submitting = $state(false);
  let dialogError = $state("");

  function resetTarget() {
    editOpen = false;
    removeTarget = null;
    submitting = false;
    dialogError = "";
  }

  function closeDialogs() {
    if (submitting) return;
    editOpen = false;
    removeTarget = null;
    dialogError = "";
  }

  function openAddDialog() {
    editMode = "add";
    name = deps.getRemotes().length === 0 ? "origin" : "";
    fetchUrl = "";
    pushUrl = "";
    dialogError = "";
    editOpen = true;
  }

  function openEditDialog(remote: Remote) {
    editMode = "edit";
    name = remote.name;
    fetchUrl = containsRedactedRemoteMask(remote.fetchUrl)
      ? ""
      : remote.fetchUrl;
    pushUrl =
      remote.pushUrl !== remote.fetchUrl &&
      !containsRedactedRemoteMask(remote.pushUrl)
        ? remote.pushUrl
        : "";
    dialogError = "";
    editOpen = true;
  }

  async function refreshRemotes(summary?: string) {
    const repoId = deps.getRepoId();
    if (!repoId) return;
    const res = await repositoryRemotes(repoId).catch(() => null);
    if (res && repoId === deps.getRepoId()) {
      deps.onRemotesUpdated(res.remotes, summary);
    } else if (summary) {
      deps.onRemotesUpdated([...deps.getRemotes()], summary);
    }
    deps.onRefreshRepositories?.();
  }

  async function runRemoteMutation(
    opKind: "remote_add" | "remote_update" | "remote_remove",
    progressText: string,
    exec: (clientRequestId: string) => Promise<{ remotes: Remote[] }>,
    onSuccess: (remotes: Remote[]) => void,
  ) {
    const repoId = deps.getRepoId();
    if (!repoId || submitting) return;
    await executeTrackedGitMutation({
      repoId,
      worktreeId: deps.getWorktreeId?.(),
      operations: deps.operations,
      opKind,
      progressText,
      setSubmitting: (busy) => (submitting = busy),
      setDialogError: (msg) => (dialogError = msg),
      exec,
      afterSuccess: (res) => {
        onSuccess(res.remotes);
        deps.onRefreshRepositories?.();
      },
    });
  }

  async function confirmSave() {
    const repoId = deps.getRepoId();
    const cleanName = name.trim();
    const cleanFetch = fetchUrl.trim();
    const cleanPush = pushUrl.trim();
    if (!repoId || !cleanName || !cleanFetch || submitting) {
      dialogError = "请完整填写远程名称与拉取地址 (Fetch URL)";
      return;
    }
    if (
      containsRedactedRemoteMask(cleanFetch) ||
      containsRedactedRemoteMask(cleanPush)
    ) {
      dialogError = "远程地址不能包含脱敏掩码 ***，请输入完整有效 URL";
      return;
    }
    const isAdd = editMode === "add";
    await runRemoteMutation(
      isAdd ? "remote_add" : "remote_update",
      "正在保存远程仓库配置…",
      (clientRequestId) =>
        isAdd
          ? gitRemoteAdd(repoId, {
              name: cleanName,
              fetchUrl: cleanFetch,
              pushUrl: cleanPush || undefined,
              clientRequestId,
            })
          : gitRemoteUpdate(repoId, cleanName, {
              fetchUrl: cleanFetch,
              ...(cleanPush ? { pushUrl: cleanPush } : { clearPushUrl: true }),
              clientRequestId,
            }),
      (nextRemotes) => {
        editOpen = false;
        deps.onRemotesUpdated(
          nextRemotes,
          isAdd
            ? `已添加远程仓库 ${cleanName}`
            : `已更新远程仓库 ${cleanName} 配置`,
        );
      },
    );
  }

  function openRemoveDialog(remote: Remote) {
    removeTarget = remote;
    dialogError = "";
  }

  async function confirmRemove() {
    const repoId = deps.getRepoId();
    const target = removeTarget;
    if (!repoId || !target) return;
    await runRemoteMutation(
      "remote_remove",
      "正在移除远程仓库…",
      (clientRequestId) =>
        gitRemoteRemove(repoId, target.name, { clientRequestId }),
      (nextRemotes) => {
        removeTarget = null;
        deps.onRemotesUpdated(nextRemotes, `已移除远程仓库 ${target.name}`);
      },
    );
  }

  return {
    get editOpen() {
      return editOpen;
    },
    get editMode() {
      return editMode;
    },
    get name() {
      return name;
    },
    setName(val: string) {
      name = val;
    },
    get fetchUrl() {
      return fetchUrl;
    },
    setFetchUrl(val: string) {
      fetchUrl = val;
    },
    get pushUrl() {
      return pushUrl;
    },
    setPushUrl(val: string) {
      pushUrl = val;
    },
    get removeTarget() {
      return removeTarget;
    },
    get submitting() {
      return submitting;
    },
    get dialogError() {
      return dialogError;
    },
    resetTarget,
    closeDialogs,
    refreshRemotes,
    openAddDialog,
    openEditDialog,
    confirmSave,
    openRemoveDialog,
    confirmRemove,
  };
}

export type GitRemotesController = ReturnType<
  typeof createGitRemotesController
>;
