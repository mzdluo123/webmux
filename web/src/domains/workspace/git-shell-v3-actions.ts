import * as m from "$lib/paraglide/messages.js";
import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
import type {
  GitDiscardPreview,
  GitLineSelection,
  SessionInfo,
} from "../../contracts/schemas";
import {
  applyOfflineDiscardChanges,
  applyOfflineLineMutation,
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
      ? m.s7_discard_summary_lines({
          path: trackedOverwritePaths[0] ?? "",
          count: String(lineCount),
        })
      : pending.scope === "hunks"
        ? m.s7_discard_summary_hunk({ path: trackedOverwritePaths[0] ?? "" })
        : m.s7_discard_summary_files({ count: String(items.length) });

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
              message: m.s7_discard_warn_untracked({
                count: String(untrackedDeletePaths.length),
              }),
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
    statusBanner: m.s7_discard_confirmed_banner({
      summary: preview.summary,
      oid: recoveryOid.slice(0, 7),
    }),
  };
}

export function applyShellStageLines(
  view: GitWorkbenchViewState,
  changeId: string,
  sel: Parameters<typeof applyOfflineLineMutation>[2],
  op: "stage" | "unstage",
): GitWorkbenchViewState {
  return {
    ...view,
    workingChanges: applyOfflineLineMutation(
      view.workingChanges,
      changeId,
      sel,
      op,
    ),
  };
}

export function applyShellCheckoutBanner(
  view: GitWorkbenchViewState,
  branch: string,
): GitWorkbenchViewState {
  return {
    ...view,
    statusBanner: m.s7_shell_checkout_preview_opened({ branch }),
  };
}

export function applyShellOccupiedWorktreeBanner(
  view: GitWorkbenchViewState,
  worktreeId: string,
  path: string,
  onOpenWorktree?: (worktreeId: string, path: string) => void,
): GitWorkbenchViewState {
  onOpenWorktree?.(worktreeId, path);
  return {
    ...view,
    statusBanner: m.s7_shell_located_occupied_wt({ path, worktreeId }),
  };
}

export function applyShellReloadVerify(
  view: GitWorkbenchViewState,
): GitWorkbenchViewState {
  return {
    ...view,
    lastCommitOutcome: null,
    statusBanner: m.s7_shell_reloaded_verify(),
  };
}

export function applyShellContinueInProgress(
  view: GitWorkbenchViewState,
): GitWorkbenchViewState {
  return {
    ...view,
    inProgress: null,
    conflictOpen: false,
    statusBanner: m.s7_shell_continued_operation(),
  };
}

export function applyShellAbortPreviewBanner(
  view: GitWorkbenchViewState,
): GitWorkbenchViewState {
  return {
    ...view,
    statusBanner: m.s7_shell_abort_preview_opened(),
  };
}

export function applyShellStashBanner(
  view: GitWorkbenchViewState,
  action: "create" | "pop" | "apply" | "drop",
  ref = "",
): GitWorkbenchViewState {
  const statusBanner =
    action === "create"
      ? m.s7_shell_stash_create_opened()
      : action === "pop"
        ? m.s7_shell_stash_pop_opened({ ref })
        : action === "apply"
          ? m.s7_shell_stash_apply_opened({ ref })
          : m.s7_shell_stash_drop_opened({ ref });
  return { ...view, statusBanner };
}
