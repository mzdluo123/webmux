import { repositoryBranches } from "../../platform/api";
import { errorMessage } from "../../platform/error";
import { gitCompare } from "../../platform/git-api";
import { gitFileHistory, gitRevisionDiff } from "../../platform/git-v2-api";
import type { GitFileHistoryRow } from "../../contracts/schemas";
import type {
  GitBranchItem,
  GitCommitFile,
} from "../../contracts/git-workbench-schemas";
import {
  mapGitBranchesResponse,
  normalizeCommitFileStatus,
} from "../../domains/workspace/git-workbench-adapter";
import { LatestRequest } from "../shared/remote-request.svelte";
import { createGitBlameController } from "./git-blame-controller.svelte";
import { createGitSnapshotDiffReader } from "./git-snapshot-diff-reader.svelte";

export type GitFileInspectorTab = "history" | "blame" | "compare";

export interface GitFileInspectorDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getBranches?: () => readonly GitBranchItem[];
  getHeadOid?: () => string | null;
  onStatusBanner?: (message: string) => void;
}

function historyRowToCommitFile(row: GitFileHistoryRow): GitCommitFile {
  return {
    oldPath: row.oldPath || row.path,
    newPath: row.path,
    status: normalizeCommitFileStatus(row.status),
    binary: false,
    additions: 0,
    deletions: 0,
    hunks: [],
  };
}

export function createGitFileInspectorController(deps: GitFileInspectorDeps) {
  let open = $state(false);
  let tab = $state<GitFileInspectorTab>("history");
  let filePath = $state("");
  let initialRev = $state("");

  let historyRows = $state<GitFileHistoryRow[]>([]);
  let historyNextCursor = $state("");
  let historyLoading = $state(false);
  let historyLoadingMore = $state(false);
  let historyError = $state("");
  let selectedHistoryOid = $state<string | null>(null);

  let compareBaseOid = $state("");
  let compareBaseLabel = $state("");
  let compareTargetKind = $state<"worktree" | "rev">("worktree");
  let compareTargetOid = $state("");
  let fallbackBranches = $state<GitBranchItem[]>([]);

  const historyReq = new LatestRequest();
  let disposed = false;

  const blame = createGitBlameController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
  });

  const historyReader = createGitSnapshotDiffReader({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
    onStatusBanner: (m) => deps.onStatusBanner?.(m),
  });

  const compareReader = createGitSnapshotDiffReader({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
    onStatusBanner: (m) => deps.onStatusBanner?.(m),
  });

  const availableRefs = $derived.by(() => {
    const fromParent = deps.getBranches?.() ?? [];
    return fromParent.length > 0 ? fromParent : fallbackBranches;
  });

  const selectedHistoryRow = $derived.by(() =>
    selectedHistoryOid
      ? (historyRows.find((r) => r.oid === selectedHistoryOid) ?? null)
      : null,
  );

  function dispose() {
    disposed = true;
    historyReq.dispose();
    blame.dispose();
    historyReader.dispose();
    compareReader.dispose();
  }

  function resetTarget() {
    historyReq.cancel();
    blame.resetTarget();
    historyReader.resetTarget();
    compareReader.resetTarget();
    open = false;
    filePath = "";
    initialRev = "";
    historyRows = [];
    historyNextCursor = "";
    historyError = "";
    selectedHistoryOid = null;
    fallbackBranches = [];
  }

  function close() {
    historyReq.cancel();
    open = false;
  }

  async function ensureRefsLoaded() {
    if (
      (deps.getBranches?.() ?? []).length > 0 ||
      fallbackBranches.length > 0
    ) {
      return;
    }
    const repoId = deps.getRepoId();
    if (!repoId) return;
    const res = await repositoryBranches(repoId).catch(() => null);
    if (res && !disposed && repoId === deps.getRepoId()) {
      fallbackBranches = mapGitBranchesResponse(undefined, null, [], res);
    }
  }

  async function loadHistoryRowDiff(row: GitFileHistoryRow) {
    const repoId = deps.getRepoId();
    if (!repoId) return;
    await historyReader.loadFetchedDiff({
      baseOid: row.comparedParentOid ?? "",
      targetOid: row.oid,
      leftLabel: row.comparedParentOid
        ? `父提交 (${row.comparedParentOid.slice(0, 7)})`
        : "空白根提交",
      rightLabel: `${row.shortOid} · ${row.path}`,
      file: historyRowToCommitFile(row),
      missingReason: "未能读取该提交下的文件差异。",
      fetchDiff: async (contextId, signal) => {
        const res = await gitCompare(
          repoId,
          {
            contextId,
            worktreeId: deps.getWorktreeId(),
            baseOid: row.comparedParentOid || "",
            targetOid: row.oid,
            path: row.path,
          },
          signal,
        );
        return res.diff ?? null;
      },
    });
  }

  function selectHistoryRow(row: GitFileHistoryRow | null) {
    if (!row) {
      selectedHistoryOid = null;
      historyReader.clearDiff();
      return;
    }
    selectedHistoryOid = row.oid;
    void loadHistoryRowDiff(row);
  }

  async function loadHistory(reset = true) {
    const repoId = deps.getRepoId();
    if (!repoId || !filePath || disposed) return;
    if (reset) {
      historyLoading = true;
      historyError = "";
    } else {
      if (!historyNextCursor || historyLoadingMore) return;
      historyLoadingMore = true;
    }
    const req = historyReq.begin();
    try {
      const page = await gitFileHistory(
        repoId,
        {
          path: filePath,
          rev: initialRev || undefined,
          worktreeId: deps.getWorktreeId(),
          cursor: reset ? undefined : historyNextCursor,
          limit: 50,
        },
        req.signal,
      );
      if (!req.current() || disposed || repoId !== deps.getRepoId()) return;
      const combined = reset ? page.commits : [...historyRows, ...page.commits];
      historyRows = combined;
      historyNextCursor = page.nextCursor ?? "";
      if (reset) {
        const first = combined[0] ?? null;
        selectedHistoryOid = first?.oid ?? null;
        if (first) void loadHistoryRowDiff(first);
        else historyReader.clearDiff();
      }
    } catch (err) {
      if (req.reportable()) historyError = errorMessage(err);
    } finally {
      if (req.current()) {
        historyLoading = false;
        historyLoadingMore = false;
      }
    }
  }

  async function loadRevisionCompare() {
    const repoId = deps.getRepoId();
    const wtId = deps.getWorktreeId();
    const cleanBase = compareBaseOid.trim();
    if (!repoId || !filePath || !cleanBase) {
      compareReader.clearDiff();
      return;
    }
    const useWorktree = compareTargetKind === "worktree" && Boolean(wtId);
    const targetRev =
      compareTargetOid.trim() || initialRev || deps.getHeadOid?.() || "";
    if (!useWorktree && !targetRev) {
      compareReader.clearDiff();
      return;
    }
    const labelBase = compareBaseLabel || cleanBase.slice(0, 12);
    const dummyFile: GitCommitFile = {
      oldPath: filePath,
      newPath: filePath,
      status: "M",
      binary: false,
      additions: 0,
      deletions: 0,
      hunks: [],
    };
    await compareReader.loadFetchedDiff({
      baseOid: cleanBase,
      targetOid: useWorktree ? "WORKTREE" : targetRev,
      leftLabel: `基准：${labelBase}`,
      rightLabel: useWorktree
        ? `当前工作区文件 (${filePath})`
        : `目标：${targetRev.slice(0, 12)}`,
      file: dummyFile,
      missingReason: "未能读取所选修订版本的文件差异。",
      fetchDiff: async (contextId, signal) => {
        if (useWorktree && wtId) {
          return gitRevisionDiff(
            repoId,
            { worktreeId: wtId, contextId, baseOid: cleanBase, path: filePath },
            signal,
          );
        }
        const res = await gitCompare(
          repoId,
          {
            contextId,
            worktreeId: wtId,
            baseOid: cleanBase,
            targetOid: targetRev,
            path: filePath,
          },
          signal,
        );
        return res.diff ?? null;
      },
    });
  }

  function switchTab(nextTab: GitFileInspectorTab) {
    tab = nextTab;
    if (nextTab === "history" && historyRows.length === 0 && !historyLoading) {
      void loadHistory(true);
    } else if (nextTab === "blame" && !blame.data && !blame.loading) {
      void blame.load(blame.rev || initialRev, blame.path || filePath, false);
    } else if (nextTab === "compare") {
      void ensureRefsLoaded().then(() => {
        if (!compareBaseOid) {
          const head =
            deps.getHeadOid?.() || initialRev || availableRefs[0]?.head || "";
          compareBaseOid = head;
          compareBaseLabel = head ? head.slice(0, 7) : "";
        }
        if (compareBaseOid) void loadRevisionCompare();
      });
    }
  }

  function openInspector(options: {
    path: string;
    tab?: GitFileInspectorTab;
    rev?: string;
    compareBaseOid?: string;
    compareBaseLabel?: string;
  }) {
    const cleanPath = options.path.trim();
    if (!cleanPath) return;
    const sameTarget =
      filePath === cleanPath && initialRev === (options.rev ?? "");
    filePath = cleanPath;
    initialRev = options.rev ?? "";
    if (!sameTarget) {
      historyRows = [];
      historyNextCursor = "";
      historyError = "";
      selectedHistoryOid = null;
      historyReader.clearDiff();
      blame.primeTarget(cleanPath, initialRev);
      compareReader.clearDiff();
    }
    compareTargetKind = deps.getWorktreeId() ? "worktree" : "rev";
    compareTargetOid = initialRev || deps.getHeadOid?.() || "";
    if (options.compareBaseOid) {
      compareBaseOid = options.compareBaseOid;
      compareBaseLabel =
        options.compareBaseLabel ?? options.compareBaseOid.slice(0, 7);
    } else if (!sameTarget) {
      compareBaseOid = "";
      compareBaseLabel = "";
    }
    open = true;
    switchTab(options.tab ?? "history");
  }

  return {
    blame,
    get open() {
      return open;
    },
    get tab() {
      return tab;
    },
    get filePath() {
      return filePath;
    },
    get initialRev() {
      return initialRev;
    },
    get historyRows() {
      return historyRows;
    },
    get historyNextCursor() {
      return historyNextCursor;
    },
    get historyLoading() {
      return historyLoading;
    },
    get historyLoadingMore() {
      return historyLoadingMore;
    },
    get historyError() {
      return historyError;
    },
    get selectedHistoryOid() {
      return selectedHistoryOid;
    },
    get selectedHistoryRow() {
      return selectedHistoryRow;
    },
    historyReader: {
      get state() {
        return historyReader.state;
      },
      openSnapshotSide: historyReader.openSnapshotSide,
      closeSnapshotSide: historyReader.closeSnapshotSide,
      downloadSnapshotSide: historyReader.downloadSnapshotSide,
      retryActiveDiff: () => {
        if (selectedHistoryRow) void loadHistoryRowDiff(selectedHistoryRow);
      },
    },
    get availableRefs() {
      return availableRefs;
    },
    get compareBaseOid() {
      return compareBaseOid;
    },
    get compareBaseLabel() {
      return compareBaseLabel;
    },
    setCompareBase(oid: string, label = "") {
      compareBaseOid = oid;
      compareBaseLabel = label;
      void loadRevisionCompare();
    },
    get compareTargetKind() {
      return compareTargetKind;
    },
    setCompareTargetKind(kind: "worktree" | "rev") {
      compareTargetKind = kind;
      void loadRevisionCompare();
    },
    get compareTargetOid() {
      return compareTargetOid;
    },
    setCompareTargetOid(oid: string) {
      compareTargetOid = oid;
      void loadRevisionCompare();
    },
    compareReader: {
      get state() {
        return compareReader.state;
      },
      openSnapshotSide: compareReader.openSnapshotSide,
      closeSnapshotSide: compareReader.closeSnapshotSide,
      downloadSnapshotSide: compareReader.downloadSnapshotSide,
      retryActiveDiff: () => void loadRevisionCompare(),
    },
    seedContextId(repoId: string, wtId: string | undefined, cid: string) {
      historyReader.seedContextId(repoId, wtId, cid);
      compareReader.seedContextId(repoId, wtId, cid);
    },
    dispose,
    resetTarget,
    close,
    openInspector,
    switchTab,
    selectHistoryRow,
    loadMoreHistory: () => loadHistory(false),
    reloadHistory: () => loadHistory(true),
    loadRevisionCompare,
  };
}

export type GitFileInspectorController = ReturnType<
  typeof createGitFileInspectorController
>;
