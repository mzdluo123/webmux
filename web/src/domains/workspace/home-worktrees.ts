import { m } from "$lib/paraglide/messages.js";
import {
  HomeWorktreesSchema,
  type HomeWorktrees,
  type Repository,
  type SessionInfo,
  type Worktree,
} from "../../contracts/schemas";

/** Unreadable stored data degrades to "never curated" for every repository. */
export function parseHomeWorktrees(raw: string): HomeWorktrees {
  if (!raw) return {};
  try {
    const result = HomeWorktreesSchema.safeParse(JSON.parse(raw));
    return result.success ? result.data : {};
  } catch {
    return {};
  }
}

/** A repository the user never curated shows only its main worktree. */
function pinnedIds(home: HomeWorktrees, repo: Repository): ReadonlySet<string> {
  return new Set(
    home[repo.id] ??
      repo.worktrees.filter((tree) => tree.kind === "main").map((t) => t.id),
  );
}

export function worktreeTitle(tree: Worktree) {
  return (
    tree.branch ||
    (tree.detached
      ? `detached · ${tree.head?.slice(0, 12)}`
      : tree.kind === "bare"
        ? m.s8_bare_repo()
        : m.s8_no_commits_yet())
  );
}

export interface HomeLayout {
  /** All worktrees in stable order: main first, then by path. */
  trees: Worktree[];
  /** Pinned worktrees plus any worktree that still has sessions or archived records. */
  shown: { tree: Worktree; pinned: boolean }[];
  /** Every worktree not pinned, including ones shown only for their sessions. */
  unpinned: Worktree[];
}

/** The main worktree first, then the rest by path. */
function sortWorktrees(worktrees: readonly Worktree[]): Worktree[] {
  return [...worktrees].sort((a, b) =>
    a.kind === "main"
      ? -1
      : b.kind === "main"
        ? 1
        : a.path < b.path
          ? -1
          : a.path > b.path
            ? 1
            : 0,
  );
}

// Sessions are never hidden: a worktree with sessions or archived records
// stays on the home page whether or not it is pinned, so curation only
// removes idle clutter.
export function homeLayout(
  home: HomeWorktrees,
  repo: Repository,
  sessions: readonly SessionInfo[],
  archived: ReadonlyMap<string, readonly unknown[]> = new Map(),
): HomeLayout {
  const pinned = pinnedIds(pruneHomeWorktrees(home, [repo]), repo);
  const active = new Set([
    ...sessions
      .filter((s) => s.workspace?.repositoryId === repo.id)
      .map((s) => s.workspace?.worktreeId),
    ...archived.keys(),
  ]);
  const trees = sortWorktrees(repo.worktrees);
  return {
    trees,
    shown: trees
      .filter((tree) => pinned.has(tree.id) || active.has(tree.id))
      .map((tree) => ({ tree, pinned: pinned.has(tree.id) })),
    unpinned: trees.filter((tree) => !pinned.has(tree.id)),
  };
}

export function setPinned(
  home: HomeWorktrees,
  repo: Repository,
  ids: readonly string[],
  pinned: boolean,
): HomeWorktrees {
  const next = new Set(pinnedIds(home, repo));
  for (const id of ids) {
    if (pinned) next.add(id);
    else next.delete(id);
  }
  return { ...home, [repo.id]: [...next] };
}

/**
 * Drops entries of unregistered repositories and pins of worktrees that a
 * healthy snapshot no longer lists. Unhealthy snapshots keep their pins so a
 * transient Git failure cannot erase the user's choice. Returns `home` itself
 * when nothing changed.
 */
export function pruneHomeWorktrees(
  home: HomeWorktrees,
  repositories: readonly Repository[],
): HomeWorktrees {
  const next: HomeWorktrees = {};
  let changed = false;
  for (const [id, ids] of Object.entries(home)) {
    const repo = repositories.find((r) => r.id === id);
    if (!repo) {
      changed = true;
      continue;
    }
    if (repo.availability !== "available" || repo.error) {
      next[id] = ids;
      continue;
    }
    const kept = ids.filter((tree) =>
      repo.worktrees.some((t) => t.id === tree),
    );
    if (kept.length !== ids.length) changed = true;
    next[id] = kept;
  }
  return changed ? next : home;
}

const MAX_HOME_REPOSITORIES = 200;
const MAX_HOME_WORKTREE_IDS = 2000;
const MAX_WORKTREE_ID_LENGTH = 128;

function sameStringList(
  a: readonly string[] | undefined,
  b: readonly string[],
): boolean {
  return (
    a !== undefined &&
    a.length === b.length &&
    a.every((item, index) => item === b[index])
  );
}

function countPinnedIds(home: HomeWorktrees): number {
  let total = 0;
  for (const ids of Object.values(home)) total += ids.length;
  return total;
}

/**
 * Reconciles per-repository observed worktree IDs (`known`) against a settled,
 * error-free repository list and pins newly discovered external linked
 * worktrees onto `home`.
 *
 * - First observation of a repository records its current worktrees without
 *   pinning historical linked worktrees.
 * - Later observations pin newly added available linked worktrees unless
 *   already pinned; previously seen worktrees the user manually unpinned stay
 *   hidden.
 * - Removed worktrees leave `known`, so a later checkout at the same path is
 *   recognized as new again.
 */
export function reconcileDiscoveredWorktrees(input: {
  known: HomeWorktrees;
  home: HomeWorktrees;
  repositories: readonly Repository[];
  settled: boolean;
  error: string;
}): { known: HomeWorktrees; home: HomeWorktrees } {
  const { known, home, repositories, settled, error } = input;
  if (!settled || error) return { known, home };

  const nextKnown: HomeWorktrees = {};
  let knownChanged = false;
  let nextHome = home;

  for (const [id, ids] of Object.entries(known)) {
    if (!repositories.some((repo) => repo.id === id)) {
      knownChanged = true;
      continue;
    }
    nextKnown[id] = ids;
  }

  for (const repo of repositories) {
    if (repo.availability !== "available" || repo.error) {
      if (repo.id in known) nextKnown[repo.id] = known[repo.id]!;
      continue;
    }
    const currentIds = [
      ...new Set(
        repo.worktrees
          .map((tree) => tree.id)
          .filter((id) => id.length > 0 && id.length <= MAX_WORKTREE_ID_LENGTH)
          .slice(0, 1000),
      ),
    ];
    const previous = known[repo.id];
    if (!sameStringList(previous, currentIds)) {
      nextKnown[repo.id] = currentIds;
      knownChanged = true;
    } else {
      nextKnown[repo.id] = previous!;
    }
    if (previous === undefined || repo.id.length > MAX_WORKTREE_ID_LENGTH) {
      continue;
    }
    const seen = new Set(previous);
    const added = repo.worktrees
      .filter(
        (tree) =>
          tree.kind === "linked" &&
          tree.available &&
          !tree.prunable &&
          tree.id.length <= MAX_WORKTREE_ID_LENGTH &&
          !seen.has(tree.id),
      )
      .map((tree) => tree.id);
    if (!added.length) continue;

    const baseHome = pruneHomeWorktrees(nextHome, repositories);
    if (
      !(repo.id in baseHome) &&
      Object.keys(baseHome).length >= MAX_HOME_REPOSITORIES
    ) {
      continue;
    }
    const alreadyPinned = pinnedIds(baseHome, repo);
    const missing = added.filter((id) => !alreadyPinned.has(id));
    if (!missing.length) continue;

    const candidate = setPinned(baseHome, repo, missing, true);
    if (
      Object.keys(candidate).length <= MAX_HOME_REPOSITORIES &&
      countPinnedIds(candidate) <= MAX_HOME_WORKTREE_IDS
    ) {
      nextHome = candidate;
    }
  }

  return {
    known: knownChanged ? nextKnown : known,
    home: nextHome,
  };
}
