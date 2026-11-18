import { m } from "$lib/paraglide/messages.js";
import { errorMessage } from "../../platform/error";
import type { GitPrecedingCommit } from "../../contracts/git-workbench-schemas";
import type {
  PushPreview,
  PushResult,
  Repository,
} from "../../contracts/schemas";
import { branchOperations } from "../../domains/workspace/branch-operations";
import {
  generateGitClientRequestId,
  isRecoverableNetworkFailure,
} from "../../domains/workspace/git-operations-model";
import type { GitOperationsController } from "./git-operations-controller.svelte";

export function canConfirmPushPreview(options: {
  open: boolean;
  repoId: string;
  preview: PushPreview | null;
  running: boolean;
  loading: boolean;
  hasPendingDebounce: boolean;
  publishing: boolean;
  remote: string;
  target: string;
}): options is typeof options & { preview: PushPreview } {
  const cur = options.preview;
  if (
    !options.open ||
    !options.repoId ||
    !cur?.confirmationId ||
    cur.blockers.length > 0 ||
    options.running ||
    options.loading ||
    options.hasPendingDebounce
  ) {
    return false;
  }
  if (
    options.publishing &&
    (cur.remote !== options.remote || cur.target !== options.target.trim())
  ) {
    return false;
  }
  return true;
}

export async function runConfirmPushFlow(options: {
  repoId: string;
  wtId: string | undefined;
  preview: PushPreview;
  precedingCommit: GitPrecedingCommit | null;
  isOpen: () => boolean;
  isSameTarget: () => boolean;
  operations?: GitOperationsController;
  onRepositoryUpdated?: (repo: Repository) => void;
  onRefreshAfterPush?: () => void;
  onStatusBanner: (banner: string) => void;
  setRunning: (running: boolean) => void;
  setRejected: (rejected: PushResult | null) => void;
  setConflictError: (error: string) => void;
  onInvalidatePreview: () => void;
  onResetDialog: () => void;
  onRefreshPreview: () => void;
}) {
  const { repoId, wtId, preview: cur, precedingCommit } = options;
  options.setRunning(true);
  options.setRejected(null);
  options.setConflictError("");
  const clientRequestId = generateGitClientRequestId("push");
  const dest = `${cur.remote}/${cur.target}`;
  options.operations?.setPhase(
    "running",
    "push",
    m.s7_push_running_phase({ dest }),
    { clientRequestId },
  );
  try {
    const result = await branchOperations.push(repoId, cur.confirmationId, {
      clientRequestId,
      worktreeId: wtId,
    });
    options.onRepositoryUpdated?.(result.repository);
    if (!options.isSameTarget() || !options.isOpen()) return;
    options.onRefreshAfterPush?.();
    if (result.status === "ok") {
      const banner = precedingCommit
        ? cur.publish
          ? m.s7_push_committed_and_published({
              oid: precedingCommit.shortOid,
              dest,
            })
          : m.s7_push_committed_and_pushed({
              oid: precedingCommit.shortOid,
              dest,
            })
        : cur.publish
          ? m.s7_push_published_to({ dest })
          : m.s7_push_pushed_to({ dest });
      options.onResetDialog();
      options.onStatusBanner(banner);
      return;
    }

    options.onInvalidatePreview();
    options.setRejected(result);
    const failMsg = result.message || m.s7_push_remote_rejected();
    options.onStatusBanner(
      precedingCommit
        ? m.s7_push_failed_after_commit({
            oid: precedingCommit.shortOid,
            error: failMsg,
          })
        : m.s7_push_failed({ error: failMsg }),
    );
    options.onRefreshPreview();
  } catch (err) {
    if (options.isSameTarget()) {
      options.onInvalidatePreview();
    }
    if (options.operations && isRecoverableNetworkFailure(err)) {
      const recovered = await options.operations.recoverByClientRequest({
        repositoryId: repoId,
        worktreeId: wtId,
        kind: "push",
        clientRequestId,
      });
      if (!options.isSameTarget() || !options.isOpen()) return;
      if (recovered && recovered.status === "succeeded") {
        options.onRefreshAfterPush?.();
        options.onResetDialog();
        options.onStatusBanner(
          recovered.summary || m.s7_push_pushed_to({ dest }),
        );
        return;
      }
    }
    if (!options.isSameTarget() || !options.isOpen()) return;
    const msg = errorMessage(err);
    options.setConflictError(msg);
    options.onStatusBanner(
      precedingCommit
        ? m.s7_push_failed_after_commit({
            oid: precedingCommit.shortOid,
            error: msg,
          })
        : m.s7_push_failed({ error: msg }),
    );
    options.onRefreshPreview();
  } finally {
    if (options.isSameTarget()) {
      options.operations?.clearActive();
      options.setRunning(false);
      void options.operations?.refreshOperations(repoId, wtId);
    }
  }
}
