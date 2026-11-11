import { z } from "zod";
import {
  GitHistoryScopeSchema,
  GitWorkbenchModeSchema,
  type GitHistoryScope,
  type GitWorkbenchMode,
} from "../../contracts/git-workbench-schemas";
import { activeHostId, store, stored } from "../../platform/storage";

const GitDeviceClassSchema = z.enum(["desktop", "compact", "tablet", "phone"]);
export type GitDeviceClass = z.infer<typeof GitDeviceClassSchema>;

const GitDeviceLayoutSchema = z.object({
  showBranchesPanel: z.boolean(),
  showAuthorColumn: z.boolean(),
  logThreePaneSizes: z.tuple([z.number(), z.number(), z.number()]),
  logTwoPaneSizes: z.tuple([z.number(), z.number()]),
  commitPaneSizes: z.tuple([z.number(), z.number()]),
  diffMode: z.enum(["unified", "split"]),
});
export type GitDeviceLayout = z.infer<typeof GitDeviceLayoutSchema>;

const GitLayoutMapSchema = z.object({
  desktop: GitDeviceLayoutSchema.optional(),
  compact: GitDeviceLayoutSchema.optional(),
  tablet: GitDeviceLayoutSchema.optional(),
  phone: GitDeviceLayoutSchema.optional(),
});
type GitLayoutMap = z.infer<typeof GitLayoutMapSchema>;

const GitTargetMemorySchema = z.object({
  mode: GitWorkbenchModeSchema,
  scope: GitHistoryScopeSchema,
  branchFilter: z.string().max(256).nullable(),
  searchQuery: z.string().max(512),
  pathFilter: z.string().max(512),
  selectedCommitOid: z.string().max(64).nullable(),
  selectedParentByCommit: z.record(z.string().max(64), z.string().max(64)),
  selectedCommitFilePath: z.string().max(1024).nullable(),
  activeChangeId: z.string().max(1024).nullable(),
  reviewedPaths: z.array(z.string().max(1024)).max(200),
  commitMessage: z.string().max(8000),
  logListScrollTop: z.number().nonnegative(),
  logDiffScrollTop: z.number().nonnegative(),
  commitListScrollTop: z.number().nonnegative(),
  commitDiffScrollTop: z.number().nonnegative(),
  updatedAt: z.number(),
});
export type GitTargetMemory = z.infer<typeof GitTargetMemorySchema>;

const GitTargetMemoryMapSchema = z.record(
  z.string().min(1).max(200),
  GitTargetMemorySchema,
);

const LAYOUT_STORAGE_KEY = "gitWorkbenchLayout";
const MEMORY_STORAGE_KEY = "gitWorkbenchState";
const MAX_STORED_TARGETS = 24;

// Session-lifetime cache keyed by `${hostKey}::${repoId}::${worktreeId || "__repo__"}`
// so scroll positions and drafts round-trip with zero latency even in private mode.
const memoryCache = new Map<string, GitTargetMemory>();

export function clearGitWorkbenchMemoryCache(): void {
  memoryCache.clear();
}

export interface GitPaneScrollCallbacks {
  diffMode?: "unified" | "split";
  listScrollTop?: number;
  diffScrollTop?: number;
  onResetLayout?: () => void;
  onDiffModeChange?: (mode: "unified" | "split") => void;
  onListScrollChange?: (top: number) => void;
  onDiffScrollChange?: (top: number) => void;
}

export function syncElementScrollTop(
  el: HTMLElement | undefined,
  targetTop: number,
): void {
  if (el && Math.abs(el.scrollTop - targetTop) > 1) {
    el.scrollTop = targetTop;
  }
}

export function resolveGitDeviceClass(width: number): GitDeviceClass {
  if (width >= 1280) return "desktop";
  if (width >= 1024) return "compact";
  if (width >= 640) return "tablet";
  return "phone";
}

function defaultGitDeviceLayout(deviceClass: GitDeviceClass): GitDeviceLayout {
  switch (deviceClass) {
    case "desktop":
      return {
        showBranchesPanel: true,
        showAuthorColumn: true,
        logThreePaneSizes: [20, 46, 34],
        logTwoPaneSizes: [60, 40],
        commitPaneSizes: [40, 60],
        diffMode: "unified",
      };
    case "compact":
      return {
        showBranchesPanel: false,
        showAuthorColumn: true,
        logThreePaneSizes: [22, 44, 34],
        logTwoPaneSizes: [56, 44],
        commitPaneSizes: [42, 58],
        diffMode: "unified",
      };
    case "tablet":
      return {
        showBranchesPanel: false,
        showAuthorColumn: true,
        logThreePaneSizes: [22, 44, 34],
        logTwoPaneSizes: [56, 44],
        commitPaneSizes: [44, 56],
        diffMode: "unified",
      };
    case "phone":
      return {
        showBranchesPanel: false,
        showAuthorColumn: false,
        logThreePaneSizes: [20, 46, 34],
        logTwoPaneSizes: [60, 40],
        commitPaneSizes: [40, 60],
        diffMode: "unified",
      };
  }
}

function readLayoutMap(): GitLayoutMap {
  const raw = stored(LAYOUT_STORAGE_KEY, "");
  if (!raw) return {};
  try {
    return GitLayoutMapSchema.safeParse(JSON.parse(raw)).data ?? {};
  } catch {
    return {};
  }
}

export function readGitDeviceLayout(
  deviceClass: GitDeviceClass,
): GitDeviceLayout {
  const map = readLayoutMap();
  return map[deviceClass] ?? defaultGitDeviceLayout(deviceClass);
}

export function writeGitDeviceLayout(
  deviceClass: GitDeviceClass,
  patch: Partial<GitDeviceLayout>,
): GitDeviceLayout {
  const map = readLayoutMap();
  const base = map[deviceClass] ?? defaultGitDeviceLayout(deviceClass);
  const next: GitDeviceLayout = {
    ...base,
    ...patch,
  };
  const validated =
    GitDeviceLayoutSchema.safeParse(next).data ??
    defaultGitDeviceLayout(deviceClass);
  const updated: GitLayoutMap = {
    ...map,
    [deviceClass]: validated,
  };
  store(LAYOUT_STORAGE_KEY, JSON.stringify(updated));
  return validated;
}

export function resetGitDeviceLayout(
  deviceClass: GitDeviceClass,
): GitDeviceLayout {
  const fallback = defaultGitDeviceLayout(deviceClass);
  const map = readLayoutMap();
  delete map[deviceClass];
  if (Object.keys(map).length === 0) {
    store(LAYOUT_STORAGE_KEY, null);
  } else {
    store(LAYOUT_STORAGE_KEY, JSON.stringify(map));
  }
  return fallback;
}

function gitTargetKey(repositoryId: string, worktreeId?: string): string {
  return `${repositoryId}::${worktreeId?.trim() || "__repo__"}`;
}

function cacheKey(repositoryId: string, worktreeId?: string): string {
  const host = activeHostId() ?? "__default__";
  return `${host}::${gitTargetKey(repositoryId, worktreeId)}`;
}

function defaultGitTargetMemory(): GitTargetMemory {
  return {
    mode: "log",
    scope: "all",
    branchFilter: null,
    searchQuery: "",
    pathFilter: "",
    selectedCommitOid: null,
    selectedParentByCommit: {},
    selectedCommitFilePath: null,
    activeChangeId: null,
    reviewedPaths: [],
    commitMessage: "",
    logListScrollTop: 0,
    logDiffScrollTop: 0,
    commitListScrollTop: 0,
    commitDiffScrollTop: 0,
    updatedAt: 0,
  };
}

function readStoredMemoryMap(): Record<string, GitTargetMemory> {
  const raw = stored(MEMORY_STORAGE_KEY, "");
  if (!raw) return {};
  try {
    return GitTargetMemoryMapSchema.safeParse(JSON.parse(raw)).data ?? {};
  } catch {
    return {};
  }
}

export function readGitTargetMemory(
  repositoryId: string,
  worktreeId?: string,
): GitTargetMemory {
  if (!repositoryId) return defaultGitTargetMemory();
  const cKey = cacheKey(repositoryId, worktreeId);
  const tKey = gitTargetKey(repositoryId, worktreeId);
  const storedMap = readStoredMemoryMap();
  const fromStorage = storedMap[tKey];
  const fromCache = memoryCache.get(cKey);
  if (
    fromCache &&
    (!fromStorage || fromCache.updatedAt >= fromStorage.updatedAt)
  ) {
    return {
      ...fromCache,
      selectedParentByCommit: { ...fromCache.selectedParentByCommit },
      reviewedPaths: [...fromCache.reviewedPaths],
    };
  }
  if (fromStorage) {
    memoryCache.set(cKey, fromStorage);
    return {
      ...fromStorage,
      selectedParentByCommit: { ...fromStorage.selectedParentByCommit },
      reviewedPaths: [...fromStorage.reviewedPaths],
    };
  }
  return defaultGitTargetMemory();
}

export function writeGitTargetMemory(
  repositoryId: string,
  worktreeId: string | undefined,
  patch: Partial<Omit<GitTargetMemory, "updatedAt">>,
): GitTargetMemory {
  if (!repositoryId) return defaultGitTargetMemory();
  const hasWorktree = Boolean(worktreeId?.trim());
  const prev = readGitTargetMemory(repositoryId, worktreeId);
  const candidate: GitTargetMemory = {
    ...prev,
    ...patch,
    // Repository-only views never persist worktree-bound drafts or review marks.
    reviewedPaths: hasWorktree
      ? (patch.reviewedPaths ?? prev.reviewedPaths).slice(0, 200)
      : [],
    commitMessage: hasWorktree
      ? (patch.commitMessage ?? prev.commitMessage).slice(0, 8000)
      : "",
    activeChangeId: hasWorktree
      ? patch.activeChangeId !== undefined
        ? patch.activeChangeId
        : prev.activeChangeId
      : null,
    updatedAt: Date.now(),
  };
  const next =
    GitTargetMemorySchema.safeParse(candidate).data ?? defaultGitTargetMemory();

  memoryCache.set(cacheKey(repositoryId, worktreeId), next);

  const map = readStoredMemoryMap();
  const tKey = gitTargetKey(repositoryId, worktreeId);
  map[tKey] = next;

  const entries = Object.entries(map).sort(
    (a, b) => b[1].updatedAt - a[1].updatedAt,
  );
  const trimmed = Object.fromEntries(entries.slice(0, MAX_STORED_TARGETS));
  store(MEMORY_STORAGE_KEY, JSON.stringify(trimmed));
  return next;
}

export interface GitQueryOverrides {
  view?: GitWorkbenchMode;
  scope?: GitHistoryScope;
  branch?: string;
  commit?: string;
  q?: string;
}

/**
 * Applies explicit URL deep-link query parameters over stored target memory.
 * Explicit URL parameters ALWAYS win over remembered state (`U12`).
 */
export function applyGitQueryOverrides(
  memory: GitTargetMemory,
  query: GitQueryOverrides,
): GitTargetMemory {
  const next: GitTargetMemory = {
    ...memory,
    selectedParentByCommit: { ...memory.selectedParentByCommit },
    reviewedPaths: [...memory.reviewedPaths],
  };
  if (query.view) next.mode = query.view;
  if (query.scope) next.scope = query.scope;
  if (query.branch !== undefined) next.branchFilter = query.branch;
  if (query.commit) {
    next.selectedCommitOid = query.commit;
    if (!query.view) next.mode = "log";
  }
  if (query.q !== undefined) next.searchQuery = query.q;
  return next;
}
