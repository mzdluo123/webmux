import { m } from "$lib/paraglide/messages.js";
import type { FileDiffMetadata } from "@pierre/diffs";
import { errorMessage } from "../../platform/error";
import { gitSnapshotRaw } from "../../platform/git-api";
import type { GitCommitFile } from "../../contracts/git-workbench-schemas";
import type { GitDiff, GitSnapshotSide } from "../../contracts/schemas";
import { resolveCalculatedDiff } from "../../domains/code/code-diff-client";
import { LatestRequest } from "../shared/remote-request.svelte";
import { createGitContextRunner } from "./git-context-cache";

type GitSnapshotDiffPhase =
  | "idle"
  | "loading"
  | "ready"
  | "binary"
  | "limited"
  | "error";

interface GitSnapshotDiffViewState {
  phase: GitSnapshotDiffPhase;
  leftLabel: string;
  rightLabel: string;
  baseOid: string;
  targetOid: string;
  path: string;
  oldPath: string;
  status: GitCommitFile["status"] | "";
  snapshotId: string;
  fileDiff: FileDiffMetadata | null;
  rawDiff: GitDiff | null;
  reason: string;
  downloadingSide: GitSnapshotSide | null;
}

export interface GitDiffPanelReader {
  readonly state: GitSnapshotDiffViewState;
  readSnapshotSide: (
    snapshotId: string,
    side: GitSnapshotSide,
    signal: AbortSignal,
  ) => Promise<Blob>;
  downloadSnapshotSide: (side: GitSnapshotSide) => Promise<void>;
  retryActiveDiff: () => void;
}

function emptySnapshotDiffState(): GitSnapshotDiffViewState {
  return {
    phase: "idle",
    leftLabel: "",
    rightLabel: "",
    baseOid: "",
    targetOid: "",
    path: "",
    oldPath: "",
    status: "",
    snapshotId: "",
    fileDiff: null,
    rawDiff: null,
    reason: "",
    downloadingSide: null,
  };
}

function isBinaryDiff(file: GitCommitFile, diff: GitDiff): boolean {
  const binaryKinds = new Set(["binary", "image", "pdf"]);
  return (
    file.binary ||
    diff.kind === "image" ||
    binaryKinds.has(diff.old.kind) ||
    binaryKinds.has(diff.new.kind)
  );
}

export function createGitSnapshotDiffReader(deps: {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  onStatusBanner: (message: string) => void;
}) {
  let state = $state.raw<GitSnapshotDiffViewState>(emptySnapshotDiffState());
  const ctxRunner = createGitContextRunner();
  const diffReq = new LatestRequest();

  function dispose() {
    diffReq.dispose();
  }

  function resetTarget() {
    diffReq.cancel();
    ctxRunner.reset();
    state = emptySnapshotDiffState();
  }

  function clearDiff() {
    diffReq.cancel();
    state = emptySnapshotDiffState();
  }

  async function loadFetchedDiff(options: {
    baseOid?: string;
    targetOid?: string;
    leftLabel: string;
    rightLabel: string;
    file: GitCommitFile | null;
    missingReason: string;
    fetchDiff: (
      contextId: string,
      signal: AbortSignal,
    ) => Promise<GitDiff | null>;
  }) {
    const repoId = deps.getRepoId();
    const worktreeId = deps.getWorktreeId();
    if (!repoId || !options.file) {
      diffReq.cancel();
      state = emptySnapshotDiffState();
      return;
    }

    const { file, leftLabel, rightLabel } = options;
    const filePath = file.newPath || file.oldPath;
    const req = diffReq.begin();
    state = {
      phase: "loading",
      leftLabel,
      rightLabel,
      baseOid: options.baseOid ?? "",
      targetOid: options.targetOid ?? "",
      path: filePath,
      oldPath: file.oldPath,
      status: file.status,
      snapshotId: "",
      fileDiff: null,
      rawDiff: null,
      reason: "",
      downloadingSide: null,
    };

    try {
      const diff = await ctxRunner.withContext(
        repoId,
        worktreeId,
        req.signal,
        (cid) => options.fetchDiff(cid, req.signal),
      );
      if (!req.current() || repoId !== deps.getRepoId()) return;

      if (!diff) {
        state = { ...state, phase: "limited", reason: options.missingReason };
        return;
      }

      if (isBinaryDiff(file, diff)) {
        state = {
          ...state,
          phase: "binary",
          snapshotId: diff.snapshotId,
          rawDiff: diff,
          reason: diff.reason || m.s7_diff_binary_reason(),
        };
        return;
      }

      if (diff.kind !== "text") {
        state = {
          ...state,
          phase: "limited",
          snapshotId: diff.snapshotId,
          rawDiff: diff,
          reason: diff.reason || m.s7_snapshot_large_or_meta_reason(),
        };
        return;
      }

      const calculated = await resolveCalculatedDiff(diff, req.signal);
      if (!req.current() || repoId !== deps.getRepoId()) return;

      if (calculated.calculation === "ready" && calculated.fileDiff) {
        state = {
          ...state,
          phase: "ready",
          snapshotId: diff.snapshotId,
          rawDiff: diff,
          fileDiff: calculated.fileDiff,
          reason: "",
        };
      } else {
        state = {
          ...state,
          phase: calculated.calculation === "error" ? "error" : "limited",
          snapshotId: diff.snapshotId,
          rawDiff: diff,
          reason: calculated.calculationReason,
        };
      }
    } catch (err) {
      if (!req.reportable()) return;
      state = { ...state, phase: "error", reason: errorMessage(err) };
    }
  }

  function readSnapshotSide(
    snapshotId: string,
    side: GitSnapshotSide,
    signal: AbortSignal,
    download = false,
  ) {
    const repoId = deps.getRepoId();
    const worktreeId = deps.getWorktreeId();
    return ctxRunner.withContext(repoId, worktreeId, signal, (contextId) =>
      gitSnapshotRaw(
        repoId,
        snapshotId,
        { side, contextId, worktreeId, download },
        signal,
      ),
    );
  }

  async function downloadSnapshotSide(side: GitSnapshotSide) {
    const repoId = deps.getRepoId();
    const snapshotId = state.snapshotId;
    if (!repoId || !snapshotId || state.downloadingSide) return;

    state = { ...state, downloadingSide: side };
    const ctrl = new AbortController();
    try {
      const blob = await readSnapshotSide(snapshotId, side, ctrl.signal, true);
      const baseName =
        (side === "before" ? state.oldPath || state.path : state.path)
          .split("/")
          .pop() || `snapshot-${side}`;
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = baseName;
      anchor.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (err) {
      deps.onStatusBanner(
        m.s7_snapshot_download_failed({ error: errorMessage(err) }),
      );
    } finally {
      if (state.snapshotId === snapshotId) {
        state = { ...state, downloadingSide: null };
      }
    }
  }

  return {
    get state() {
      return state;
    },
    dispose,
    resetTarget,
    clearDiff,
    seedContextId: ctxRunner.seed,
    withContext: ctxRunner.withContext,
    loadFetchedDiff,
    readSnapshotSide,
    downloadSnapshotSide,
  };
}
