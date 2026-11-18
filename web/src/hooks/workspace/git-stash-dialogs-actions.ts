import { m } from "$lib/paraglide/messages.js";
import {
  gitStashApply,
  gitStashCreate,
  gitStashDrop,
  gitStashPop,
} from "../../platform/git-stash-api";
import type {
  GitInProgressOperation,
  GitStashApplyPreview,
  GitStashCreatePreview,
  GitStashDropPreview,
  GitStashEntry,
} from "../../contracts/schemas";
import type {
  GitOperationKind,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import { generateGitClientRequestId } from "../../domains/workspace/git-operations-model";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import { parseGitManageError } from "./git-remotes-controller.svelte";

export function canConfirmStashCreate(options: {
  createOpen: boolean;
  repoId: string;
  wtId: string | undefined;
  preview: GitStashCreatePreview | null;
  loadingPreview: boolean;
  submitting: boolean;
  hasPendingDebounce: boolean;
  includeUntracked: boolean;
  keepIndex: boolean;
  trimmedMessage: string;
  acknowledgeSessions: boolean;
}): options is typeof options & {
  wtId: string;
  preview: GitStashCreatePreview;
} {
  const cur = options.preview;
  return Boolean(
    options.createOpen &&
    options.repoId &&
    options.wtId &&
    cur?.confirmationId &&
    !options.loadingPreview &&
    !options.submitting &&
    !options.hasPendingDebounce &&
    cur.includeUntracked === options.includeUntracked &&
    cur.keepIndex === options.keepIndex &&
    (cur.message === undefined || cur.message === options.trimmedMessage) &&
    cur.blockers.length === 0 &&
    (cur.sessions.length === 0 || options.acknowledgeSessions),
  );
}

export function canConfirmStashApply(options: {
  applyOpen: boolean;
  repoId: string;
  wtId: string | undefined;
  preview: GitStashApplyPreview | null;
  target: GitStashEntry | null;
  mode: "apply" | "pop";
  reinstateIndex: boolean;
  loadingPreview: boolean;
  submitting: boolean;
  acknowledgeSessions: boolean;
}): options is typeof options & {
  wtId: string;
  preview: GitStashApplyPreview;
} {
  const cur = options.preview;
  return Boolean(
    options.applyOpen &&
    options.repoId &&
    options.wtId &&
    cur?.confirmationId &&
    !options.loadingPreview &&
    !options.submitting &&
    cur.mode === options.mode &&
    cur.reinstateIndex === options.reinstateIndex &&
    (!options.target || cur.entry.oid === options.target.oid) &&
    cur.blockers.length === 0 &&
    (cur.sessions.length === 0 || options.acknowledgeSessions),
  );
}

export function canConfirmStashDrop(options: {
  dropOpen: boolean;
  repoId: string;
  preview: GitStashDropPreview | null;
  target: GitStashEntry | null;
  loadingPreview: boolean;
  submitting: boolean;
}): options is typeof options & { preview: GitStashDropPreview } {
  const cur = options.preview;
  return Boolean(
    options.dropOpen &&
    options.repoId &&
    cur?.confirmationId &&
    !options.loadingPreview &&
    !options.submitting &&
    (!options.target || cur.entry.oid === options.target.oid) &&
    cur.blockers.length === 0,
  );
}

interface StashDialogMutationHost {
  repoId: string;
  confirmationId: string;
  operations?: GitOperationsController;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  setSubmitting: (busy: boolean) => void;
  setDialogError: (msg: string) => void;
  onSuccessClose: () => void;
  onInvalidatePreview: () => void;
}

interface WorktreeStashMutationHost extends StashDialogMutationHost {
  wtId: string;
  acknowledgeSessions: boolean;
  isSameTarget: () => boolean;
  onRefreshList: (preferredOid?: string | null) => Promise<void>;
  onAfterStashMutation: () => Promise<void>;
}

async function runTrackedStashMutation<T>(
  host: StashDialogMutationHost,
  spec: {
    opKind: GitOperationKind;
    phaseLabel: string;
    getRefreshWorktreeId: () => string | undefined;
    isContextValid: () => boolean;
    shouldClearActive: () => boolean;
    exec: (clientRequestId: string) => Promise<T>;
    afterSuccess: (res: T) => Promise<void>;
  },
) {
  host.setSubmitting(true);
  host.setDialogError("");
  const clientRequestId = generateGitClientRequestId(spec.opKind);
  host.operations?.setPhase("running", spec.opKind, spec.phaseLabel, {
    clientRequestId,
  });
  try {
    const res = await spec.exec(clientRequestId);
    if (!spec.isContextValid()) return;
    await spec.afterSuccess(res);
    if (spec.isContextValid()) {
      await host.operations?.refreshOperations(
        host.repoId,
        spec.getRefreshWorktreeId(),
      );
    }
  } catch (err) {
    if (spec.isContextValid()) {
      host.onInvalidatePreview();
      host.setDialogError(parseGitManageError(err));
    }
  } finally {
    if (spec.shouldClearActive()) {
      host.operations?.clearActive();
    }
    if (spec.isContextValid()) {
      host.setSubmitting(false);
    }
  }
}

export async function executeStashCreate(options: WorktreeStashMutationHost) {
  await runTrackedStashMutation(options, {
    opKind: "stash_push",
    phaseLabel: m.s7_stash_saving_phase(),
    getRefreshWorktreeId: () => options.wtId,
    isContextValid: options.isSameTarget,
    shouldClearActive: options.isSameTarget,
    exec: (clientRequestId) =>
      gitStashCreate(options.repoId, options.wtId, {
        confirmationId: options.confirmationId,
        clientRequestId,
        acknowledgeSessions: options.acknowledgeSessions,
      }),
    afterSuccess: async (res) => {
      options.onSuccessClose();
      options.patchView({
        statusBanner: m.s7_stash_saved_banner({
          ref: res.entry.stashRef,
          oid: res.entry.shortOid,
          summary: res.entry.summary,
        }),
      });
      await Promise.all([
        options.onRefreshList(res.entry.oid),
        options.onAfterStashMutation(),
      ]);
    },
  });
}

export async function executeStashApply(
  options: WorktreeStashMutationHost & {
    mode: "apply" | "pop";
    onInProgressUpdated: (next: GitInProgressOperation) => void;
    onOpenConflictResolver: (path?: string) => void;
  },
) {
  const exec = options.mode === "pop" ? gitStashPop : gitStashApply;
  let openConflict: { open: boolean; path?: string } = { open: false };
  await runTrackedStashMutation(options, {
    opKind: options.mode === "pop" ? "stash_pop" : "stash_apply",
    phaseLabel:
      options.mode === "pop"
        ? m.s7_stash_popping_phase()
        : m.s7_stash_applying_phase(),
    getRefreshWorktreeId: () => options.wtId,
    isContextValid: options.isSameTarget,
    shouldClearActive: options.isSameTarget,
    exec: (clientRequestId) =>
      exec(options.repoId, options.wtId, {
        confirmationId: options.confirmationId,
        clientRequestId,
        acknowledgeSessions: options.acknowledgeSessions,
      }),
    afterSuccess: async (res) => {
      options.onSuccessClose();
      options.onInProgressUpdated(res.inProgress);
      options.patchView({ statusBanner: res.summary });
      const preferredOid =
        res.outcome === "needs_review" || res.stashPreserved
          ? res.stashOid
          : undefined;
      openConflict = {
        open:
          res.outcome === "conflicted" || res.inProgress.conflictedCount > 0,
        path: res.inProgress.conflictedFiles[0]?.path,
      };
      await Promise.all([
        options.onRefreshList(preferredOid),
        options.onAfterStashMutation(),
      ]);
    },
  });
  if (openConflict.open && options.isSameTarget()) {
    options.onOpenConflictResolver(openConflict.path);
  }
}

export async function executeStashDrop(
  options: StashDialogMutationHost & {
    wtId: string | undefined;
    isSameRepo: () => boolean;
    isSameTarget: () => boolean;
    getLatestWorktreeId: () => string | undefined;
    onRefreshList: () => Promise<void>;
  },
) {
  await runTrackedStashMutation(options, {
    opKind: "stash_drop",
    phaseLabel: m.s7_stash_dropping_phase(),
    getRefreshWorktreeId: options.getLatestWorktreeId,
    isContextValid: options.isSameRepo,
    shouldClearActive: options.isSameTarget,
    exec: (clientRequestId) =>
      gitStashDrop(options.repoId, {
        confirmationId: options.confirmationId,
        clientRequestId,
      }),
    afterSuccess: async (res) => {
      options.onSuccessClose();
      options.patchView({
        stashes: res.stashes,
        statusBanner: m.s7_stash_dropped_banner({
          ref: res.stashRef,
          cmd: res.recoveryCommand,
        }),
      });
      await options.onRefreshList();
    },
  });
}
