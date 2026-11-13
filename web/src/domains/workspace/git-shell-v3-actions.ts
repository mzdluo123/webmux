import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import type {
  GitDiscardPreview,
  GitLineSelection,
  SessionInfo,
} from "../../contracts/schemas";
import {
  applyOfflineDiscardChanges,
  countSelectedLines,
} from "./git-line-selection";

export interface PendingShellDiscard {
  scope: "files" | "hunks" | "lines";
  paths?: string[];
  changeId?: string;
  hunkId?: string;
  lineSelections?: GitLineSelection[];
}

export function buildShellDiscardPreview(
  view: GitWorkbenchViewState,
  pending: PendingShellDiscard,
  activeSessions: readonly SessionInfo[],
): GitDiscardPreview {
  const trackedOverwritePaths: string[] = [];
  const untrackedDeletePaths: string[] = [];
  const items: GitDiscardPreview["items"] = [];

  if (pending.scope === "files") {
    for (const p of pending.paths ?? []) {
      const entry = view.workingChanges.find(
        (c) => c.path === p && c.group !== "staged",
      );
      if (!entry) continue;
      if (entry.group === "untracked") {
        untrackedDeletePaths.push(p);
        items.push({ path: p, status: "?", action: "delete_untracked" });
      } else {
        trackedOverwritePaths.push(p);
        items.push({
          path: p,
          status: entry.status,
          action:
            entry.status === "D" ? "restore_deleted" : "overwrite_tracked",
        });
      }
    }
  } else if (pending.changeId) {
    const entry = view.workingChanges.find((c) => c.id === pending.changeId);
    if (entry) {
      trackedOverwritePaths.push(entry.path);
      items.push({
        path: entry.path,
        status: entry.status,
        action: "overwrite_tracked",
      });
    }
  }

  const hunkCount = pending.hunkId ? 1 : 0;
  const lineCount = pending.lineSelections
    ? countSelectedLines(pending.lineSelections)
    : 0;
  const summary =
    pending.scope === "lines"
      ? `将从 ${trackedOverwritePaths[0] ?? ""} 丢弃 ${lineCount} 行未暂存更改`
      : pending.scope === "hunks"
        ? `将从 ${trackedOverwritePaths[0] ?? ""} 丢弃 1 个变更块`
        : `将丢弃 ${items.length} 个未暂存路径更改`;

  return {
    confirmationId: "shell-discard-confirm",
    expiresAt: new Date(Date.now() + 300_000).toISOString(),
    repositoryId: view.context.repositoryId || "repo-demo",
    worktreeId: view.context.worktreeId || "wt-main",
    worktreePath: view.context.worktreePath || "/work/webmux",
    scope: pending.scope,
    trackedOverwritePaths,
    untrackedDeletePaths,
    items,
    hunkCount,
    lineCount,
    summary,
    sessions: [...activeSessions],
    blockers: [],
    warnings:
      untrackedDeletePaths.length > 0
        ? [
            {
              code: "untracked_delete",
              message: `其中 ${untrackedDeletePaths.length} 个未跟踪文件将从磁盘移除（执行前将备份至恢复快照）。`,
            },
          ]
        : [],
  };
}

export function applyShellDiscardConfirm(
  view: GitWorkbenchViewState,
  pending: PendingShellDiscard | null,
): GitWorkbenchViewState {
  const preview = view.discardPreview;
  if (!preview) return view;
  const nextChanges = pending
    ? applyOfflineDiscardChanges(view.workingChanges, pending)
    : applyOfflineDiscardChanges(view.workingChanges, {
        scope: "files",
        paths: [
          ...preview.trackedOverwritePaths,
          ...preview.untrackedDeletePaths,
        ],
      });
  const recoveryOid = "9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b";
  const targetPaths = [
    ...preview.trackedOverwritePaths,
    ...preview.untrackedDeletePaths,
  ];
  const recoveryCommand = `git restore --source=${recoveryOid} --worktree -- ${targetPaths.join(" ") || "."}`;
  const opId = `op-discard-${Date.now().toString(36)}`;
  const nowIso = new Date().toISOString();
  return {
    ...view,
    discardPreview: null,
    workingChanges: nextChanges,
    activeChangeId: nextChanges[0]?.id ?? null,
    operations: [
      {
        id: opId,
        repositoryId: preview.repositoryId,
        worktreeId: preview.worktreeId,
        kind: "discard",
        status: "succeeded",
        startedAt: nowIso,
        finishedAt: nowIso,
        summary: preview.summary,
        recoveryOid,
        recoveryCommand,
        fetchResults: [],
        preview: { worktreePath: preview.worktreePath },
        parents: [],
        reviewReasons: [],
      },
      ...(view.operations ?? []),
    ],
    statusBanner: `${preview.summary}（已生成安全恢复快照 ${recoveryOid.slice(0, 7)}）`,
  };
}
