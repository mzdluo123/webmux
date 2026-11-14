import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vite-plus/test";
import type {
  Branches,
  GitCommitDetailResponse,
  GitCompare,
  GitContext,
  GitHistoryPage,
  Repository,
  Worktree,
} from "../../src/contracts/schemas";

const mocks = vi.hoisted(() => ({
  gitContext: vi.fn(),
  gitHistory: vi.fn(),
  gitCommitDetail: vi.fn(),
  gitCompare: vi.fn(),
  gitSnapshotFile: vi.fn(),
  gitSnapshotRaw: vi.fn(),
  gitStaging: vi.fn(),
  repositoryBranches: vi.fn(),
  resolveCalculatedDiff: vi.fn(),
  navigate: vi.fn(),
  replacePath: vi.fn(),
}));

vi.mock("../../src/platform/git-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("../../src/platform/git-api")>();
  return {
    ...actual,
    gitContext: mocks.gitContext,
    gitHistory: mocks.gitHistory,
    gitCommitDetail: mocks.gitCommitDetail,
    gitCompare: mocks.gitCompare,
    gitSnapshotFile: mocks.gitSnapshotFile,
    gitSnapshotRaw: mocks.gitSnapshotRaw,
    gitStaging: mocks.gitStaging,
  };
});

vi.mock("../../src/platform/api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("../../src/platform/api")>();
  return {
    ...actual,
    repositoryBranches: mocks.repositoryBranches,
    navigate: mocks.navigate,
    replacePath: mocks.replacePath,
  };
});

vi.mock("../../src/domains/code/code-diff-client", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("../../src/domains/code/code-diff-client")
    >();
  return {
    ...actual,
    resolveCalculatedDiff: mocks.resolveCalculatedDiff,
  };
});

import { createGitWorkbenchController } from "../../src/hooks/workspace/git-workbench-controller.svelte";

const OID_HEAD = "1111111111111111111111111111111111111111";
const OID_P1 = "2222222222222222222222222222222222222222";
const OID_P2 = "3333333333333333333333333333333333333333";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function makeRepo(): { repo: Repository; wt: Worktree } {
  const wt: Worktree = {
    id: "wt-main",
    path: "/work/repo",
    branch: "main",
    head: OID_HEAD,
    detached: false,
    bare: false,
    kind: "main",
    available: true,
    locked: false,
    prunable: false,
    ahead: 1,
    behind: 0,
    upstream: "origin/main",
    upstreamGone: false,
    dirty: false,
    staged: 0,
    unstaged: 0,
    untracked: 0,
    conflicted: 0,
    operation: "",
    commitSubject: "feat: head commit",
    committedAt: "2026-05-01T10:00:00Z",
  };
  const repo: Repository = {
    id: "repo-1",
    name: "webmux",
    path: "/work/repo",
    availability: "available",
    defaultBranch: "main",
    homeWorktrees: ["wt-main"],
    worktrees: [wt],
  };
  return { repo, wt };
}

function makeHistoryRow(oid: string, subject: string) {
  return {
    oid,
    shortOid: oid.slice(0, 7),
    parents: [OID_P1],
    subject,
    authorName: "Alice",
    authorEmail: "alice@example.com",
    authorAt: "2026-05-01T10:00:00Z",
    committerName: "Alice",
    committerAt: "2026-05-01T10:00:00Z",
    refs: oid === OID_HEAD ? [{ name: "main", kind: "head" as const }] : [],
    lane: 0,
    activeLanes: [0],
    edges: [],
    directParentsMissing: false,
  };
}

function makeDetail(
  oid: string,
  parentOid: string,
  filePath: string,
): GitCommitDetailResponse {
  return {
    oid,
    shortOid: oid.slice(0, 7),
    subject: `subject ${oid.slice(0, 7)}`,
    body: "body text",
    message: `subject ${oid.slice(0, 7)}\n\nbody text`,
    authorName: "Alice",
    authorEmail: "alice@example.com",
    authorAt: "2026-05-01T10:00:00Z",
    committerName: "Alice",
    committerEmail: "alice@example.com",
    committerAt: "2026-05-01T10:00:00Z",
    distinctCommitter: false,
    parents: [
      { oid: parentOid, shortOid: parentOid.slice(0, 7), subject: "p" },
    ],
    comparedParentOid: parentOid,
    refs: [],
    files: [
      {
        oldPath: "",
        newPath: filePath,
        status: "M",
        oldMode: "100644",
        newMode: "100644",
        oldOid: OID_P1,
        newOid: oid,
        modeChanged: false,
        binary: false,
      },
    ],
  };
}

function makeCompareResponse(
  baseOid: string,
  targetOid: string,
  filePath: string,
): GitCompare {
  const emptyDoc = {
    nodeId: "n-1",
    path: filePath,
    absolutePath: `/work/repo/${filePath}`,
    source: `commit:${targetOid}`,
    oid: targetOid,
    contentVersion: "v1",
    kind: "text" as const,
    text: "line 1\n",
    lineCount: 1,
    referenceable: true,
    reason: "",
    mediaType: "text/plain",
    size: 7,
    downloadable: true,
  };
  return {
    baseOid,
    targetOid,
    changes: {
      nextCursor: "",
      total: 1,
      entries: [
        {
          nodeId: "n-1",
          changeId: `chg-${filePath}`,
          group: "commit",
          oldPath: filePath,
          newPath: filePath,
          status: "M",
          oldOid: baseOid || OID_P1,
          newOid: targetOid,
          oldMode: "100644",
          newMode: "100644",
        },
      ],
      contentVersion: "v1",
      checkedAt: "2026-05-01T10:00:00Z",
    },
    diff: {
      snapshotId: `snap-${baseOid.slice(0, 4)}-${targetOid.slice(0, 4)}-${filePath}`,
      old: { ...emptyDoc, source: baseOid ? `commit:${baseOid}` : "empty" },
      new: emptyDoc,
      kind: "text",
      reason: "",
      checkedAt: "2026-05-01T10:00:00Z",
      change: {
        nodeId: "n-1",
        changeId: `chg-${filePath}`,
        group: "commit",
        oldPath: filePath,
        newPath: filePath,
        status: "M",
        oldOid: baseOid || OID_P1,
        newOid: targetOid,
        oldMode: "100644",
        newMode: "100644",
      },
    },
  };
}

describe("Git Log controller (async isolation, pagination, scope forwarding)", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();

    const ctx: GitContext = {
      contextId: "ctx-1",
      rootPath: "/work/repo",
      rootKind: "worktree",
      browseAvailable: true,
      diffAvailable: true,
      ignoreAvailable: true,
      reason: "",
      writableAvailable: true,
    };
    mocks.gitContext.mockResolvedValue(ctx);

    const branches: Branches = {
      defaultBranch: "main",
      defaultBase: "main",
      defaultBaseSubject: "feat: head",
      defaultParent: "/work",
      branches: [
        {
          name: "main",
          head: OID_HEAD,
          subject: "feat: head",
          committedAt: "2026-05-01T10:00:00Z",
          upstream: "origin/main",
          ahead: 1,
          behind: 0,
          upstreamGone: false,
          worktreePaths: ["/work/repo"],
        },
      ],
      bases: [],
      tags: [
        {
          name: "v1.0.0",
          head: OID_P1,
          subject: "release 1.0.0",
          committedAt: "2026-04-30T10:00:00Z",
        },
      ],
    };
    mocks.repositoryBranches.mockResolvedValue(branches);
    mocks.gitStaging.mockResolvedValue({
      conflicted: [],
      staged: [],
      unstaged: [],
    });
    mocks.resolveCalculatedDiff.mockImplementation(async (diff) => ({
      modelSnapshot: diff.snapshotId,
      calculation: "ready",
      calculationReason: "",
      fileDiff: {
        name: diff.new.path,
        type: "change",
        hunks: [],
        cacheKey: diff.snapshotId,
      },
    }));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("discards stale async commit detail and compare responses (U13)", async () => {
    const { repo, wt } = makeRepo();
    const firstPage: GitHistoryPage = {
      status: "ready",
      scope: "all",
      refsChanged: false,
      nextCursor: "",
      rows: [makeHistoryRow(OID_HEAD, "head"), makeHistoryRow(OID_P1, "older")],
    };
    mocks.gitHistory.mockResolvedValue(firstPage);

    const slowHeadDetail = deferred<GitCommitDetailResponse>();
    const fastOlderDetail = deferred<GitCommitDetailResponse>();

    mocks.gitCommitDetail.mockImplementation((_repoId: string, oid: string) => {
      if (oid === OID_HEAD) return slowHeadDetail.promise;
      return fastOlderDetail.promise;
    });
    mocks.gitCompare.mockImplementation(
      async (
        _repoId: string,
        params: { baseOid?: string; targetOid: string; path?: string },
      ) =>
        makeCompareResponse(
          params.baseOid ?? "",
          params.targetOid,
          params.path ?? "file-b.ts",
        ),
    );

    const ctrl = createGitWorkbenchController();
    ctrl.syncRoute({
      repositoryId: repo.id,
      worktreeId: wt.id,
      search: "",
      repository: repo,
      worktree: wt,
      sessions: [],
    });
    await vi.waitFor(() => expect(ctrl.view.commits).toHaveLength(2));

    // User immediately clicks OID_P1 while OID_HEAD detail is still in-flight!
    ctrl.selectCommit(OID_P1);
    fastOlderDetail.resolve(makeDetail(OID_P1, OID_P2, "older.ts"));
    await vi.waitFor(() => expect(ctrl.logDiff.state.path).toBe("older.ts"));

    // Late OID_HEAD response arrives afterwards — must NOT overwrite OID_P1 selection or diff!
    slowHeadDetail.resolve(makeDetail(OID_HEAD, OID_P1, "stale-head.ts"));
    await Promise.resolve();
    await Promise.resolve();

    expect(ctrl.view.selectedCommitOid).toBe(OID_P1);
    expect(ctrl.logDiff.state.path).toBe("older.ts");
    expect(ctrl.logDiff.state.targetOid).toBe(OID_P1);

    // Also verify stale two-commit compare isolation when swapping direction rapidly:
    const slowForwardCompare = deferred<GitCompare>();
    const fastSwappedCompare = deferred<GitCompare>();
    mocks.gitCompare.mockImplementation(
      (
        _repoId: string,
        params: { baseOid?: string; targetOid: string; path?: string },
      ) => {
        if (
          !params.path &&
          params.baseOid === OID_P1 &&
          params.targetOid === OID_HEAD
        ) {
          return slowForwardCompare.promise;
        }
        if (
          !params.path &&
          params.baseOid === OID_HEAD &&
          params.targetOid === OID_P1
        ) {
          return fastSwappedCompare.promise;
        }
        return Promise.resolve(
          makeCompareResponse(
            params.baseOid ?? "",
            params.targetOid,
            params.path ?? "swapped.ts",
          ),
        );
      },
    );

    void ctrl.startCompare(OID_P1, OID_HEAD);
    ctrl.swapCompareDirection();

    fastSwappedCompare.resolve(
      makeCompareResponse(OID_HEAD, OID_P1, "swapped.ts"),
    );
    await vi.waitFor(() =>
      expect(ctrl.view.compareMode?.files[0]?.newPath).toBe("swapped.ts"),
    );

    slowForwardCompare.resolve(
      makeCompareResponse(OID_P1, OID_HEAD, "stale-forward.ts"),
    );
    await Promise.resolve();
    await Promise.resolve();

    expect(ctrl.view.compareMode?.baseOid).toBe(OID_HEAD);
    expect(ctrl.view.compareMode?.targetOid).toBe(OID_P1);
    expect(ctrl.view.compareMode?.files[0]?.newPath).toBe("swapped.ts");

    ctrl.dispose();
  });

  it("appends cursor pages, surfaces refsChanged, and reports disappeared selection with jumpToHead", async () => {
    const { repo, wt } = makeRepo();
    mocks.gitHistory.mockResolvedValueOnce({
      status: "ready",
      scope: "all",
      refsChanged: false,
      nextCursor: "cur-page-2",
      rows: [makeHistoryRow(OID_HEAD, "head")],
    } satisfies GitHistoryPage);
    mocks.gitCommitDetail.mockImplementation(async (_r: string, oid: string) =>
      makeDetail(oid, OID_P1, "main.ts"),
    );
    mocks.gitCompare.mockImplementation(
      async (
        _r: string,
        p: { baseOid?: string; targetOid: string; path?: string },
      ) =>
        makeCompareResponse(p.baseOid ?? "", p.targetOid, p.path ?? "main.ts"),
    );

    const ctrl = createGitWorkbenchController();
    ctrl.syncRoute({
      repositoryId: repo.id,
      worktreeId: wt.id,
      search: "",
      repository: repo,
      worktree: wt,
      sessions: [],
    });
    await vi.waitFor(() => expect(ctrl.view.nextCursor).toBe("cur-page-2"));
    expect(
      ctrl.view.branches.some((b) => b.kind === "tag" && b.name === "v1.0.0"),
    ).toBe(true);

    // Load page 2 with refsChanged: true
    mocks.gitHistory.mockResolvedValueOnce({
      status: "ready",
      scope: "all",
      refsChanged: true,
      nextCursor: "",
      rows: [makeHistoryRow(OID_P1, "page 2 commit")],
    } satisfies GitHistoryPage);

    await ctrl.loadMoreHistory();
    expect(ctrl.view.commits.map((c) => c.oid)).toEqual([OID_HEAD, OID_P1]);
    expect(ctrl.view.refsChanged).toBe(true);
    expect(ctrl.view.nextCursor).toBe("");

    // Select OID_P1, then filter to a branch where OID_P1 is absent -> selectionNotice appears!
    ctrl.selectCommit(OID_P1);
    mocks.gitHistory.mockResolvedValueOnce({
      status: "ready",
      scope: "all",
      refsChanged: false,
      nextCursor: "",
      rows: [makeHistoryRow(OID_P2, "other branch tip")],
    } satisfies GitHistoryPage);

    ctrl.setBranchFilter("feat/other");
    await vi.waitFor(() => expect(ctrl.view.selectedCommitOid).toBe(OID_P2));
    expect(ctrl.view.selectionNotice?.canJumpHead).toBe(true);
    expect(ctrl.view.selectionNotice?.message).toContain(OID_P1.slice(0, 7));

    // Jump back to HEAD clears restricting filters and selects OID_HEAD!
    mocks.gitHistory.mockResolvedValueOnce({
      status: "ready",
      scope: "all",
      refsChanged: false,
      nextCursor: "",
      rows: [makeHistoryRow(OID_HEAD, "head")],
    } satisfies GitHistoryPage);
    ctrl.jumpToHead();
    await vi.waitFor(() => expect(ctrl.view.selectedCommitOid).toBe(OID_HEAD));
    expect(ctrl.view.branchFilter).toBeNull();
    expect(ctrl.view.selectionNotice).toBeNull();

    ctrl.dispose();
  });

  it("forwards non-all scope in repository-only mode so backend no_worktree status is surfaced", async () => {
    const { repo } = makeRepo();
    mocks.gitContext.mockResolvedValue({
      contextId: "ctx-repo",
      rootPath: "/work/repo",
      rootKind: "repository",
      browseAvailable: false,
      diffAvailable: true,
      ignoreAvailable: false,
      reason: "",
      writableAvailable: false,
    });
    mocks.gitHistory.mockResolvedValueOnce({
      status: "ready",
      scope: "all",
      refsChanged: false,
      nextCursor: "",
      rows: [makeHistoryRow(OID_HEAD, "head")],
    } satisfies GitHistoryPage);
    mocks.gitCommitDetail.mockResolvedValue(
      makeDetail(OID_HEAD, OID_P1, "a.ts"),
    );
    mocks.gitCompare.mockResolvedValue(
      makeCompareResponse(OID_P1, OID_HEAD, "a.ts"),
    );

    const ctrl = createGitWorkbenchController();
    ctrl.syncRoute({
      repositoryId: repo.id,
      worktreeId: undefined,
      search: "",
      repository: repo,
      worktree: undefined,
      sessions: [],
    });
    await vi.waitFor(() => expect(ctrl.view.commits).toHaveLength(1));

    mocks.gitHistory.mockResolvedValueOnce({
      status: "no_worktree",
      reason: "当前未选择工作树，无法查看待推送提交",
      scope: "to-push",
      refsChanged: false,
      nextCursor: "",
      rows: [],
    } satisfies GitHistoryPage);

    ctrl.setScope("to-push");
    await vi.waitFor(() => expect(ctrl.view.historyState).toBe("no_worktree"));
    expect(mocks.gitHistory).toHaveBeenLastCalledWith(
      "repo-1",
      expect.objectContaining({ scope: "to-push", worktreeId: undefined }),
      expect.any(AbortSignal),
    );
    expect(ctrl.view.historyReason).toContain("无法查看待推送提交");

    ctrl.dispose();
  });
});
