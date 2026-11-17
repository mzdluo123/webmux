import { z } from "zod";
import { store, stored } from "../../platform/storage";

const FAVORITES_STORAGE_KEY = "gitWorkbenchFavorites";
const MAX_FAVORITE_BRANCHES = 64;
const MAX_FAVORITE_REPOS = 32;

const GitFavoritesMapSchema = z.record(
  z.string().min(1).max(128),
  z.array(z.string().min(1).max(256)).max(MAX_FAVORITE_BRANCHES),
);

function readFavoritesMap(): Record<string, string[]> {
  const raw = stored(FAVORITES_STORAGE_KEY, "");
  if (!raw) return {};
  try {
    return GitFavoritesMapSchema.safeParse(JSON.parse(raw)).data ?? {};
  } catch {
    return {};
  }
}

/**
 * Reads user-pinned favorite branch names for `repositoryId` (host-namespaced
 * via `platform/storage.ts`). When a repository has never been customized,
 * seeds `defaultBranch` (or `main`/`master` if present) in memory.
 */
export function readFavoriteBranches(
  repositoryId: string,
  defaultBranch?: string | null,
  availableLocalNames?: ReadonlySet<string>,
): string[] {
  if (!repositoryId) return [];
  const map = readFavoritesMap();
  const saved = map[repositoryId];
  if (saved !== undefined) {
    return [...saved];
  }
  const seeds: string[] = [];
  if (
    defaultBranch &&
    (!availableLocalNames || availableLocalNames.has(defaultBranch))
  ) {
    seeds.push(defaultBranch);
  } else if (availableLocalNames?.has("main")) {
    seeds.push("main");
  } else if (availableLocalNames?.has("master")) {
    seeds.push("master");
  }
  return seeds;
}

export function toggleFavoriteBranch(
  repositoryId: string,
  branchName: string,
  current: readonly string[],
): string[] {
  if (!repositoryId || !branchName) return [...current];
  const exists = current.includes(branchName);
  const next = exists
    ? current.filter((item) => item !== branchName)
    : [...current, branchName].slice(0, MAX_FAVORITE_BRANCHES);

  const map = readFavoritesMap();
  delete map[repositoryId];
  map[repositoryId] = next;
  const entries = Object.entries(map);
  const trimmed = Object.fromEntries(entries.slice(-MAX_FAVORITE_REPOS));
  store(FAVORITES_STORAGE_KEY, JSON.stringify(trimmed));
  return next;
}
