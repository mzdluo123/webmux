import * as m from "$lib/paraglide/messages.js";
import { placeArchived } from "../sessions/archived";
import { homeLayout, worktreeTitle } from "./home-worktrees";
import type {
  ArchivedSession,
  HomeWorktrees,
  RemovalPreviewItem,
  Repository,
  SessionInfo,
  Worktree,
} from "../../contracts/schemas";

type WorktreeBatchCategory = "safe" | "blocked" | "protected";
export type WorktreeBatchFilter = "all" | WorktreeBatchCategory;

/** Why a locked registration whose directory is gone is left alone. */
export const lockedStaleReason = () => m.s8_batch_locked_stale_reason();

export interface WorktreeBatchEntry {
  tree: Worktree;
  title: string;
  category: WorktreeBatchCategory;
  pinned: boolean;
  activeSessionCount: number;
  exitedSessionCount: number;
  archivedCount: number;
  reason: string;
}

export interface WorktreeBatchCounts {
  all: number;
  safe: number;
  blocked: number;
  protected: number;
}

export interface BatchPreviewGroups {
  safe: RemovalPreviewItem[];
  blocked: RemovalPreviewItem[];
  unsupported: RemovalPreviewItem[];
}

function insideWorktree(cwd: string, worktreePath: string): boolean {
  if (!cwd || !worktreePath) return false;
  if (cwd === worktreePath) return true;
  const prefix = worktreePath.endsWith("/") ? worktreePath : `${worktreePath}/`;
  return cwd.startsWith(prefix);
}

function classifyWorktree(
  tree: Worktree,
  activeSessionCount: number,
): { category: WorktreeBatchCategory; reason: string } {
  if (tree.kind === "main") {
    return { category: "protected", reason: m.s8_batch_main_protected() };
  }
  if (tree.kind !== "linked") {
    return { category: "protected", reason: m.s8_batch_bare_protected() };
  }
  if (tree.locked) {
    return {
      category: "protected",
      reason:
        tree.prunable && !tree.available
          ? lockedStaleReason()
          : tree.lockReason
            ? m.s8_batch_locked_with_reason({ reason: tree.lockReason })
            : m.s8_batch_locked_unlock_first(),
    };
  }
  const missingPrunable = tree.prunable && !tree.available;
  if (!missingPrunable) {
    if (!tree.available || tree.prunable) {
      return {
        category: "protected",
        reason: m.s8_batch_dir_missing_unverifiable(),
      };
    }
    if (tree.dirty === null || tree.error !== "") {
      return {
        category: "protected",
        reason: tree.error
          ? m.s8_batch_status_unknown_with_err({ error: tree.error })
          : m.s8_batch_status_unknown(),
      };
    }
  }
  const blockers: string[] = [];
  if (tree.dirty === true) blockers.push(m.s8_batch_uncommitted_changes());
  if (tree.hasUntracked) blockers.push(m.s8_batch_untracked_files());
  if (activeSessionCount > 0) {
    blockers.push(
      m.s8_batch_running_sessions({ count: String(activeSessionCount) }),
    );
  }
  if (blockers.length > 0) {
    return { category: "blocked", reason: blockers.join(m.s8_list_sep()) };
  }
  return {
    category: "safe",
    reason: missingPrunable
      ? m.s8_batch_stale_reg_cleanable()
      : m.s8_batch_clean_idle(),
  };
}

/**
 * Builds all worktrees of `repo` in stable order (main first, then by path),
 * regardless of whether they are pinned to the home page.
 */
export function buildWorktreeBatchEntries(
  repo: Repository,
  home: HomeWorktrees,
  sessions: readonly SessionInfo[],
  archived: readonly ArchivedSession[],
): WorktreeBatchEntry[] {
  const repoSessions = sessions.filter((s) => s.state === "active");
  const placement = placeArchived([...archived], [repo]).repositories.get(
    repo.id,
  );
  const layout = homeLayout(home, repo, repoSessions, placement?.worktrees);
  const pinnedIds = new Set(
    layout.shown.filter((item) => item.pinned).map((item) => item.tree.id),
  );
  return layout.trees.map((tree) => {
    const matchedSessions = repoSessions.filter(
      (s) =>
        s.workspace?.worktreeId === tree.id || insideWorktree(s.cwd, tree.path),
    );
    const activeSessionCount = matchedSessions.filter(
      (s) => s.status !== "exited",
    ).length;
    const exitedSessionCount = matchedSessions.length - activeSessionCount;
    const archivedCount = archived.filter(
      (rec) =>
        !rec.released &&
        (rec.workspace?.worktreeId === tree.id ||
          insideWorktree(rec.cwd, tree.path)),
    ).length;
    const { category, reason } = classifyWorktree(tree, activeSessionCount);
    return {
      tree,
      title: worktreeTitle(tree),
      category,
      pinned: pinnedIds.has(tree.id),
      activeSessionCount,
      exitedSessionCount,
      archivedCount,
      reason,
    };
  });
}

export function countWorktreeBatchCategories(
  entries: readonly WorktreeBatchEntry[],
): WorktreeBatchCounts {
  const counts: WorktreeBatchCounts = {
    all: entries.length,
    safe: 0,
    blocked: 0,
    protected: 0,
  };
  for (const entry of entries) {
    counts[entry.category]++;
  }
  return counts;
}

/**
 * The stale registrations "清理失效登记" preselects: unlocked linked
 * worktrees Git marks prunable whose directory is gone, and that are safe
 * to clean (no running session). The batch preview checks the directory
 * again on the server; this list only drives the shortcut and its count.
 */
export function staleRegistrations(
  entries: readonly WorktreeBatchEntry[],
): WorktreeBatchEntry[] {
  return entries.filter(
    ({ tree, category }) =>
      category === "safe" &&
      tree.kind === "linked" &&
      tree.prunable &&
      !tree.available &&
      !tree.locked,
  );
}

export function filterWorktreeBatchEntries(
  entries: readonly WorktreeBatchEntry[],
  filter: WorktreeBatchFilter,
  query: string,
): WorktreeBatchEntry[] {
  const term = query.trim().toLowerCase();
  return entries.filter((entry) => {
    if (filter !== "all" && entry.category !== filter) return false;
    if (!term) return true;
    const head = entry.tree.head ?? "";
    const branch = entry.tree.branch ?? "";
    return (
      entry.title.toLowerCase().includes(term) ||
      branch.toLowerCase().includes(term) ||
      head.toLowerCase().includes(term) ||
      entry.tree.path.toLowerCase().includes(term)
    );
  });
}

export function groupBatchPreviewItems(
  items: readonly RemovalPreviewItem[],
): BatchPreviewGroups {
  const groups: BatchPreviewGroups = {
    safe: [],
    blocked: [],
    unsupported: [],
  };
  for (const item of items) {
    if (!item.forceAllowed) groups.unsupported.push(item);
    else if (item.blockers.length > 0) groups.blocked.push(item);
    else groups.safe.push(item);
  }
  return groups;
}
