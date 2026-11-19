import {
  MAX_WORKTREE_PIN_REPOS,
  MAX_WORKTREE_PINS_PER_REPO,
  MAX_WORKTREE_PINS_TOTAL,
  type Repository,
  type Worktree,
  type WorktreePins,
} from "../../contracts/schemas";
import type { WorktreeMenuActionProps } from "./branch-actions";

function withRepoPins(
  pins: WorktreePins,
  repositoryId: string,
  ids: readonly string[],
): WorktreePins {
  const next: WorktreePins = { ...pins };
  if (ids.length > 0) next[repositoryId] = [...ids];
  else delete next[repositoryId];
  return next;
}

/**
 * Prunes stale worktree IDs only within `repo.id` on a healthy snapshot while
 * keeping sibling repositories' pins untouched.
 */
function pruneRepoWorktreePins(
  pins: WorktreePins,
  repo: Repository,
): WorktreePins {
  const saved = pins[repo.id];
  if (!saved) return pins;
  if (repo.availability !== "available" || repo.error) {
    return saved.length > 0 ? pins : withRepoPins(pins, repo.id, []);
  }
  const kept = saved.filter((treeId) =>
    repo.worktrees.some((t) => t.id === treeId),
  );
  if (kept.length === saved.length && kept.length > 0) return pins;
  return withRepoPins(pins, repo.id, kept);
}

/**
 * Ordered pinned worktree IDs for `repositoryId`, narrowed to `worktrees` when
 * a snapshot of worktrees is supplied.
 */
export function pinnedWorktreeIds(
  pins: WorktreePins,
  repositoryId: string,
  worktrees?: readonly Worktree[],
): string[] {
  const saved = pins[repositoryId] ?? [];
  if (!worktrees) return [...saved];
  const known = new Set(worktrees.map((tree) => tree.id));
  return saved.filter((id) => known.has(id));
}

export function isWorktreePinned(
  pins: WorktreePins,
  repositoryId: string,
  worktreeId: string,
): boolean {
  return Boolean(pins[repositoryId]?.includes(worktreeId));
}

export function isRepoWorktreePinsFull(
  pins: WorktreePins,
  repo: Repository,
): boolean {
  const pruned = pruneRepoWorktreePins(pins, repo);
  const count = pruned[repo.id]?.length ?? 0;
  if (count >= MAX_WORKTREE_PINS_PER_REPO) return true;
  const entries = Object.entries(pruned).filter(([, ids]) => ids.length > 0);
  if (count === 0 && entries.length >= MAX_WORKTREE_PIN_REPOS) return true;
  const total = entries.reduce((sum, [, ids]) => sum + ids.length, 0);
  return total >= MAX_WORKTREE_PINS_TOTAL;
}

/**
 * Sorts a repository's worktrees: pinned worktrees first in their saved order
 * (ahead of unpinned main and linked worktrees), then unpinned main, then the
 * rest by path.
 */
export function sortRepositoryWorktrees(
  worktrees: readonly Worktree[],
  pins: WorktreePins = {},
  repositoryId?: string,
): Worktree[] {
  const repoId = repositoryId ?? worktrees[0]?.repositoryId ?? "";
  const saved = pins[repoId] ?? [];
  const pinRank = new Map(saved.map((id, index) => [id, index]));
  return [...worktrees].sort((a, b) => {
    const rankA = pinRank.get(a.id);
    const rankB = pinRank.get(b.id);
    if (rankA !== undefined && rankB !== undefined) return rankA - rankB;
    if (rankA !== undefined) return -1;
    if (rankB !== undefined) return 1;
    if (a.kind === "main" && b.kind !== "main") return -1;
    if (b.kind === "main" && a.kind !== "main") return 1;
    return a.path < b.path ? -1 : a.path > b.path ? 1 : 0;
  });
}

export function toggleWorktreePin(
  pins: WorktreePins,
  repo: Repository,
  worktreeId: string,
): { pins: WorktreePins; pinned: boolean } {
  const pruned = pruneRepoWorktreePins(pins, repo);
  const current = pruned[repo.id] ?? [];
  if (current.includes(worktreeId)) {
    return {
      pins: withRepoPins(
        pruned,
        repo.id,
        current.filter((id) => id !== worktreeId),
      ),
      pinned: false,
    };
  }
  if (isRepoWorktreePinsFull(pruned, repo)) {
    return { pins: pruned, pinned: false };
  }
  return {
    pins: withRepoPins(pruned, repo.id, [worktreeId, ...current]),
    pinned: true,
  };
}

export function canMoveWorktreePin(
  pins: WorktreePins,
  repo: Repository,
  worktreeId: string,
  offset: -1 | 1,
  candidates?: readonly Worktree[],
): boolean {
  const ordered = pinnedWorktreeIds(
    pins,
    repo.id,
    candidates ?? repo.worktrees,
  );
  const index = ordered.indexOf(worktreeId);
  return index >= 0 && ordered[index + offset] !== undefined;
}

export function moveWorktreePin(
  pins: WorktreePins,
  repo: Repository,
  worktreeId: string,
  offset: -1 | 1,
  candidates?: readonly Worktree[],
): WorktreePins {
  const pruned = pruneRepoWorktreePins(pins, repo);
  const ordered = pinnedWorktreeIds(
    pruned,
    repo.id,
    candidates ?? repo.worktrees,
  );
  const index = ordered.indexOf(worktreeId);
  const other = ordered[index + offset];
  if (index < 0 || !other) return pruned;
  const current = pruned[repo.id] ?? [];
  return withRepoPins(
    pruned,
    repo.id,
    current.map((id) =>
      id === worktreeId ? other : id === other ? worktreeId : id,
    ),
  );
}

export function removeWorktreePins(
  pins: WorktreePins,
  repositoryId: string,
  removedIds: readonly string[],
): WorktreePins {
  if (!removedIds.length || !pins[repositoryId]?.length) return pins;
  const removed = new Set(removedIds);
  const kept = (pins[repositoryId] ?? []).filter((id) => !removed.has(id));
  if (kept.length === (pins[repositoryId]?.length ?? 0)) return pins;
  return withRepoPins(pins, repositoryId, kept);
}

export function removeRepositoryWorktreePins(
  pins: WorktreePins,
  repositoryId: string,
): WorktreePins {
  if (!(repositoryId in pins)) return pins;
  return withRepoPins(pins, repositoryId, []);
}

/**
 * Drops entries of unregistered repositories and pins of worktrees missing
 * from a healthy snapshot; unhealthy snapshots retain their pins.
 */
export function pruneWorktreePins(
  pins: WorktreePins,
  repositories: readonly Repository[],
): WorktreePins {
  const next: WorktreePins = {};
  let changed = false;
  for (const [id, ids] of Object.entries(pins)) {
    const repo = repositories.find((r) => r.id === id);
    if (!repo) {
      changed = true;
      continue;
    }
    if (repo.availability !== "available" || repo.error) {
      if (ids.length > 0) next[id] = ids;
      else changed = true;
      continue;
    }
    const kept = ids.filter((treeId) =>
      repo.worktrees.some((t) => t.id === treeId),
    );
    if (kept.length !== ids.length) changed = true;
    if (kept.length > 0) next[id] = kept;
    else if (ids.length > 0) changed = true;
  }
  return changed ? next : pins;
}

export function buildWorktreePinMenuProps(input: {
  pins: WorktreePins;
  repo: Repository | undefined;
  worktreeId: string;
  candidates?: readonly Worktree[];
  ready?: boolean;
  ontoggle?: (repositoryId: string, worktreeId: string) => void;
  onmove?: (repositoryId: string, worktreeId: string, offset: -1 | 1) => void;
}): Pick<
  WorktreeMenuActionProps,
  | "topPinned"
  | "topPinDisabled"
  | "ontoggletoppin"
  | "canMoveTopPinUp"
  | "canMoveTopPinDown"
  | "onmovetoppinup"
  | "onmovetoppindown"
> {
  const {
    pins,
    repo,
    worktreeId,
    candidates,
    ready = true,
    ontoggle,
    onmove,
  } = input;
  if (!repo || !ontoggle) return {};
  const topPinned = isWorktreePinned(pins, repo.id, worktreeId);
  const target = repo.worktrees.find((item) => item.id === worktreeId);
  const pinnable =
    !target || (target.available && !target.prunable && target.kind !== "bare");
  if (!topPinned && !pinnable) return {};
  const full = isRepoWorktreePinsFull(pins, repo);
  const canUp =
    topPinned && canMoveWorktreePin(pins, repo, worktreeId, -1, candidates);
  const canDown =
    topPinned && canMoveWorktreePin(pins, repo, worktreeId, 1, candidates);
  return {
    topPinned,
    topPinDisabled: !ready || (!topPinned && full),
    ontoggletoppin: () => ontoggle(repo.id, worktreeId),
    canMoveTopPinUp: ready && canUp,
    canMoveTopPinDown: ready && canDown,
    onmovetoppinup:
      topPinned && onmove ? () => onmove(repo.id, worktreeId, -1) : undefined,
    onmovetoppindown:
      topPinned && onmove ? () => onmove(repo.id, worktreeId, 1) : undefined,
  };
}
