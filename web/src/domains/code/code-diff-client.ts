import type { FileDiffMetadata, SelectedLineRange } from "@pierre/diffs";
import { locale } from "$lib/locale";
import * as m from "$lib/paraglide/messages.js";
import { api, APIError } from "../../platform/api";
import { DiffReplySchema, type DiffResult } from "./code-diff-model";
import { nodeId } from "./code-reference";
import {
  CodeChangesSchema,
  CodeDiffSchema,
  CodeFileSchema,
  type CodeDiff,
  type CodeFile,
  type GitDiff,
  type GitStagingHunk,
} from "../../contracts/schemas";

// One job per active document. Terminating the Worker cancels synchronous
// jsdiff work too, and prevents obsolete results from reaching the document.
export function calculateDiff(
  snapshotId: string,
  old: string,
  next: string,
  signal: AbortSignal,
): Promise<DiffResult> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new DOMException("Diff cancelled", "AbortError"));
      return;
    }
    let worker: Worker;
    try {
      worker = new Worker(new URL("./code-diff.worker.ts", import.meta.url), {
        type: "module",
      });
    } catch {
      reject(new Error(m.s5_diff_unable_to_start()));
      return;
    }
    let settled = false;
    const finish = (result?: DiffResult, error?: unknown) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      signal.removeEventListener("abort", abort);
      worker.onmessage = null;
      worker.onerror = null;
      worker.onmessageerror = null;
      worker.terminate();
      if (error) reject(error);
      else resolve(result!);
    };
    const abort = () =>
      finish(undefined, new DOMException("Diff cancelled", "AbortError"));
    const timer = setTimeout(
      () =>
        finish({
          kind: "limited",
          reason: m.s5_diff_timed_out(),
        }),
      10000,
    );
    signal.addEventListener("abort", abort, { once: true });
    worker.onmessage = (event: MessageEvent<unknown>) => {
      const reply = DiffReplySchema.safeParse(event.data);
      if (!reply.success || reply.data.snapshotId !== snapshotId) {
        finish(undefined, new Error(m.s5_diff_snapshot_mismatch()));
        return;
      }
      finish(reply.data.result);
    };
    worker.onerror = (event) => {
      event.preventDefault();
      finish(undefined, new Error(m.s5_diff_failed_view_full()));
    };
    worker.onmessageerror = () =>
      finish(undefined, new Error(m.s5_diff_unreadable_result()));
    try {
      worker.postMessage({
        snapshotId,
        old,
        new: next,
        locale: locale.current,
      });
    } catch (error) {
      finish(undefined, error);
    }
  });
}

function buildFileDiffMetadata(
  current: CodeDiff | GitDiff,
  fileDiff: FileDiffMetadata,
): FileDiffMetadata {
  return {
    ...fileDiff,
    name: current.new.path || current.old.path,
    prevName:
      current.old.path !== current.new.path ? current.old.path : undefined,
    type:
      current.old.source === "empty"
        ? "new"
        : current.new.source === "empty"
          ? "deleted"
          : current.old.path !== current.new.path
            ? fileDiff.hunks.length
              ? "rename-changed"
              : "rename-pure"
            : fileDiff.type,
    cacheKey: current.snapshotId,
  };
}

export async function fetchFileComparisonDiff(
  id: string,
  contextId: string,
  file: CodeFile,
  signal: AbortSignal,
  onState: (state: string) => void,
): Promise<CodeDiff> {
  const params = new URLSearchParams({
    contextId,
    nodeId: file.document.nodeId ?? nodeId(file.document.path),
  });
  const list = await api(
    `/sessions/${id}/code/changes?${params}`,
    CodeChangesSchema,
    "GET",
    undefined,
    signal,
  );
  const group = file.document.source !== "worktree" ? "staged" : "unstaged";
  const change = list.entries.find(
    (c) => c.group === group || c.group === "untracked",
  );
  if (!change) throw new Error(m.s5_no_uncommitted_diff());
  const value = await api(
    `/sessions/${id}/code/diff?${new URLSearchParams({ contextId, changeId: change.changeId })}`,
    CodeDiffSchema,
    "GET",
    undefined,
    signal,
  );
  if (signal.aborted) return value;
  const candidate = file.document.source === "head" ? value.old : value.new;
  if (candidate.contentVersion !== file.document.contentVersion) {
    onState("updated");
    throw new Error(m.s5_diff_source_updated_refresh());
  }
  return value;
}

export function sourceLabel(source: string) {
  return source === "worktree"
    ? m.s5_workspace()
    : source === "index"
      ? m.s5_source_index()
      : source === "head"
        ? m.s5_source_head()
        : source === "empty"
          ? m.s5_source_missing()
          : "";
}

export function resolveDiffSelectionRange(
  range: SelectedLineRange | null,
  defaultSide: "old" | "new",
): {
  side: "old" | "new";
  start?: number;
  end?: number;
  invalid: boolean;
  error: string;
} {
  if (!range) return { side: defaultSide, invalid: false, error: "" };
  if (range.endSide && range.endSide !== range.side) {
    return {
      side: defaultSide,
      invalid: true,
      error: m.s5_select_contiguous_same_side(),
    };
  }
  return {
    side: range.side === "deletions" ? "old" : "new",
    start: Math.min(range.start, range.end),
    end: Math.max(range.start, range.end),
    invalid: false,
    error: "",
  };
}

/** A missing new side and a HEAD file both initially show the old source. */
export function defaultDiffSide(
  diff?: CodeDiff,
  file?: CodeFile,
): "old" | "new" {
  return diff?.new.source === "empty" || file?.document.source === "head"
    ? "old"
    : "new";
}

export async function fetchSnapshotSideFile(
  id: string,
  snapshotId: string,
  which: "old" | "new",
  signal: AbortSignal,
  onState: (state: string) => void,
): Promise<CodeFile> {
  try {
    return await api(
      `/sessions/${id}/code/snapshots/${snapshotId}/file?side=${which}`,
      CodeFileSchema,
      "GET",
      undefined,
      signal,
    );
  } catch (e) {
    if (!signal.aborted && e instanceof APIError && e.status === 410)
      onState("expired");
    throw e;
  }
}

export type CalculatedDiffState = {
  modelSnapshot: string;
  calculation: "ready" | "limited" | "error";
  calculationReason: string;
  fileDiff?: FileDiffMetadata;
};

export async function resolveCalculatedDiff(
  current: CodeDiff | GitDiff,
  signal: AbortSignal,
): Promise<CalculatedDiffState> {
  try {
    const result = await calculateDiff(
      current.snapshotId,
      current.old.text,
      current.new.text,
      signal,
    );
    if (result.kind === "ready") {
      return {
        modelSnapshot: current.snapshotId,
        calculation: "ready",
        calculationReason: "",
        fileDiff: buildFileDiffMetadata(current, result.fileDiff),
      };
    }
    return {
      modelSnapshot: current.snapshotId,
      calculation: result.kind,
      calculationReason: result.reason,
    };
  } catch (error: unknown) {
    return {
      modelSnapshot: current.snapshotId,
      calculation: "error",
      calculationReason:
        error instanceof Error ? error.message : m.s5_diff_failed_view_full(),
    };
  }
}

/** Synthesizes old/new text from parsed diff hunks for offline fixture previews via the same Pierre Worker. */
export async function resolveHunksCalculatedDiff(
  snapshotId: string,
  path: string,
  oldPath: string,
  status: string,
  hunks: readonly GitStagingHunk[],
  signal: AbortSignal,
): Promise<CalculatedDiffState> {
  const oldLines: string[] = [];
  const newLines: string[] = [];
  for (const hunk of hunks) {
    for (const line of hunk.lines) {
      if (line.kind !== "add") oldLines.push(line.text);
      if (line.kind !== "delete") newLines.push(line.text);
    }
  }
  const oldText = oldLines.length > 0 ? `${oldLines.join("\n")}\n` : "";
  const newText = newLines.length > 0 ? `${newLines.join("\n")}\n` : "";
  try {
    const result = await calculateDiff(snapshotId, oldText, newText, signal);
    if (result.kind === "ready") {
      return {
        modelSnapshot: snapshotId,
        calculation: "ready",
        calculationReason: "",
        fileDiff: {
          ...result.fileDiff,
          name: path || oldPath,
          prevName: oldPath && oldPath !== path ? oldPath : undefined,
          type:
            status === "A"
              ? "new"
              : status === "D"
                ? "deleted"
                : oldPath && oldPath !== path
                  ? result.fileDiff.hunks.length
                    ? "rename-changed"
                    : "rename-pure"
                  : result.fileDiff.type,
          cacheKey: snapshotId,
        },
      };
    }
    return {
      modelSnapshot: snapshotId,
      calculation: result.kind,
      calculationReason: result.reason,
    };
  } catch (error: unknown) {
    return {
      modelSnapshot: snapshotId,
      calculation: "error",
      calculationReason:
        error instanceof Error ? error.message : m.s5_diff_failed_short(),
    };
  }
}
