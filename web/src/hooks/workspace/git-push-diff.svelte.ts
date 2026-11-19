import { m } from "$lib/paraglide/messages.js";
import type { FileDiffMetadata } from "@pierre/diffs";
import { errorMessage } from "../../platform/error";
import { gitCompare } from "../../platform/git-api";
import type { GitCommitFile } from "../../contracts/git-workbench-schemas";
import { resolveCalculatedDiff } from "../../domains/code/code-diff-client";
import { LatestRequest } from "../shared/remote-request.svelte";
import { createGitContextRunner } from "./git-context-cache";

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
  const ctxRunner = createGitContextRunner();
  const diffReq = new LatestRequest();

  function dispose() {
    diffReq.dispose();
  }

  function reset() {
    diffReq.cancel();
    ctxRunner.reset();
    state = emptyPushFileDiff();
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

    const isCurrentTarget = () =>
      deps.isOpen() &&
      repoId === deps.getRepoId() &&
      wtId === deps.getWorktreeId();

    const filePath = file.newPath || file.oldPath;
    const leftLabel = parentOid ? parentOid.slice(0, 7) : m.s7_empty_tree();
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
        reason: m.s7_diff_binary_reason(),
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
      const cmp = await ctxRunner.withContext(repoId, wtId, req.signal, (cid) =>
        gitCompare(
          repoId,
          {
            contextId: cid,
            worktreeId: wtId,
            baseOid: parentOid || undefined,
            targetOid: commitOid,
            path: filePath,
          },
          req.signal,
        ),
      );

      if (!req.current() || !isCurrentTarget()) return;
      const diff = cmp.diff ?? null;
      if (!diff) {
        state = {
          ...state,
          phase: "limited",
          reason: m.s7_snapshot_missing_reason(),
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
          reason: diff.reason || m.s7_diff_binary_reason(),
        };
        return;
      }
      if (diff.kind !== "text") {
        state = {
          ...state,
          phase: "limited",
          reason: diff.reason || m.s7_diff_limited_reason(),
        };
        return;
      }

      const calc = await resolveCalculatedDiff(diff, req.signal);
      if (!req.current() || !isCurrentTarget()) return;
      const ready = calc.calculation === "ready" && Boolean(calc.fileDiff);
      state = {
        ...state,
        phase: ready
          ? "ready"
          : calc.calculation === "error"
            ? "error"
            : "limited",
        fileDiff: ready ? (calc.fileDiff ?? null) : null,
        reason: ready ? "" : calc.calculationReason,
      };
    } catch (err) {
      if (req.reportable() && isCurrentTarget()) {
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
