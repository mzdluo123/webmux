import type { FileDiffMetadata } from "@pierre/diffs";
import { APIError } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { gitCompare, gitContext } from "../../platform/git-api";
import type { GitCommitFile } from "../../contracts/git-workbench-schemas";
import { resolveCalculatedDiff } from "../../domains/code/code-diff-client";
import { LatestRequest } from "../shared/remote-request.svelte";

export interface GitPushFileDiffState {
  phase: "idle" | "loading" | "ready" | "binary" | "limited" | "error";
  path: string;
  oldPath: string;
  status: string;
  leftLabel: string;
  rightLabel: string;
  fileDiff: FileDiffMetadata | null;
  reason: string;
}

function emptyPushFileDiff(): GitPushFileDiffState {
  return {
    phase: "idle",
    path: "",
    oldPath: "",
    status: "",
    leftLabel: "",
    rightLabel: "",
    fileDiff: null,
    reason: "",
  };
}

export function createGitPushDiffController(deps: {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  isOpen: () => boolean;
}) {
  let state = $state.raw<GitPushFileDiffState>(emptyPushFileDiff());
  let cachedContextId = "";
  const diffReq = new LatestRequest();

  function dispose() {
    diffReq.dispose();
  }

  function reset() {
    diffReq.cancel();
    cachedContextId = "";
    state = emptyPushFileDiff();
  }

  async function ensureContextId(
    repoId: string,
    wtId: string | undefined,
    signal: AbortSignal,
    force = false,
  ): Promise<string> {
    if (!force && cachedContextId) return cachedContextId;
    const ctx = await gitContext(repoId, wtId, signal);
    cachedContextId = ctx.contextId;
    return cachedContextId;
  }

  async function loadFileDiff(
    commitOid: string,
    parentOid: string,
    file: GitCommitFile | null,
  ) {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    if (!repoId || !commitOid || !file) {
      diffReq.cancel();
      state = emptyPushFileDiff();
      return;
    }

    const filePath = file.newPath || file.oldPath;
    const leftLabel = parentOid ? parentOid.slice(0, 7) : "空树";
    const rightLabel = commitOid.slice(0, 7);

    if (file.binary) {
      diffReq.cancel();
      state = {
        phase: "binary",
        path: filePath,
        oldPath: file.oldPath,
        status: file.status,
        leftLabel,
        rightLabel,
        fileDiff: null,
        reason: "二进制文件不直接展开文本行级差异。",
      };
      return;
    }

    const req = diffReq.begin();
    state = {
      phase: "loading",
      path: filePath,
      oldPath: file.oldPath,
      status: file.status,
      leftLabel,
      rightLabel,
      fileDiff: null,
      reason: "",
    };

    try {
      const runCompare = async (forceCtx: boolean) => {
        const cid = await ensureContextId(repoId, wtId, req.signal, forceCtx);
        return gitCompare(
          repoId,
          {
            contextId: cid,
            worktreeId: wtId,
            baseOid: parentOid || undefined,
            targetOid: commitOid,
            path: filePath,
          },
          req.signal,
        );
      };

      let cmp;
      try {
        cmp = await runCompare(false);
      } catch (err) {
        if (
          !req.signal.aborted &&
          err instanceof APIError &&
          (err.status === 409 || err.status === 410)
        ) {
          cmp = await runCompare(true);
        } else {
          throw err;
        }
      }

      if (!req.current() || !deps.isOpen()) return;
      const diff = cmp.diff ?? null;
      if (!diff) {
        state = {
          ...state,
          phase: "limited",
          reason: "未找到该路径对应的历史对象快照。",
        };
        return;
      }
      if (
        diff.kind === "image" ||
        diff.old.kind === "binary" ||
        diff.new.kind === "binary"
      ) {
        state = {
          ...state,
          phase: "binary",
          reason: diff.reason || "二进制文件不直接展开文本行级差异。",
        };
        return;
      }
      if (diff.kind !== "text") {
        state = {
          ...state,
          phase: "limited",
          reason: diff.reason || "文件超过内联差异上限或仅包含模式变更。",
        };
        return;
      }

      const calc = await resolveCalculatedDiff(diff, req.signal);
      if (!req.current() || !deps.isOpen()) return;
      if (calc.calculation === "ready" && calc.fileDiff) {
        state = {
          ...state,
          phase: "ready",
          fileDiff: calc.fileDiff,
          reason: "",
        };
      } else {
        state = {
          ...state,
          phase: calc.calculation === "error" ? "error" : "limited",
          reason: calc.calculationReason,
        };
      }
    } catch (err) {
      if (req.reportable()) {
        state = { ...state, phase: "error", reason: errorMessage(err) };
      }
    }
  }

  return {
    get state() {
      return state;
    },
    dispose,
    reset,
    loadFileDiff,
  };
}
