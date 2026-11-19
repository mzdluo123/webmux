import { m } from "$lib/paraglide/messages.js";
import {
  gitStashApplyPreview,
  gitStashCreatePreview,
  gitStashDropPreview,
} from "../../platform/git-stash-api";
import type {
  GitInProgressOperation,
  GitStashApplyPreview,
  GitStashCreatePreview,
  GitStashDropPreview,
  GitStashEntry,
} from "../../contracts/schemas";
import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import {
  parseGitManageError,
  runLatestGitPreview,
} from "./git-remotes-controller.svelte";
import {
  canConfirmStashApply,
  canConfirmStashCreate,
  canConfirmStashDrop,
  executeStashApply,
  executeStashCreate,
  executeStashDrop,
} from "./git-stash-dialogs-actions";

export { parseGitManageError as parseStashApiError };

export interface GitStashDialogsDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  operations?: GitOperationsController;
  onRefreshList: (preferredOid?: string | null) => Promise<void>;
  onAfterStashMutation: () => Promise<void>;
  onInProgressUpdated: (next: GitInProgressOperation) => void;
  onOpenConflictResolver: (path?: string) => void;
}

export function createGitStashDialogsController(deps: GitStashDialogsDeps) {
  let createOpen = $state(false);
  let createMessage = $state("");
  let createIncludeUntracked = $state(false);
  let createKeepIndex = $state(false);
  let createPreview = $state<GitStashCreatePreview | null>(null);

  let applyOpen = $state(false);
  let applyMode = $state<"apply" | "pop">("pop");
  let applyReinstateIndex = $state(false);
  let applyTarget = $state<GitStashEntry | null>(null);
  let applyPreview = $state<GitStashApplyPreview | null>(null);

  let dropOpen = $state(false);
  let dropTarget = $state<GitStashEntry | null>(null);
  let dropPreview = $state<GitStashDropPreview | null>(null);

  let loadingPreview = $state(false);
  let submitting = $state(false);
  let acknowledgeSessions = $state(false);
  let dialogError = $state("");

  const previewReq = new LatestRequest();
  let createMessageTimer: ReturnType<typeof setTimeout> | null = null;
  let disposed = false;

  function clearCreateMessageTimer() {
    if (createMessageTimer !== null) {
      clearTimeout(createMessageTimer);
      createMessageTimer = null;
    }
  }

  function isSameTarget(repoId: string, wtId: string | undefined): boolean {
    return (
      !disposed && repoId === deps.getRepoId() && wtId === deps.getWorktreeId()
    );
  }

  function dispose() {
    disposed = true;
    clearCreateMessageTimer();
    previewReq.dispose();
  }

  function resetTarget() {
    clearCreateMessageTimer();
    previewReq.cancel();
    createOpen = false;
    createPreview = null;
    applyOpen = false;
    applyPreview = null;
    dropOpen = false;
    dropPreview = null;
    loadingPreview = false;
    submitting = false;
    acknowledgeSessions = false;
    dialogError = "";
  }

  async function refreshCreatePreview() {
    clearCreateMessageTimer();
    previewReq.cancel();
    createPreview = null;
    acknowledgeSessions = false;
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !wtId) {
      loadingPreview = false;
      dialogError = m.s7_stash_readonly_create_error();
      return;
    }
    const message = createMessage.trim() || undefined;
    const includeUntracked = createIncludeUntracked;
    const keepIndex = createKeepIndex;
    await runLatestGitPreview({
      tracker: previewReq,
      isActive: () => createOpen && isSameTarget(repoId, wtId),
      setLoading: (busy) => (loadingPreview = busy),
      setDialogError: (msg) => (dialogError = msg),
      load: (signal) =>
        gitStashCreatePreview(
          repoId,
          wtId,
          { message, includeUntracked, keepIndex },
          signal,
        ),
      onLoaded: (res) => {
        createPreview = res;
      },
      onCleared: () => {
        createPreview = null;
        acknowledgeSessions = false;
      },
    });
  }

  function openCreateDialog(initialUntracked = false) {
    clearCreateMessageTimer();
    createOpen = true;
    createMessage = "";
    createIncludeUntracked = initialUntracked;
    createKeepIndex = false;
    createPreview = null;
    acknowledgeSessions = false;
    dialogError = "";
    void refreshCreatePreview();
  }

  async function confirmCreate() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const check = {
      createOpen,
      repoId,
      wtId,
      preview: createPreview,
      loadingPreview,
      submitting,
      hasPendingDebounce: createMessageTimer !== null,
      includeUntracked: createIncludeUntracked,
      keepIndex: createKeepIndex,
      trimmedMessage: createMessage.trim(),
      acknowledgeSessions,
    };
    if (!canConfirmStashCreate(check)) return;
    await executeStashCreate({
      repoId,
      wtId: check.wtId,
      confirmationId: check.preview.confirmationId,
      acknowledgeSessions,
      isSameTarget: () => isSameTarget(repoId, check.wtId),
      operations: deps.operations,
      patchView: deps.patchView,
      onRefreshList: deps.onRefreshList,
      onAfterStashMutation: deps.onAfterStashMutation,
      setSubmitting: (busy) => (submitting = busy),
      setDialogError: (msg) => (dialogError = msg),
      onSuccessClose: () => {
        createOpen = false;
        createPreview = null;
        acknowledgeSessions = false;
      },
      onInvalidatePreview: () => {
        createPreview = null;
        acknowledgeSessions = false;
      },
    });
  }

  async function refreshApplyPreview() {
    previewReq.cancel();
    applyPreview = null;
    acknowledgeSessions = false;
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const target = applyTarget;
    if (!repoId || !wtId || !target) {
      loadingPreview = false;
      dialogError = m.s7_stash_select_writable_apply();
      return;
    }
    await runLatestGitPreview({
      tracker: previewReq,
      isActive: () => applyOpen && isSameTarget(repoId, wtId),
      setLoading: (busy) => (loadingPreview = busy),
      setDialogError: (msg) => (dialogError = msg),
      load: (signal) =>
        gitStashApplyPreview(
          repoId,
          wtId,
          {
            stashOid: target.oid,
            stashRef: target.stashRef,
            mode: applyMode,
            reinstateIndex: applyReinstateIndex,
          },
          signal,
        ),
      onLoaded: (res) => {
        applyPreview = res;
      },
      onCleared: () => {
        applyPreview = null;
        acknowledgeSessions = false;
      },
    });
  }

  function openApplyDialog(entry: GitStashEntry, mode: "apply" | "pop") {
    applyTarget = entry;
    applyMode = mode;
    applyReinstateIndex = false;
    applyPreview = null;
    acknowledgeSessions = false;
    dialogError = "";
    applyOpen = true;
    void refreshApplyPreview();
  }

  async function confirmApply() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const check = {
      applyOpen,
      repoId,
      wtId,
      preview: applyPreview,
      target: applyTarget,
      mode: applyMode,
      reinstateIndex: applyReinstateIndex,
      loadingPreview,
      submitting,
      acknowledgeSessions,
    };
    if (!canConfirmStashApply(check)) return;
    await executeStashApply({
      repoId,
      wtId: check.wtId,
      mode: applyMode,
      confirmationId: check.preview.confirmationId,
      acknowledgeSessions,
      isSameTarget: () => isSameTarget(repoId, check.wtId),
      operations: deps.operations,
      patchView: deps.patchView,
      onRefreshList: deps.onRefreshList,
      onAfterStashMutation: deps.onAfterStashMutation,
      onInProgressUpdated: deps.onInProgressUpdated,
      onOpenConflictResolver: deps.onOpenConflictResolver,
      setSubmitting: (busy) => (submitting = busy),
      setDialogError: (msg) => (dialogError = msg),
      onSuccessClose: () => {
        applyOpen = false;
        applyPreview = null;
        acknowledgeSessions = false;
      },
      onInvalidatePreview: () => {
        applyPreview = null;
        acknowledgeSessions = false;
      },
    });
  }

  async function openDropDialog(entry: GitStashEntry) {
    clearCreateMessageTimer();
    previewReq.cancel();
    const repoId = deps.getRepoId();
    if (!repoId) return;
    dropTarget = entry;
    dropPreview = null;
    acknowledgeSessions = false;
    dialogError = "";
    dropOpen = true;
    await runLatestGitPreview({
      tracker: previewReq,
      isActive: () => !disposed && dropOpen && repoId === deps.getRepoId(),
      setLoading: (busy) => (loadingPreview = busy),
      setDialogError: (msg) => (dialogError = msg),
      load: (signal) =>
        gitStashDropPreview(
          repoId,
          { stashOid: entry.oid, stashRef: entry.stashRef },
          signal,
        ),
      onLoaded: (res) => {
        dropPreview = res;
      },
      onCleared: () => {
        dropPreview = null;
      },
    });
  }

  async function confirmDrop() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const check = {
      dropOpen,
      repoId,
      preview: dropPreview,
      target: dropTarget,
      loadingPreview,
      submitting,
    };
    if (!canConfirmStashDrop(check)) return;
    await executeStashDrop({
      repoId,
      wtId,
      confirmationId: check.preview.confirmationId,
      isSameRepo: () => !disposed && repoId === deps.getRepoId(),
      isSameTarget: () => isSameTarget(repoId, wtId),
      getLatestWorktreeId: deps.getWorktreeId,
      operations: deps.operations,
      patchView: deps.patchView,
      onRefreshList: () => deps.onRefreshList(),
      setSubmitting: (busy) => (submitting = busy),
      setDialogError: (msg) => (dialogError = msg),
      onSuccessClose: () => {
        dropOpen = false;
        dropPreview = null;
      },
      onInvalidatePreview: () => {
        dropPreview = null;
      },
    });
  }

  function closeDialogs() {
    if (submitting) return;
    clearCreateMessageTimer();
    previewReq.cancel();
    loadingPreview = false;
    createOpen = false;
    createPreview = null;
    applyOpen = false;
    applyPreview = null;
    dropOpen = false;
    dropPreview = null;
    acknowledgeSessions = false;
    dialogError = "";
  }

  return {
    get createOpen() {
      return createOpen;
    },
    get createMessage() {
      return createMessage;
    },
    setCreateMessage(val: string) {
      if (submitting || val === createMessage) return;
      createMessage = val;
      clearCreateMessageTimer();
      previewReq.cancel();
      createPreview = null;
      acknowledgeSessions = false;
      dialogError = "";
      if (!deps.getRepoId() || !deps.getWorktreeId()) {
        loadingPreview = false;
        dialogError = m.s7_stash_readonly_create_error();
        return;
      }
      loadingPreview = true;
      createMessageTimer = setTimeout(() => {
        createMessageTimer = null;
        void refreshCreatePreview();
      }, 250);
    },
    get createIncludeUntracked() {
      return createIncludeUntracked;
    },
    setCreateIncludeUntracked(val: boolean) {
      if (submitting || val === createIncludeUntracked) return;
      createIncludeUntracked = val;
      void refreshCreatePreview();
    },
    get createKeepIndex() {
      return createKeepIndex;
    },
    setCreateKeepIndex(val: boolean) {
      if (submitting || val === createKeepIndex) return;
      createKeepIndex = val;
      void refreshCreatePreview();
    },
    get createPreview() {
      return createPreview;
    },
    get applyOpen() {
      return applyOpen;
    },
    get applyMode() {
      return applyMode;
    },
    setApplyMode(mode: "apply" | "pop") {
      if (submitting || mode === applyMode) return;
      applyMode = mode;
      void refreshApplyPreview();
    },
    get applyReinstateIndex() {
      return applyReinstateIndex;
    },
    setApplyReinstateIndex(val: boolean) {
      if (submitting || val === applyReinstateIndex) return;
      applyReinstateIndex = val;
      void refreshApplyPreview();
    },
    get applyTarget() {
      return applyTarget;
    },
    get applyPreview() {
      return applyPreview;
    },
    get dropOpen() {
      return dropOpen;
    },
    get dropTarget() {
      return dropTarget;
    },
    get dropPreview() {
      return dropPreview;
    },
    get loadingPreview() {
      return loadingPreview;
    },
    get submitting() {
      return submitting;
    },
    get acknowledgeSessions() {
      return acknowledgeSessions;
    },
    setAcknowledgeSessions(val: boolean) {
      acknowledgeSessions = val;
    },
    get dialogError() {
      return dialogError;
    },
    dispose,
    resetTarget,
    openCreateDialog,
    refreshCreatePreview,
    confirmCreate,
    openApplyDialog,
    refreshApplyPreview,
    confirmApply,
    openDropDialog,
    confirmDrop,
    closeDialogs,
  };
}
