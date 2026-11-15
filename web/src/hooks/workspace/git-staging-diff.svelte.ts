import type { FileDiffMetadata } from "@pierre/diffs";
import { errorMessage } from "../../platform/error";
import { gitChanges, gitDiff } from "../../platform/git-api";
import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";
import type { GitDiff } from "../../contracts/schemas";
import {
  resolveCalculatedDiff,
  resolveHunksCalculatedDiff,
} from "../../domains/code/code-diff-client";
import { LatestRequest } from "../shared/remote-request.svelte";
import { createGitContextRunner } from "./git-context-cache";

type GitStagingReaderPhase =
  | "idle"
  | "loading"
  | "ready"
  | "binary"
  | "limited"
  | "error";

export interface GitStagingReaderState {
  phase: GitStagingReaderPhase;
  changeId: string;
  path: string;
  oldPath: string;
  leftEndpoint: string;
  rightEndpoint: string;
  fileDiff: FileDiffMetadata | null;
  rawDiff: GitDiff | null;
  reason: string;
}

function emptyStagingReaderState(): GitStagingReaderState {
  return {
    phase: "idle",
    changeId: "",
    path: "",
    oldPath: "",
    leftEndpoint: "",
    rightEndpoint: "",
    fileDiff: null,
    rawDiff: null,
    reason: "",
  };
}

export function formatStagingComparisonEndpoints(change: GitWorkingChange): {
  left: string;
  right: string;
} {
  if (change.group === "staged") {
    return {
      left: change.status === "A" ? "文件不存在 (HEAD)" : "HEAD",
      right: change.status === "D" ? "文件不存在 (index)" : "index",
    };
  }
  if (change.group === "untracked") {
    return {
      left: "文件不存在",
      right: "working tree",
    };
  }
  if (change.group === "conflicted") {
    return {
      left: "冲突基准 (ours/theirs)",
      right: "working tree",
    };
  }
  return {
    left: change.status === "A" ? "文件不存在 (index)" : "index",
    right: change.status === "D" ? "文件不存在 (working tree)" : "working tree",
  };
}

export function createGitStagingDiffController(deps: {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
}) {
  let state = $state.raw<GitStagingReaderState>(emptyStagingReaderState());
  const ctxRunner = createGitContextRunner();
  const reqTracker = new LatestRequest();

  function dispose() {
    reqTracker.dispose();
  }

  function resetTarget() {
    reqTracker.cancel();
    ctxRunner.reset();
    state = emptyStagingReaderState();
  }

  const seedContextId = ctxRunner.seed;
  const withContext = ctxRunner.withContext;

  async function loadForChange(change: GitWorkingChange | null) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!change) {
      reqTracker.cancel();
      state = emptyStagingReaderState();
      return;
    }

    const endpoints = formatStagingComparisonEndpoints(change);

    if (change.group === "conflicted") {
      reqTracker.cancel();
      state = {
        phase: "limited",
        changeId: change.id,
        path: change.path,
        oldPath: change.oldPath ?? "",
        leftEndpoint: endpoints.left,
        rightEndpoint: endpoints.right,
        fileDiff: null,
        rawDiff: null,
        reason:
          "该条目存在未解决的合并或变基冲突，不能直接暂存或伪装为可提交文件；请前往工作树终端解决冲突。",
      };
      return;
    }

    if (change.binary) {
      reqTracker.cancel();
      state = {
        phase: "binary",
        changeId: change.id,
        path: change.path,
        oldPath: change.oldPath ?? "",
        leftEndpoint: endpoints.left,
        rightEndpoint: endpoints.right,
        fileDiff: null,
        rawDiff: null,
        reason:
          change.degradeReason ||
          "二进制文件不展开文本行级差异，仅支持整文件暂存或取消暂存。",
      };
      return;
    }

    const req = reqTracker.begin();
    state = {
      phase: "loading",
      changeId: change.id,
      path: change.path,
      oldPath: change.oldPath ?? "",
      leftEndpoint: endpoints.left,
      rightEndpoint: endpoints.right,
      fileDiff: null,
      rawDiff: null,
      reason: "",
    };

    try {
      if (repoId && wtId) {
        const groupParam =
          change.group === "staged"
            ? "staged"
            : change.group === "untracked"
              ? "untracked"
              : "unstaged";

        const rawDiff = await withContext(
          repoId,
          wtId,
          req.signal,
          async (cid) => {
            const list = await gitChanges(
              repoId,
              {
                contextId: cid,
                worktreeId: wtId,
                group: groupParam,
                q: change.path,
              },
              req.signal,
            );
            const match = list.entries.find(
              (entry) =>
                entry.group === groupParam &&
                (entry.newPath === change.path ||
                  entry.oldPath === change.path),
            );
            if (!match) return null;
            return gitDiff(
              repoId,
              { contextId: cid, worktreeId: wtId, changeId: match.changeId },
              req.signal,
            );
          },
        );

        if (!req.current()) return;

        if (rawDiff) {
          if (
            rawDiff.kind === "image" ||
            rawDiff.old.kind === "binary" ||
            rawDiff.new.kind === "binary"
          ) {
            state = {
              ...state,
              phase: "binary",
              rawDiff,
              reason:
                rawDiff.reason ||
                change.degradeReason ||
                "二进制文件不展开文本行级差异，仅支持整文件暂存或取消暂存。",
            };
            return;
          }

          if (rawDiff.kind !== "text") {
            state = {
              ...state,
              phase: "limited",
              rawDiff,
              reason:
                rawDiff.reason ||
                change.degradeReason ||
                "文件超过内联差异上限或仅包含模式变更，仅支持整文件暂存。",
            };
            return;
          }

          const calc = await resolveCalculatedDiff(rawDiff, req.signal);
          if (!req.current()) return;
          if (calc.calculation === "ready" && calc.fileDiff) {
            state = {
              ...state,
              phase: "ready",
              rawDiff,
              fileDiff: calc.fileDiff,
              reason: "",
            };
            return;
          }
          state = {
            ...state,
            phase: calc.calculation === "error" ? "error" : "limited",
            rawDiff,
            reason: calc.calculationReason,
          };
          return;
        }
      }

      // Fallback when codeview change list doesn't match (e.g. >200 changed files or offline fixtures):
      if (change.hunks.length > 0) {
        const snapKey = `staging:${change.id}:${change.identityKey || change.hunks.length}`;
        const calc = await resolveHunksCalculatedDiff(
          snapKey,
          change.path,
          change.oldPath ?? "",
          change.status,
          change.hunks,
          req.signal,
        );
        if (!req.current()) return;
        if (calc.calculation === "ready" && calc.fileDiff) {
          state = {
            ...state,
            phase: "ready",
            fileDiff: calc.fileDiff,
            reason: "",
          };
          return;
        }
      }

      state = {
        ...state,
        phase: "limited",
        reason:
          change.degradeReason ||
          "当前条目仅包含重命名、模式变更或空内容变化，支持整文件暂存。",
      };
    } catch (err) {
      if (!req.reportable()) return;
      state = {
        ...state,
        phase: "error",
        reason: errorMessage(err),
      };
    }
  }

  return {
    get state() {
      return state;
    },
    dispose,
    resetTarget,
    seedContextId,
    loadForChange,
  };
}
export type GitStagingDiffController = ReturnType<
  typeof createGitStagingDiffController
>;
