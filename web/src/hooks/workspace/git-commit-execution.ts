import { m } from "$lib/paraglide/messages.js";
import { gitCommitPreview } from "../../platform/git-api";
import type {
  GitCommitOutcome,
  GitPrecedingCommit,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import type { GitCommitPreviewRequest } from "../../contracts/schemas";
import {
  generateGitClientRequestId,
  isRecoverableNetworkFailure,
  mapGitOperationToCommitOutcome,
} from "../../domains/workspace/git-operations-model";
import { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import { errorMessage } from "../../platform/error";
import type { GitCommitOptionsController } from "./git-commit-options.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import {
  performGitCommitExecution,
  requestCommitPreviewModel,
} from "./git-workbench-actions";

export async function runSyncCommitPreflight(options: {
  repoId: string;
  wtId: string;
  signal: AbortSignal;
  isCurrent: () => boolean;
  commitOptions?: GitCommitOptionsController;
  getLatestView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
}) {
  try {
    const preview = await gitCommitPreview(
      options.repoId,
      options.wtId,
      options.commitOptions?.buildPreviewRequest(),
      options.signal,
    );
    if (!options.isCurrent() || options.signal.aborted) return;
    options.commitOptions?.ingestPreflightMetadata({
      headMessage: preview.headMessage,
      amendPushed: preview.amendPushed,
      signoffTrailer: preview.identity.signoffTrailer,
      committerName: preview.identity.committerName,
      committerEmail: preview.identity.committerEmail,
    });
    const latest = options.getLatestView();
    options.patchView({
      context: {
        ...latest.context,
        authorName: preview.identity.authorName,
        authorEmail: preview.identity.authorEmail,
        committerName: preview.identity.committerName,
        committerEmail: preview.identity.committerEmail,
        signEnabled: preview.identity.signEnabled,
        signFormat: preview.identity.signFormat ?? "",
        executableHooks: preview.executableHooks,
        operationInProgress:
          preview.operation ?? latest.context.operationInProgress,
      },
    });
  } catch {
    /* Readonly or transient worktree state keeps existing context */
  }
}

export async function runOpenCommitPreview(options: {
  repoId: string;
  wtId: string;
  view: GitWorkbenchViewState;
  followUpPush: boolean;
  previewRequest?: GitCommitPreviewRequest;
  signal: AbortSignal;
  isCurrent: () => boolean;
  setConfirmationId: (id: string) => void;
  getLatestView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
}) {
  try {
    const res = await requestCommitPreviewModel({
      repositoryId: options.repoId,
      worktreeId: options.wtId,
      repositoryName: options.view.context.repositoryName,
      workingChanges: options.view.workingChanges,
      followUpPush: options.followUpPush,
      previewRequest: options.previewRequest,
      signal: options.signal,
    });
    if (!options.isCurrent()) return;
    options.setConfirmationId(res.confirmationId);
    const latest = options.getLatestView();
    options.patchView({
      context: {
        ...latest.context,
        authorName: res.authorName,
        authorEmail: res.authorEmail,
        committerName: res.committerName,
        committerEmail: res.committerEmail,
        signEnabled: res.signEnabled,
        signFormat: res.signFormat ?? "",
        executableHooks: res.executableHooks,
        operationInProgress:
          res.operation ?? latest.context.operationInProgress,
      },
      commitPreview: res.model,
    });
  } catch (err) {
    options.patchView({
      statusBanner: m.s7_commit_preview_load_failed({
        error: errorMessage(err),
      }),
    });
  }
}

export async function runConfirmCommitFlow(options: {
  repoId: string;
  wtId: string;
  confirmationId: string;
  current: GitWorkbenchViewState;
  acknowledgePushedAmend?: boolean;
  signal: AbortSignal;
  operations?: GitOperationsController;
  clearWebmuxStaged: () => void;
  refreshStaging: () => Promise<void>;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  onRefreshRepositories?: () => void;
  onAfterCommitRefresh?: () => void;
  onCommitDraftCleared?: () => Promise<void> | void;
  onCommitOptionsReset?: () => void;
  onFollowUpPush?: (
    branch: string,
    precedingCommit: GitPrecedingCommit,
  ) => void;
}) {
  const preview = options.current.commitPreview;
  if (!preview) return;

  const clientRequestId = generateGitClientRequestId("commit");
  options.operations?.setPhase(
    "running",
    "commit",
    m.s7_commit_running_phase(),
    { clientRequestId },
  );

  async function finalizeCommitOutcome(outcome: GitCommitOutcome) {
    const shouldClearDraft =
      outcome.status === "matched" || outcome.status === "needs_review";
    const nextMsg = shouldClearDraft ? "" : options.current.commitMessage;
    const subjectLine =
      options.current.commitMessage.trim().split(/\r?\n/, 1)[0] ?? "";
    writeGitTargetMemory(options.repoId, options.wtId, {
      commitMessage: nextMsg,
    });
    options.clearWebmuxStaged();
    if (shouldClearDraft) {
      options.onCommitOptionsReset?.();
    }
    options.patchView({
      commitMessage: nextMsg,
      commitPreview: null,
      lastCommitOutcome: outcome,
      statusBanner: outcome.summary,
    });
    await Promise.all([
      options.refreshStaging(),
      shouldClearDraft ? options.onCommitDraftCleared?.() : undefined,
    ]);
    await options.operations?.refreshOperations(options.repoId, options.wtId);
    options.onRefreshRepositories?.();
    options.onAfterCommitRefresh?.();

    if (
      preview?.followUpPush &&
      outcome.status === "matched" &&
      outcome.commitOid
    ) {
      options.onFollowUpPush?.(preview.branch, {
        commitOid: outcome.commitOid,
        shortOid: outcome.commitOid.slice(0, 7),
        subject: subjectLine,
      });
    }
  }

  try {
    const out = await performGitCommitExecution({
      repositoryId: options.repoId,
      worktreeId: options.wtId,
      confirmationId: options.confirmationId,
      clientRequestId,
      message: options.current.commitMessage,
      acknowledgePushedAmend: options.acknowledgePushedAmend,
      signal: options.signal,
    });

    if (out.ok) {
      await finalizeCommitOutcome(out.outcome);
      return;
    }

    if (
      options.operations &&
      out.rawError &&
      isRecoverableNetworkFailure(out.rawError)
    ) {
      const recovered = await options.operations.recoverByClientRequest({
        repositoryId: options.repoId,
        worktreeId: options.wtId,
        kind: "commit",
        clientRequestId,
        signal: options.signal,
      });
      if (recovered) {
        const mapped = mapGitOperationToCommitOutcome(recovered);
        if (mapped) {
          await finalizeCommitOutcome(mapped);
          return;
        }
        if (recovered.status === "failed") {
          const failMsg =
            recovered.summary || m.s7_commit_failed_default_reason();
          options.patchView({
            commitPreview: { ...preview, dialogError: failMsg },
            statusBanner: m.s7_commit_failed_banner({ error: failMsg }),
          });
          return;
        }
      }
    }

    await options.operations?.refreshOperations(options.repoId, options.wtId);

    if (out.reason === "stale") {
      options.patchView({
        commitPreview: {
          ...preview,
          stale: true,
          dialogError: out.banner || m.s7_commit_stale_dialog_error(),
        },
        statusBanner: out.banner || m.s7_commit_stale_banner(),
      });
      return;
    }

    if (out.busyOutcome) {
      options.patchView({
        commitPreview: {
          ...preview,
          dialogError: m.s7_busy_outcome_with_reasons({
            summary: out.busyOutcome.summary,
            reasons: out.busyOutcome.reasons.join(m.s7_reason_sep()),
          }),
        },
        lastCommitOutcome: out.busyOutcome,
        statusBanner: out.busyOutcome.summary,
      });
      return;
    }

    const errMsg = out.banner || m.s7_commit_failed_default_reason();
    options.patchView({
      commitPreview: {
        ...preview,
        dialogError: errMsg,
      },
      statusBanner: m.s7_commit_failed_banner({ error: errMsg }),
    });
  } finally {
    options.operations?.clearActive();
  }
}
