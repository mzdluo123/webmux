import {
  gitStashDetail,
  gitStashDiff,
  gitStashes,
} from "../../platform/git-stash-api";
import type {
  GitInProgressOperation,
  GitStashDetail,
  GitStashFileChange,
} from "../../contracts/schemas";
import type {
  GitCommitFile,
  GitWorkbenchViewState,
} from "../../contracts/git-workbench-schemas";
import { normalizeCommitFileStatus } from "../../domains/workspace/git-workbench-adapter";
import { LatestRequest } from "../shared/remote-request.svelte";
import type { GitOperationsController } from "./git-operations-controller.svelte";
import { createGitSnapshotDiffReader } from "./git-snapshot-diff-reader.svelte";
import {
  createGitStashDialogsController,
  parseStashApiError,
} from "./git-stash-dialogs.svelte";

export interface GitStashControllerDeps {
  getRepoId: () => string;
  getWorktreeId: () => string | undefined;
  getView: () => GitWorkbenchViewState;
  patchView: (patch: Partial<GitWorkbenchViewState>) => void;
  operations?: GitOperationsController;
  onAfterStashMutation: () => Promise<void>;
  onInProgressUpdated: (next: GitInProgressOperation) => void;
  onOpenConflictResolver: (path?: string) => void;
}

function toCommitFile(file: GitStashFileChange): GitCommitFile {
  const path = file.newPath || file.oldPath;
  const status =
    file.status.trim() === "?" ? "A" : normalizeCommitFileStatus(file.status);
  return {
    oldPath: file.oldPath || path,
    newPath: path,
    status,
    binary: file.binary,
    additions: 0,
    deletions: 0,
    hunks: [],
  };
}

export function stashFileKey(file: GitStashFileChange): string {
  const path = file.newPath || file.oldPath;
  return `${file.untracked ? "u" : "t"}:${path}`;
}

export function createGitStashController(deps: GitStashControllerDeps) {
  let loadingList = $state(false);
  let loadingDetail = $state(false);
  let selectedStashOid = $state<string | null>(null);
  let detail = $state<GitStashDetail | null>(null);
  let selectedFileKey = $state<string | null>(null);

  const listReq = new LatestRequest();
  const detailReq = new LatestRequest();
  let disposed = false;

  const reader = createGitSnapshotDiffReader({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
    onStatusBanner: (statusBanner) => deps.patchView({ statusBanner }),
  });

  const dialogs = createGitStashDialogsController({
    getRepoId: deps.getRepoId,
    getWorktreeId: deps.getWorktreeId,
    patchView: deps.patchView,
    operations: deps.operations,
    onRefreshList: (preferredOid) =>
      refreshList(deps.getRepoId(), undefined, preferredOid),
    onAfterStashMutation: deps.onAfterStashMutation,
    onInProgressUpdated: deps.onInProgressUpdated,
    onOpenConflictResolver: deps.onOpenConflictResolver,
  });

  const entries = $derived(deps.getView().stashes ?? []);
  const selectedFile = $derived.by(() => {
    if (!detail || !selectedFileKey) return null;
    return (
      detail.files.find((f) => stashFileKey(f) === selectedFileKey) ?? null
    );
  });

  function dispose() {
    disposed = true;
    listReq.dispose();
    detailReq.dispose();
    dialogs.dispose();
    reader.dispose();
  }

  function resetTarget() {
    listReq.cancel();
    detailReq.cancel();
    dialogs.resetTarget();
    reader.resetTarget();
    loadingList = false;
    loadingDetail = false;
    selectedStashOid = null;
    detail = null;
    selectedFileKey = null;
  }

  async function loadFileDiff(
    stashDetail: GitStashDetail,
    file: GitStashFileChange,
  ) {
    const entry = stashDetail.entry;
    const path = file.newPath || file.oldPath;
    const baseShort = entry.baseOid ? entry.baseOid.slice(0, 7) : "";
    await reader.loadFetchedDiff({
      baseOid: entry.baseOid ?? "",
      targetOid: entry.oid,
      leftLabel: file.untracked
        ? "工作区未跟踪基准 (空)"
        : `基准提交 (${baseShort || "HEAD"})`,
      rightLabel: `${entry.stashRef} (${entry.shortOid})${file.untracked ? " · 未跟踪 (S^3)" : ""}`,
      file: toCommitFile(file),
      missingReason: "未能读取此贮藏文件的差异快照。",
      fetchDiff: (contextId, signal) =>
        gitStashDiff(
          deps.getRepoId(),
          entry.oid,
          {
            contextId,
            path,
            untracked: file.untracked || undefined,
            worktreeId: deps.getWorktreeId(),
          },
          signal,
        ),
    });
  }

  function selectFile(file: GitStashFileChange | null) {
    if (!file || !detail) {
      selectedFileKey = null;
      reader.clearDiff();
      return;
    }
    selectedFileKey = stashFileKey(file);
    void loadFileDiff(detail, file);
  }

  async function selectStash(stashOid: string | null, force = false) {
    const repoId = deps.getRepoId();
    if (!repoId || !stashOid || disposed) {
      selectedStashOid = null;
      detail = null;
      selectedFileKey = null;
      reader.clearDiff();
      return;
    }
    if (
      !force &&
      selectedStashOid === stashOid &&
      detail?.entry.oid === stashOid
    ) {
      return;
    }
    selectedStashOid = stashOid;
    loadingDetail = true;
    const req = detailReq.begin();
    try {
      const res = await gitStashDetail(repoId, stashOid, req.signal);
      if (!req.current() || disposed || repoId !== deps.getRepoId()) return;
      detail = res;
      const firstFile = res.files[0] ?? null;
      selectedFileKey = firstFile ? stashFileKey(firstFile) : null;
      if (firstFile) {
        await loadFileDiff(res, firstFile);
      } else {
        reader.clearDiff();
      }
    } catch (err) {
      if (req.reportable()) {
        deps.patchView({
          statusBanner: `贮藏详情读取失败：${parseStashApiError(err)}`,
        });
      }
    } finally {
      if (req.current()) loadingDetail = false;
    }
  }

  async function refreshList(
    repoId = deps.getRepoId(),
    signal?: AbortSignal,
    preferredOid?: string | null,
  ) {
    if (!repoId || disposed) return;
    loadingList = true;
    const req = listReq.begin();
    const sig = signal ?? req.signal;
    try {
      const res = await gitStashes(repoId, sig);
      if (
        !req.current() ||
        disposed ||
        sig.aborted ||
        repoId !== deps.getRepoId()
      ) {
        return;
      }
      deps.patchView({ stashes: res.stashes });
      const candidate =
        preferredOid !== undefined ? preferredOid : selectedStashOid;
      const matched =
        (candidate && res.stashes.find((e) => e.oid === candidate)?.oid) ??
        res.stashes[0]?.oid ??
        null;
      if (matched) {
        await selectStash(matched, true);
      } else {
        selectedStashOid = null;
        detail = null;
        selectedFileKey = null;
        reader.clearDiff();
      }
    } catch {
      /* Preserve existing stash list on transient reload error */
    } finally {
      if (req.current()) loadingList = false;
    }
  }

  function ensureSelectionLoaded() {
    if (!selectedStashOid && entries.length > 0) {
      void selectStash(entries[0]!.oid);
    }
  }

  return {
    get entries() {
      return entries;
    },
    get loadingList() {
      return loadingList;
    },
    get loadingDetail() {
      return loadingDetail;
    },
    get selectedStashOid() {
      return selectedStashOid;
    },
    get detail() {
      return detail;
    },
    get selectedFileKey() {
      return selectedFileKey;
    },
    get selectedFile() {
      return selectedFile;
    },
    reader: {
      get state() {
        return reader.state;
      },
      openSnapshotSide: reader.openSnapshotSide,
      closeSnapshotSide: reader.closeSnapshotSide,
      downloadSnapshotSide: reader.downloadSnapshotSide,
      retryActiveDiff: () => {
        if (detail && selectedFile) void loadFileDiff(detail, selectedFile);
      },
    },
    dialogs,
    seedContextId: reader.seedContextId,
    dispose,
    resetTarget,
    refreshList,
    ensureSelectionLoaded,
    selectStash,
    selectFile,
  };
}
