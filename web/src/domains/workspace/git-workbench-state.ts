import { m } from "$lib/paraglide/messages.js";
import type {
  GitCommitDetail,
  GitCommitFile,
  GitCommitPreviewModel,
  GitHistoryRow,
  GitIssue,
  GitWorkbenchViewState,
  GitWorkingChange,
  GroupedWorkingChanges,
} from "../../contracts/git-workbench-schemas";

export function groupWorkingChanges(
  changes: readonly GitWorkingChange[],
): GroupedWorkingChanges {
  const out: GroupedWorkingChanges = {
    conflicted: [],
    unstaged: [],
    staged: [],
    untracked: [],
  };
  for (const item of changes) {
    out[item.group].push(item);
  }
  return out;
}

/**
 * Clicking a branch in the Log pane ONLY updates the history filter and NEVER
 * mutates `context.checkedOutBranch`.
 */
export interface LogClientFilterExtra {
  authorFilter?: string;
  sinceFilter?: string;
  untilFilter?: string;
}

export function selectLogBranchFilter(
  state: GitWorkbenchViewState,
  branchName: string | null,
): GitWorkbenchViewState {
  const filtered = filterLogCommits(
    state.commits,
    branchName,
    state.searchQuery,
    state.pathFilter,
    {
      authorFilter: state.authorFilter,
      sinceFilter: state.sinceFilter,
      untilFilter: state.untilFilter,
    },
  );
  const nextSelected =
    filtered.find((c) => c.oid === state.selectedCommitOid)?.oid ??
    filtered[0]?.oid ??
    null;
  return {
    ...state,
    branchFilter: branchName,
    selectedCommitOid: nextSelected,
    statusBanner: branchName
      ? m.s7_state_log_filtered_by_ref({
          ref: branchName,
          checkedOut: state.context.checkedOutBranch ?? m.s7_readonly_context(),
        })
      : m.s7_state_log_all_reachable(),
  };
}

export function filterLogCommits(
  commits: readonly GitHistoryRow[],
  branchFilter: string | null,
  searchQuery: string,
  pathFilter: string,
  extra?: LogClientFilterExtra,
): GitHistoryRow[] {
  const q = searchQuery.trim().toLowerCase();
  const p = pathFilter.trim().toLowerCase();
  const author = extra?.authorFilter?.trim().toLowerCase() ?? "";
  const sinceMs = extra?.sinceFilter
    ? Date.parse(extra.sinceFilter)
    : Number.NaN;
  const untilMs = extra?.untilFilter
    ? Date.parse(
        extra.untilFilter.includes("T")
          ? extra.untilFilter
          : `${extra.untilFilter}T23:59:59.999Z`,
      )
    : Number.NaN;
  return commits.filter((row) => {
    if (
      branchFilter &&
      row.branches.length > 0 &&
      !row.branches.includes(branchFilter) &&
      !row.refs.some((ref) => ref.name === branchFilter)
    ) {
      return false;
    }
    if (
      q &&
      !row.subject.toLowerCase().includes(q) &&
      !row.authorName.toLowerCase().includes(q) &&
      !row.shortOid.toLowerCase().includes(q) &&
      !row.oid.toLowerCase().startsWith(q)
    ) {
      return false;
    }
    if (
      author &&
      !row.authorName.toLowerCase().includes(author) &&
      !row.authorEmail.toLowerCase().includes(author)
    ) {
      return false;
    }
    if (!Number.isNaN(sinceMs) || !Number.isNaN(untilMs)) {
      const rowMs = Date.parse(row.committerAt || row.authorAt);
      if (!Number.isNaN(rowMs)) {
        if (!Number.isNaN(sinceMs) && rowMs < sinceMs) return false;
        if (!Number.isNaN(untilMs) && rowMs > untilMs) return false;
      }
    }
    if (p) {
      const hitPath = row.changedPaths.some((filePath) =>
        filePath.toLowerCase().includes(p),
      );
      if (!hitPath && !row.subject.toLowerCase().includes(p)) {
        return false;
      }
    }
    return true;
  });
}

export function resolveCommitParentFiles(
  detail: GitCommitDetail,
  parentOverride?: string,
): { selectedParent: string; files: GitCommitFile[] } {
  const firstParent = detail.parents[0];
  if (!firstParent) {
    return { selectedParent: "", files: detail.filesByParent[""] ?? [] };
  }
  const valid =
    parentOverride && detail.parents.some((p) => p.oid === parentOverride)
      ? parentOverride
      : detail.selectedParent || firstParent.oid;
  return {
    selectedParent: valid,
    files: detail.filesByParent[valid] ?? [],
  };
}

const REVIEW_VERSION_SEP = "::v=";

function buildChangeReviewToken(change: {
  id?: string;
  path: string;
  identityKey?: string;
}): string {
  const key = change.id || change.path;
  const ver = change.identityKey?.trim();
  return ver ? `${key}${REVIEW_VERSION_SEP}${ver}` : key;
}

export function isChangeReviewed(
  reviewedPaths: readonly string[],
  change: { id?: string; path: string; identityKey?: string },
): boolean {
  const token = buildChangeReviewToken(change);
  if (reviewedPaths.includes(token)) return true;
  if (!change.identityKey?.trim()) {
    return (
      reviewedPaths.includes(change.path) ||
      Boolean(change.id && reviewedPaths.includes(change.id))
    );
  }
  return false;
}

export function toggleReviewedChange(
  reviewedPaths: readonly string[],
  change: { id?: string; path: string; identityKey?: string },
): string[] {
  const key = change.id || change.path;
  const token = buildChangeReviewToken(change);
  const wasReviewed = isChangeReviewed(reviewedPaths, change);
  const cleaned = reviewedPaths.filter(
    (entry) =>
      entry !== token &&
      entry !== key &&
      entry !== change.path &&
      !entry.startsWith(`${key}${REVIEW_VERSION_SEP}`) &&
      !entry.startsWith(`${change.path}${REVIEW_VERSION_SEP}`),
  );
  return wasReviewed ? cleaned : [...cleaned, token];
}

export function toggleReviewedPath(
  reviewedPaths: readonly string[],
  path: string,
): string[] {
  return reviewedPaths.includes(path)
    ? reviewedPaths.filter((item) => item !== path)
    : [...reviewedPaths, path];
}

export function applyStageHunk(
  changes: readonly GitWorkingChange[],
  changeId: string,
  hunkId?: string,
): GitWorkingChange[] {
  const target = changes.find((c) => c.id === changeId);
  if (
    !target ||
    (target.group !== "unstaged" && target.group !== "untracked")
  ) {
    return [...changes];
  }
  const chosenHunks = hunkId
    ? target.hunks.filter((h) => h.id === hunkId)
    : target.hunks;
  const remainingHunks = hunkId
    ? target.hunks.filter((h) => h.id !== hunkId)
    : [];

  const next: GitWorkingChange[] = [];
  let mergedIntoExistingStaged = false;

  for (const item of changes) {
    if (item.id === changeId) {
      if (hunkId && remainingHunks.length > 0) {
        next.push({
          ...item,
          group: "unstaged",
          status: item.status === "?" ? "M" : item.status,
          hunks: remainingHunks,
        });
      }
      continue;
    }
    if (item.group === "staged" && item.path === target.path) {
      mergedIntoExistingStaged = true;
      next.push({
        ...item,
        hunks: [...item.hunks, ...chosenHunks],
      });
      continue;
    }
    next.push(item);
  }

  if (!mergedIntoExistingStaged) {
    next.push({
      ...target,
      id: `staged:${target.path}`,
      group: "staged",
      status: target.status === "?" ? "A" : target.status,
      preStagedFromCli: false,
      hunks: chosenHunks,
    });
  }
  return next;
}

export function applyUnstageHunk(
  changes: readonly GitWorkingChange[],
  changeId: string,
  hunkId?: string,
): GitWorkingChange[] {
  const target = changes.find((c) => c.id === changeId);
  if (!target || target.group !== "staged") {
    return [...changes];
  }
  const movedHunks = hunkId
    ? target.hunks.filter((h) => h.id === hunkId)
    : target.hunks;
  const remainingStaged = hunkId
    ? target.hunks.filter((h) => h.id !== hunkId)
    : [];

  const next: GitWorkingChange[] = [];
  let mergedIntoExistingUnstaged = false;

  for (const item of changes) {
    if (item.id === changeId) {
      if (hunkId && remainingStaged.length > 0) {
        next.push({ ...item, hunks: remainingStaged });
      }
      continue;
    }
    if (item.group === "unstaged" && item.path === target.path) {
      mergedIntoExistingUnstaged = true;
      next.push({
        ...item,
        hunks: [...movedHunks, ...item.hunks],
      });
      continue;
    }
    next.push(item);
  }

  if (!mergedIntoExistingUnstaged) {
    next.push({
      ...target,
      id: `unstaged:${target.path}`,
      group: target.status === "A" && !hunkId ? "untracked" : "unstaged",
      status: target.status === "A" && !hunkId ? "?" : "M",
      preStagedFromCli: false,
      hunks: movedHunks,
    });
  }
  return next;
}

export function buildCommitPreviewModel(
  state: GitWorkbenchViewState,
  followUpPush: boolean,
): GitCommitPreviewModel {
  const grouped = groupWorkingChanges(state.workingChanges);
  const blockers: GitIssue[] = [];
  const warnings: GitIssue[] = [];

  if (grouped.conflicted.length > 0) {
    blockers.push({
      code: "conflicted",
      message: m.s7_preview_blocker_conflicted(),
    });
  }
  if (state.context.operationInProgress) {
    blockers.push({
      code: "operation_in_progress",
      message: m.s7_preview_blocker_op_in_progress({
        op: state.context.operationInProgress,
      }),
    });
  }
  if (state.context.headState === "detached") {
    blockers.push({
      code: "detached_head",
      message: m.s7_preview_blocker_detached_head(),
    });
  }
  if (state.context.headState === "readonly" || !state.context.worktreePath) {
    blockers.push({
      code: "no_worktree",
      message: m.s7_preview_blocker_no_worktree(),
    });
  }
  if (
    grouped.staged.length === 0 &&
    grouped.conflicted.length === 0 &&
    !state.commitAmend
  ) {
    blockers.push({
      code: "empty_index",
      message: m.s7_preview_blocker_empty_index(),
    });
  }
  if (state.commitAmend && state.context.headState === "unborn") {
    blockers.push({
      code: "unborn_head",
      message: m.s7_preview_blocker_unborn_amend(),
    });
  }
  if (
    !state.commitAuthorOverride?.trim() &&
    (!state.context.authorName.trim() || !state.context.authorEmail.trim())
  ) {
    blockers.push({
      code: "missing_identity",
      message: m.s7_preview_blocker_missing_identity(),
    });
  }
  if (state.commitAmend && state.commitAmendPushed) {
    warnings.push({
      code: "amend_pushed",
      message: m.s7_preview_warn_amend_pushed(),
    });
  }

  const cliPreStagedCount = grouped.staged.filter(
    (s) => s.preStagedFromCli,
  ).length;
  if (cliPreStagedCount > 0) {
    warnings.push({
      code: "cli_prestaged",
      message: m.s7_preview_warn_cli_prestaged({
        count: String(cliPreStagedCount),
      }),
    });
  }
  if (grouped.unstaged.length > 0 || grouped.untracked.length > 0) {
    warnings.push({
      code: "remaining_unstaged",
      message: m.s7_preview_warn_remaining_unstaged({
        unstaged: String(grouped.unstaged.length),
        untracked: String(grouped.untracked.length),
      }),
    });
  }
  if (state.context.executableHooks.length > 0) {
    warnings.push({
      code: "hooks_active",
      message: m.s7_preview_warn_hooks_active({
        hooks: state.context.executableHooks.join(", "),
      }),
    });
  }

  const headState =
    state.context.headState === "readonly" ? "branch" : state.context.headState;
  const rawOverride = state.commitAuthorOverride?.trim() || "";
  const overrideMatch = /^([^<>]+?)\s*<([^<>\s]+@[^<>\s]+)>$/.exec(rawOverride);
  const effectiveAuthorName = overrideMatch
    ? (overrideMatch[1] ?? "").trim()
    : state.context.authorName;
  const effectiveAuthorEmail = overrideMatch
    ? (overrideMatch[2] ?? "").trim()
    : state.context.authorEmail;

  return {
    repositoryName: state.context.repositoryName,
    worktreePath: state.context.worktreePath ?? m.s7_no_worktree_paren(),
    headState,
    branch: state.context.checkedOutBranch ?? "(detached)",
    headOid: state.context.headOid ?? "",
    headMessage: "",
    headParents: [],
    expectedTreeOid: `tree-${grouped.staged.map((f) => f.path).join("+") || "empty"}`,
    authorName: effectiveAuthorName,
    authorEmail: effectiveAuthorEmail,
    committerName: state.context.committerName,
    committerEmail: state.context.committerEmail,
    signEnabled: state.context.signEnabled,
    signFormat: state.context.signFormat || undefined,
    operation: state.context.operationInProgress ?? undefined,
    executableHooks: state.context.executableHooks,
    stagedFiles: grouped.staged,
    unstagedCount: grouped.unstaged.length,
    untrackedCount: grouped.untracked.length,
    conflictedCount: grouped.conflicted.length,
    followUpPush:
      state.commitAmend && state.commitAmendPushed ? false : followUpPush,
    amend: Boolean(state.commitAmend),
    amendPushed: Boolean(state.commitAmend && state.commitAmendPushed),
    authorOverride: rawOverride,
    authorOverridden: Boolean(rawOverride),
    envAuthorName: overrideMatch ? state.context.authorName : "",
    envAuthorEmail: overrideMatch ? state.context.authorEmail : "",
    signoff: Boolean(state.commitSignoff),
    signoffTrailer: state.commitSignoff
      ? `Signed-off-by: ${state.context.committerName} <${state.context.committerEmail}>`
      : "",
    blockers,
    warnings,
    stale: false,
    dialogError: "",
  };
}
