import { m } from "$lib/paraglide/messages.js";
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
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";

export function parseGitManageError(err: unknown): string {
  if (err instanceof APIError && err.status === 409) {
    const parsed = GitConflictSchema.safeParse(err.body);
    if (parsed.success) return parsed.data.error;
  }
  return errorMessage(err);
}

export async function runLatestGitPreview<T>(options: {
  tracker: LatestRequest;
  isActive: () => boolean;
  setLoading: (loading: boolean) => void;
  setDialogError: (error: string) => void;
  load: (signal: AbortSignal) => Promise<T>;
  onLoaded: (preview: T) => void;
  onCleared: () => void;
}): Promise<void> {
  options.setLoading(true);
  options.setDialogError("");
  const req = options.tracker.begin();
  try {
    const res = await options.load(req.signal);
    if (!req.current() || !options.isActive()) return;
    options.onLoaded(res);
  } catch (err) {
    options.onCleared();
    if (req.reportable() && options.isActive()) {
      options.setDialogError(parseGitManageError(err));
    }
  } finally {
    if (req.current() && options.isActive()) {
      options.setLoading(false);
    }
  }
}

export async function executeTrackedGitMutation<T>(options: {
  getRepoId: () => string;
  getWorktreeId?: () => string | undefined;
  isAlive?: () => boolean;
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
  exec: (repoId: string, clientRequestId: string) => Promise<T>;
  afterSuccess: (res: T) => Promise<void> | void;
  onFailure?: () => void;
}): Promise<void> {
  const repoId = options.getRepoId();
  const worktreeId = options.getWorktreeId?.();
  if (!repoId) return;
  const stillCurrent = () =>
    (options.isAlive ? options.isAlive() : true) &&
    repoId === options.getRepoId() &&
    worktreeId === options.getWorktreeId?.();
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
    const res = await options.exec(repoId, clientRequestId);
    if (!stillCurrent()) return;
    await options.afterSuccess(res);
    if (!stillCurrent()) return;
    await options.operations?.refreshOperations(repoId, worktreeId);
  } catch (err) {
    if (stillCurrent()) {
      options.onFailure?.();
      options.setDialogError(parseGitManageError(err));
    }
  } finally {
    if (stillCurrent()) {
      options.operations?.clearActive();
      options.setSubmitting(false);
    }
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

  function isSameTarget(repoId: string, wtId: string | undefined): boolean {
    return repoId === deps.getRepoId() && wtId === deps.getWorktreeId?.();
  }

  async function refreshRemotes(summary?: string) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId?.();
    if (!repoId) return;
    const res = await repositoryRemotes(repoId).catch(() => null);
    if (!isSameTarget(repoId, wtId)) return;
    if (res) {
      deps.onRemotesUpdated(res.remotes, summary);
    } else if (summary) {
      deps.onRemotesUpdated([...deps.getRemotes()], summary);
    }
    deps.onRefreshRepositories?.();
  }

  async function runRemoteMutation(
    opKind: "remote_add" | "remote_update" | "remote_remove",
    progressText: string,
    exec: (
      repoId: string,
      clientRequestId: string,
    ) => Promise<{ remotes: Remote[] }>,
    onSuccess: (remotes: Remote[]) => void,
  ) {
    if (submitting) return;
    await executeTrackedGitMutation({
      getRepoId: deps.getRepoId,
      getWorktreeId: deps.getWorktreeId,
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
      dialogError = m.s7_remote_req_name_url();
      return;
    }
    if (
      containsRedactedRemoteMask(cleanFetch) ||
      containsRedactedRemoteMask(cleanPush)
    ) {
      dialogError = m.s7_remote_no_redacted_mask();
      return;
    }
    const isAdd = editMode === "add";
    await runRemoteMutation(
      isAdd ? "remote_add" : "remote_update",
      m.s7_remote_saving_phase(),
      (targetRepoId, clientRequestId) =>
        isAdd
          ? gitRemoteAdd(targetRepoId, {
              name: cleanName,
              fetchUrl: cleanFetch,
              pushUrl: cleanPush || undefined,
              clientRequestId,
            })
          : gitRemoteUpdate(targetRepoId, cleanName, {
              fetchUrl: cleanFetch,
              ...(cleanPush ? { pushUrl: cleanPush } : { clearPushUrl: true }),
              clientRequestId,
            }),
      (nextRemotes) => {
        editOpen = false;
        deps.onRemotesUpdated(
          nextRemotes,
          isAdd
            ? m.s7_remote_added_banner({ name: cleanName })
            : m.s7_remote_updated_banner({ name: cleanName }),
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
      m.s7_remote_removing_phase(),
      (targetRepoId, clientRequestId) =>
        gitRemoteRemove(targetRepoId, target.name, { clientRequestId }),
      (nextRemotes) => {
        removeTarget = null;
        deps.onRemotesUpdated(
          nextRemotes,
          m.s7_remote_removed_banner({ name: target.name }),
        );
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
