import type { FileDiffMetadata } from "@pierre/diffs";
import type { GitStagingHunk } from "../../contracts/schemas";
import { resolveHunksCalculatedDiff } from "../../domains/code/code-diff-client";

export interface OfflineHunkDiffState {
  phase: "idle" | "loading" | "ready" | "binary" | "limited" | "error";
  fileDiff: FileDiffMetadata | null;
  reason: string;
}

const IDLE_OFFLINE_DIFF: OfflineHunkDiffState = {
  phase: "idle",
  fileDiff: null,
  reason: "",
};

export function createOfflineHunkDiffState() {
  let current = $state.raw<OfflineHunkDiffState>(IDLE_OFFLINE_DIFF);

  function load(options: {
    disabled: boolean;
    hasTarget: boolean;
    binary: boolean;
    binaryReason: string;
    emptyReason: string;
    snapshotKey: string;
    path: string;
    oldPath: string;
    status: string;
    hunks: readonly GitStagingHunk[];
  }): (() => void) | void {
    if (options.disabled) return;
    if (!options.hasTarget) {
      current = IDLE_OFFLINE_DIFF;
      return;
    }
    if (options.binary) {
      current = {
        phase: "binary",
        fileDiff: null,
        reason: options.binaryReason,
      };
      return;
    }
    if (options.hunks.length === 0) {
      current = {
        phase: "limited",
        fileDiff: null,
        reason: options.emptyReason,
      };
      return;
    }
    const ctrl = new AbortController();
    current = { phase: "loading", fileDiff: null, reason: "" };
    void resolveHunksCalculatedDiff(
      options.snapshotKey,
      options.path,
      options.oldPath,
      options.status,
      options.hunks,
      ctrl.signal,
    ).then((res) => {
      if (ctrl.signal.aborted) return;
      current =
        res.calculation === "ready" && res.fileDiff
          ? { phase: "ready", fileDiff: res.fileDiff, reason: "" }
          : {
              phase: res.calculation === "error" ? "error" : "limited",
              fileDiff: null,
              reason: res.calculationReason,
            };
    });
    return () => ctrl.abort();
  }

  return {
    get state() {
      return current;
    },
    set(next: OfflineHunkDiffState) {
      current = next;
    },
    load,
  };
}
