import type {
  GitCommitOutcome,
  GitPrecedingCommit,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import {
  generateGitClientRequestId,
  isRecoverableNetworkFailure,
  mapGitOperationToCommitOutcome,
} from "../../domains/workspace/git-operations-model";
import { writeGitTargetMemory } from "../../domains/workspace/git-workbench-memory";
import { errorMessage } from "../../platform/error";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import {
  performGitCommitExecution,
  requestCommitPreviewModel,
} from "./git-workbench-actions";

export async function runOpenCommitPreview(options: {
  repoId: string;
  wtId: string;
  view: GitWorkbenchViewState;
  followUpPush: boolean;
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
      statusBanner: `提交预览读取失败：${errorMessage(err)}`,
    });
  }
}

export async function runConfirmCommitFlow(options: {
  repoId: string;
  wtId: string;
  confirmationId: string;
  current: GitWorkbenchViewState;
  signal: AbortSignal;
  operations?: GitOperationsController;
  clearWebmuxStaged: () => void;
  refreshStaging: () => Promise<void>;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  onRefreshRepositories?: () => void;
  onAfterCommitRefresh?: () => void;
  onCommitDraftCleared?: () => Promise<void> | void;
  onFollowUpPush?: (
    branch: string,
    precedingCommit: GitPrecedingCommit,
  ) => void;
}) {
  const preview = options.current.commitPreview;
  if (!preview) return;

  const clientRequestId = generateGitClientRequestId("commit");
  options.operations?.setPhase("running", "commit", "正在执行提交并校验快照…", {
    clientRequestId,
  });

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
            recovered.summary || "提交未完成，请检查钩子输出或仓库状态";
          options.patchView({
            commitPreview: { ...preview, dialogError: failMsg },
            statusBanner: `提交失败：${failMsg}`,
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
          dialogError:
            out.banner ||
            "暂存区或 HEAD 在预览后已被修改，请点击「重新生成提交预览」确认最新内容（草稿已保留）。",
        },
        statusBanner: out.banner || "提交预览已过期，请刷新预览后重试",
      });
      return;
    }

    if (out.busyOutcome) {
      options.patchView({
        commitPreview: {
          ...preview,
          dialogError: `${out.busyOutcome.summary}（${out.busyOutcome.reasons.join("；")}）`,
        },
        lastCommitOutcome: out.busyOutcome,
        statusBanner: out.busyOutcome.summary,
      });
      return;
    }

    const errMsg = out.banner || "提交未完成，请检查钩子输出或仓库状态";
    options.patchView({
      commitPreview: {
        ...preview,
        dialogError: errMsg,
      },
      statusBanner: `提交失败：${errMsg}`,
    });
  } finally {
    options.operations?.clearActive();
  }
}
