import * as m from "$lib/paraglide/messages.js";
import type {
  GitBranchItem,
  GitCommitOutcome,
  GitPrecedingCommit,
  GitPushPreviewModel,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import {
  type GitCommitDraftConflictState,
  joinCommitMessageDraft,
  mergeCommitMessageDrafts,
} from "./git-commit-draft-model";
import type { GitSharedBranchActionKind } from "./git-branches-model";
import {
  applyStageHunk,
  applyUnstageHunk,
  selectLogBranchFilter,
} from "./git-workbench-state";

export function applyShellStageGroup(
  changes: GitWorkbenchViewState["workingChanges"],
  op: "stage" | "unstage",
  group: "unstaged" | "staged" | "untracked",
): GitWorkbenchViewState["workingChanges"] {
  let next = changes;
  for (const it of changes.filter((c) => c.group === group)) {
    next =
      op === "stage"
        ? applyStageHunk(next, it.id)
        : applyUnstageHunk(next, it.id);
  }
  return next;
}

export function applyShellStartCompare(
  view: GitWorkbenchViewState,
  baseOid: string,
  targetOid: string,
): GitWorkbenchViewState {
  const d = view.commitDetails[targetOid];
  const files = d?.filesByParent[d.selectedParent ?? ""] ?? [];
  return {
    ...view,
    compareBaseMarkOid: null,
    selectedCommitFilePath: files[0]?.newPath ?? null,
    compareMode: {
      baseOid,
      targetOid,
      files,
      nextCursor: "",
      loading: false,
      loadingMore: false,
      error: "",
    },
  };
}

export function applyShellClearFilters(
  view: GitWorkbenchViewState,
): GitWorkbenchViewState {
  return {
    ...view,
    scope: "all",
    branchFilter: null,
    searchQuery: "",
    authorFilter: "",
    sinceFilter: "",
    untilFilter: "",
    pathFilter: "",
  };
}

export function applyShellDraftResolution(
  view: GitWorkbenchViewState,
  conflict: GitCommitDraftConflictState | null,
  strategy: "keep-local" | "adopt-remote" | "merge-both",
): {
  nextView: GitWorkbenchViewState;
  nextRevision: number;
} | null {
  if (!conflict) return null;
  if (strategy === "keep-local") {
    return {
      nextRevision: conflict.remoteDraft.revision + 1,
      nextView: {
        ...view,
        statusBanner: m.s7_shell_draft_kept_local(),
      },
    };
  }
  if (strategy === "adopt-remote") {
    return {
      nextRevision: conflict.remoteDraft.revision,
      nextView: {
        ...view,
        commitMessage: joinCommitMessageDraft(
          conflict.remoteDraft.subject,
          conflict.remoteDraft.body,
        ),
        statusBanner: m.s7_shell_draft_adopted_remote(),
      },
    };
  }
  const merged = mergeCommitMessageDrafts(
    { subject: conflict.localSubject, body: conflict.localBody },
    {
      subject: conflict.remoteDraft.subject,
      body: conflict.remoteDraft.body,
    },
  );
  return {
    nextRevision: conflict.remoteDraft.revision + 1,
    nextView: {
      ...view,
      commitMessage: joinCommitMessageDraft(merged.subject, merged.body),
      statusBanner: m.s7_shell_draft_merged(),
    },
  };
}

export function buildShellPushPreview(
  view: GitWorkbenchViewState,
  readonlyMode: boolean,
  defaultPushPreview: GitPushPreviewModel | null,
  branchOverride?: string,
  precedingCommit?: GitPrecedingCommit,
): GitPushPreviewModel {
  const sourceBranch =
    branchOverride ?? view.context.checkedOutBranch ?? "main";
  const baseModel: GitPushPreviewModel = defaultPushPreview ?? {
    repositoryName: view.context.repositoryName,
    sourceBranch,
    remote: "origin",
    targetBranch: sourceBranch,
    pushUrl: `https://example.invalid/${view.context.repositoryName}.git`,
    publish: !view.context.upstream,
    commits: Object.values(view.commitDetails),
    blockers: readonlyMode
      ? [
          {
            code: "no_worktree",
            message: m.s7_shell_push_blocker_readonly(),
          },
        ]
      : [],
    warnings: [],
  };
  return precedingCommit ? { ...baseModel, precedingCommit } : baseModel;
}

export function applyShellCommitConfirm(
  view: GitWorkbenchViewState,
  simulatedCommitOutcome: GitCommitOutcome | null,
): {
  nextView: GitWorkbenchViewState;
  followUpPush?: { branch: string; preceding: GitPrecedingCommit };
} | null {
  const preview = view.commitPreview;
  if (!preview) return null;
  const outcome: GitCommitOutcome = simulatedCommitOutcome ?? {
    status: "matched",
    commitOid: "f00dbabe1234567890abcdef1234567890abcdef",
    expectedTreeOid: preview.expectedTreeOid,
    actualTreeOid: preview.expectedTreeOid,
    summary: m.s7_shell_committed_from_index({
      count: String(preview.stagedFiles.length),
    }),
    reasons: [],
  };
  const remainingChanges =
    outcome.status === "matched"
      ? view.workingChanges.filter((c) => c.group !== "staged")
      : view.workingChanges;
  const shouldFollowUpPush =
    preview.followUpPush && outcome.status === "matched";
  const subject =
    view.commitMessage.trim().split(/\r?\n/, 1)[0] ||
    m.s7_shell_local_commit_subject();
  const commitOid =
    outcome.commitOid || "f00dbabe1234567890abcdef1234567890abcdef";

  return {
    nextView: {
      ...view,
      workingChanges: remainingChanges,
      commitMessage: outcome.status === "matched" ? "" : view.commitMessage,
      commitPreview: null,
      lastCommitOutcome: outcome,
      statusBanner: outcome.summary,
    },
    followUpPush: shouldFollowUpPush
      ? {
          branch: preview.branch,
          preceding: { commitOid, shortOid: commitOid.slice(0, 7), subject },
        }
      : undefined,
  };
}

export function applyShellBranchAction(options: {
  view: GitWorkbenchViewState;
  kind: GitSharedBranchActionKind;
  branch: GitBranchItem;
  onOpenWorktree?: (worktreeId: string, path: string) => void;
  onOpenPush: (branchName: string) => void;
}): GitWorkbenchViewState | null {
  const { view, kind, branch, onOpenWorktree, onOpenPush } = options;

  switch (kind) {
    case "filter-log":
      return { ...selectLogBranchFilter(view, branch.name), mode: "log" };
    case "clear-filter":
      return selectLogBranchFilter(view, null);
    case "toggle-favorite":
      return {
        ...view,
        branches: view.branches.map((b) =>
          b.name === branch.name ? { ...b, favorite: !b.favorite } : b,
        ),
      };
    case "compare": {
      const base =
        view.selectedCommitOid ??
        view.context.headOid ??
        view.commits[0]?.oid ??
        "";
      if (!base || base === branch.head) return view;
      const targetDetail = view.commitDetails[base];
      const pKey = targetDetail?.selectedParent ?? "";
      const files = targetDetail?.filesByParent[pKey] ?? [];
      return {
        ...view,
        mode: "log",
        compareBaseMarkOid: null,
        selectedCommitFilePath: files[0]?.newPath ?? null,
        compareMode: {
          baseOid: branch.head,
          targetOid: base,
          files,
          nextCursor: "",
          loading: false,
          loadingMore: false,
          error: "",
        },
      };
    }
    case "open-occupied-worktree":
      if (!branch.occupiedBy) return view;
      onOpenWorktree?.(
        branch.occupiedBy.worktreeId,
        branch.occupiedBy.worktreePath,
      );
      return {
        ...view,
        statusBanner: m.s7_shell_located_occupied_wt({
          path: branch.occupiedBy.worktreePath,
          worktreeId: branch.occupiedBy.worktreeId,
        }),
      };
    case "checkout":
      return {
        ...view,
        statusBanner: m.s7_shell_checkout_preview_opened({
          branch: branch.name,
        }),
      };
    case "create-branch-from":
      return {
        ...view,
        statusBanner: m.s7_shell_create_branch_from({ branch: branch.name }),
      };
    case "open-new-worktree":
      return {
        ...view,
        statusBanner: m.s7_shell_open_new_wt({ branch: branch.name }),
      };
    case "merge-into-current": {
      const src =
        branch.name === view.context.checkedOutBranch && branch.upstream
          ? branch.upstream
          : branch.name;
      const dst = view.context.checkedOutBranch ?? m.s7_current_branch();
      return {
        ...view,
        statusBanner: m.s7_shell_merge_preview_opened({ src, dst }),
      };
    }
    case "rebase-onto": {
      const src = view.context.checkedOutBranch ?? m.s7_current_branch();
      const dst =
        branch.name === view.context.checkedOutBranch && branch.upstream
          ? branch.upstream
          : branch.name;
      return {
        ...view,
        statusBanner: m.s7_shell_rebase_preview_opened({ src, dst }),
      };
    }
    case "fetch":
      return { ...view, statusBanner: m.s7_shell_fetch_triggered() };
    case "fast-forward":
      return {
        ...view,
        statusBanner: m.s7_shell_ff_preview_opened({ branch: branch.name }),
      };
    case "push":
      onOpenPush(branch.name);
      return null;
    case "rename-branch":
      return {
        ...view,
        statusBanner: m.s7_shell_rename_preview_opened({ branch: branch.name }),
      };
    case "set-upstream":
      return {
        ...view,
        statusBanner: m.s7_shell_upstream_opened({ branch: branch.name }),
      };
    case "create-tag":
      return {
        ...view,
        statusBanner: m.s7_shell_create_tag_opened({
          oid: branch.shortHead,
          branch: branch.name,
        }),
      };
    case "delete-tag":
      return {
        ...view,
        statusBanner: m.s7_shell_delete_tag_opened({ branch: branch.name }),
      };
    case "delete":
      return {
        ...view,
        statusBanner: m.s7_shell_delete_branch_opened({ branch: branch.name }),
      };
  }
}

export function applyShellSetTaskBaseline(
  view: GitWorkbenchViewState,
  baseOidOverride?: string,
): GitWorkbenchViewState {
  const unborn = !baseOidOverride && view.context.headState === "unborn";
  const baseOid = unborn ? "" : (baseOidOverride ?? view.context.headOid ?? "");
  const cleanAtStart = view.workingChanges.length === 0;
  const baseline = {
    baselineId: `bl-${view.context.worktreeId ?? "wt-main"}`,
    repositoryId: view.context.repositoryId,
    worktreeId: view.context.worktreeId ?? "wt-main",
    worktreePath: view.context.worktreePath ?? "/work/webmux",
    source: "manual_select" as const,
    unborn,
    ...(baseOid ? { baseOid } : {}),
    capturedAt: new Date().toISOString(),
    cleanAtStart,
    preExistingChanges: {
      staged: view.workingChanges.filter((c) => c.group === "staged").length,
      unstaged: view.workingChanges.filter((c) => c.group === "unstaged")
        .length,
      untracked: view.workingChanges.filter((c) => c.group === "untracked")
        .length,
      conflicted: view.workingChanges.filter((c) => c.group === "conflicted")
        .length,
    },
  };
  const label = unborn ? m.s7_unborn_baseline() : baseOid.slice(0, 7);
  return {
    ...view,
    taskBaseline: baseline,
    historyState:
      view.scope === "task" && view.historyState === "no_task_baseline"
        ? "ready"
        : view.historyState,
    taskReview: {
      status: "ready",
      reason: "",
      baseline,
      headOid: view.context.headOid ?? "",
      headUnborn: view.context.headState === "unborn",
      commitCount: view.taskReview?.commitCount ?? 0,
      uncommittedCount: view.workingChanges.length,
      files: view.taskReview?.files ?? [],
      selectedPath: view.taskReview?.selectedPath ?? null,
      searchQuery: view.taskReview?.searchQuery ?? "",
      nextCursor: "",
      loadingMore: false,
      mutating: false,
    },
    statusBanner: m.s7_task_baseline_set({ label }),
  };
}

export function applyShellToggleAmend(
  view: GitWorkbenchViewState,
  checked: boolean,
): GitWorkbenchViewState {
  if (!checked) {
    return {
      ...view,
      commitAmend: false,
      amendDraftConflict: null,
    };
  }
  const headOid = view.context.headOid ?? "";
  const headDetail = headOid ? view.commitDetails[headOid] : undefined;
  const headRow =
    view.commits.find((c) => c.oid === headOid) ?? view.commits[0];
  const headMsg = headDetail
    ? [headDetail.subject, headDetail.body].filter(Boolean).join("\n\n").trim()
    : (headRow?.subject ?? "").trim();
  const curDraft = view.commitMessage.trim();
  if (!curDraft && headMsg) {
    return {
      ...view,
      commitAmend: true,
      commitMessage: headMsg,
      amendDraftConflict: null,
    };
  }
  if (curDraft && headMsg && curDraft !== headMsg) {
    return {
      ...view,
      commitAmend: true,
      amendDraftConflict: { headMessage: headMsg, currentDraft: curDraft },
    };
  }
  return { ...view, commitAmend: true, amendDraftConflict: null };
}

export function applyShellResolveAmendDraft(
  view: GitWorkbenchViewState,
  choice: "replace" | "merge" | "keep",
): GitWorkbenchViewState {
  const cur = view.amendDraftConflict;
  if (!cur) return view;
  if (choice === "keep") {
    return { ...view, amendDraftConflict: null };
  }
  const commitMessage =
    choice === "replace"
      ? cur.headMessage
      : `${cur.currentDraft}\n\n${cur.headMessage}`;
  return { ...view, commitMessage, amendDraftConflict: null };
}

export function applyShellClearTaskBaseline(
  view: GitWorkbenchViewState,
): GitWorkbenchViewState {
  return {
    ...view,
    taskBaseline: null,
    historyState:
      view.scope === "task" ? "no_task_baseline" : view.historyState,
    taskReview: {
      status: "no_task_baseline",
      reason: "",
      baseline: null,
      headOid: view.context.headOid ?? "",
      headUnborn: view.context.headState === "unborn",
      commitCount: 0,
      uncommittedCount: view.workingChanges.length,
      files: [],
      selectedPath: null,
      searchQuery: "",
      nextCursor: "",
      loadingMore: false,
      mutating: false,
    },
    statusBanner: m.s7_task_baseline_cleared(),
  };
}
