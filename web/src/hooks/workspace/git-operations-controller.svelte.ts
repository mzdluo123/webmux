import { SvelteSet } from "svelte/reactivity";
import { fetchRepositoryRemotes } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { gitOperation, gitOperations } from "../../platform/git-api";
import type {
  GitActiveOperationPhase,
  GitActiveOperationState,
  GitCommitOutcome,
  GitOperation,
  GitOperationKind,
} from "../../contracts/git-workbench-schemas";
import type {
  Repository,
  SessionInfo,
  Worktree,
} from "../../contracts/schemas";
import {
  findRestorableAttentionOperation,
  generateGitClientRequestId,
  gitOperationKindLabel,
  isRecoverableNetworkFailure,
  isTerminalGitOperationStatus,
  mapGitOperationToCommitOutcome,
} from "../../domains/workspace/git-operations-model";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface GitOperationsControllerDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  onOutcomeRestored: (outcome: GitCommitOutcome | null) => void;
  onStatusBanner: (banner: string) => void;
  onRepositoryUpdated?: (repo: Repository) => void;
  onReloadTarget?: (
    repo?: Repository,
    wt?: Worktree,
    sessions?: readonly SessionInfo[],
  ) => Promise<void>;
}

const POLL_DELAYS_MS = [150, 300, 600, 1000, 1500, 2000];

function delayMs(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const timer = setTimeout(resolve, ms);
    signal?.addEventListener(
      "abort",
      () => {
        clearTimeout(timer);
        resolve();
      },
      { once: true },
    );
  });
}

export function createGitOperationsController(
  deps: GitOperationsControllerDeps,
) {
  let operations = $state<GitOperation[]>([]);
  let active = $state<GitActiveOperationState | null>(null);
  let fetching = $state(false);
  const acknowledgedIds = new SvelteSet<string>();
  const listReq = new LatestRequest();
  let disposed = false;

  function dispose() {
    disposed = true;
    listReq.dispose();
  }

  function resetTarget() {
    listReq.cancel();
    operations = [];
    active = null;
  }

  function setPhase(
    phase: GitActiveOperationPhase,
    kind: GitOperationKind,
    summary: string,
    extra: { operationId?: string; clientRequestId?: string } = {},
  ) {
    if (phase === "idle") {
      active = null;
      return;
    }
    active = {
      phase,
      kind,
      operationId: extra.operationId,
      clientRequestId: extra.clientRequestId,
      summary,
    };
  }

  function clearActive() {
    active = null;
  }

  async function refreshOperations(
    repoId = deps.getRepoId(),
    wtId = deps.getWorktreeId(),
    signal?: AbortSignal,
  ): Promise<GitOperation[]> {
    if (!repoId || disposed) return [];
    const req = listReq.begin();
    const effectiveSignal = signal ?? req.signal;
    try {
      const res = await gitOperations(
        repoId,
        { worktreeId: wtId, limit: 20 },
        effectiveSignal,
      );
      if (
        !req.current() ||
        disposed ||
        effectiveSignal.aborted ||
        repoId !== deps.getRepoId()
      ) {
        return res.operations;
      }
      operations = res.operations;
      const inFlight = res.operations.find(
        (op) => op.status === "pending" || op.status === "running",
      );
      if (inFlight && !active) {
        active = {
          phase: inFlight.status === "pending" ? "validating" : "running",
          kind: inFlight.kind,
          operationId: inFlight.id,
          clientRequestId: inFlight.clientRequestId,
          summary:
            inFlight.summary ||
            `正在执行${gitOperationKindLabel(inFlight.kind)}…`,
        };
      }
      const attention = findRestorableAttentionOperation(
        res.operations,
        acknowledgedIds,
      );
      if (attention) {
        const mapped = mapGitOperationToCommitOutcome(attention);
        if (mapped) deps.onOutcomeRestored(mapped);
      }
      return res.operations;
    } catch {
      return operations;
    }
  }

  function acknowledgeAttention(operationId?: string) {
    if (operationId) {
      acknowledgedIds.add(operationId);
    } else {
      for (const op of operations) {
        if (op.status === "uncertain" || op.status === "needs_review") {
          acknowledgedIds.add(op.id);
        }
      }
    }
  }

  const hasUncertainUnresolved = $derived.by(() =>
    operations.some(
      (op) => op.status === "uncertain" && !acknowledgedIds.has(op.id),
    ),
  );

  /**
   * Queries recent/specific operations after a timeout or network drop to recover
   * an already-recorded operation without blind re-execution.
   */
  async function recoverByClientRequest(options: {
    repositoryId: string;
    worktreeId?: string;
    kind: GitOperationKind;
    clientRequestId: string;
    operationId?: string;
    signal?: AbortSignal;
  }): Promise<GitOperation | null> {
    setPhase(
      "reconciling",
      options.kind,
      `连接中断，正在核对${gitOperationKindLabel(options.kind)}执行状态…`,
      {
        operationId: options.operationId,
        clientRequestId: options.clientRequestId,
      },
    );

    for (const wait of POLL_DELAYS_MS) {
      if (disposed || options.signal?.aborted) break;
      try {
        let matched: GitOperation | undefined;
        if (options.operationId) {
          matched = await gitOperation(
            options.repositoryId,
            options.operationId,
            options.signal,
          );
        } else {
          const res = await gitOperations(
            options.repositoryId,
            { worktreeId: options.worktreeId, limit: 20 },
            options.signal,
          );
          if (options.repositoryId === deps.getRepoId()) {
            operations = res.operations;
          }
          matched = res.operations.find(
            (op) => op.clientRequestId === options.clientRequestId,
          );
        }
        if (matched) {
          if (isTerminalGitOperationStatus(matched.status)) {
            await refreshOperations(
              options.repositoryId,
              options.worktreeId,
              options.signal,
            );
            clearActive();
            return matched;
          }
          setPhase(
            "running",
            options.kind,
            matched.summary ||
              `服务端正在完成${gitOperationKindLabel(options.kind)}…`,
            {
              operationId: matched.id,
              clientRequestId: options.clientRequestId,
            },
          );
        }
      } catch {
        /* Network may still be reconnecting; wait for next poll */
      }
      await delayMs(wait, options.signal);
    }
    clearActive();
    return null;
  }

  async function runFetch(
    wt?: Worktree,
    sessions: readonly SessionInfo[] = [],
  ) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || fetching) return;
    fetching = true;
    const clientRequestId = generateGitClientRequestId("fetch");
    setPhase("running", "fetch", "正在获取远程引用…", { clientRequestId });
    try {
      const res = await fetchRepositoryRemotes(repoId, {
        clientRequestId,
        worktreeId: wtId,
      });
      deps.onRepositoryUpdated?.(res.repository);
      const okCount = res.results.filter((r) => r.status === "ok").length;
      const failCount = res.results.length - okCount;
      const banner =
        failCount === 0
          ? `已获取全部远程（${okCount} 个）`
          : `远程获取完成：成功 ${okCount} 个，失败 ${failCount} 个`;
      deps.onStatusBanner(banner);
      const nextTree = wt?.id
        ? res.repository.worktrees.find((w) => w.id === wt.id)
        : wt;
      await deps.onReloadTarget?.(res.repository, nextTree, sessions);
      await refreshOperations(repoId, wtId);
    } catch (err) {
      if (isRecoverableNetworkFailure(err)) {
        const recovered = await recoverByClientRequest({
          repositoryId: repoId,
          worktreeId: wtId,
          kind: "fetch",
          clientRequestId,
        });
        if (recovered && recovered.status === "succeeded") {
          deps.onStatusBanner(recovered.summary || "已恢复远程获取结果");
          await deps.onReloadTarget?.(undefined, wt, sessions);
          return;
        }
      }
      deps.onStatusBanner(`获取远程失败：${errorMessage(err)}`);
      await refreshOperations(repoId, wtId);
    } finally {
      clearActive();
      fetching = false;
    }
  }

  return {
    get operations() {
      return operations;
    },
    get active() {
      return active;
    },
    get fetching() {
      return fetching;
    },
    get hasUncertainUnresolved() {
      return hasUncertainUnresolved;
    },
    dispose,
    resetTarget,
    setPhase,
    clearActive,
    refreshOperations,
    acknowledgeAttention,
    recoverByClientRequest,
    runFetch,
  };
}

export type GitOperationsController = ReturnType<
  typeof createGitOperationsController
>;
