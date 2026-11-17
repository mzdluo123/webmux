import { readFileSync } from "node:fs";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  test,
  vi,
} from "vite-plus/test";
import { APIError } from "../src/platform/api";
import {
  gitChanges,
  gitCommit,
  gitCommitDetail,
  gitCommitPreview,
  gitCompare,
  gitConflictReason,
  gitContext,
  gitDiff,
  gitFile,
  gitFiles,
  gitHistory,
  gitSnapshotFile,
  gitSnapshotRaw,
  gitStage,
  gitStaging,
  gitStagingDiff,
  gitTree,
  gitUnstage,
  type GitChangesParams,
  type GitCompareParams,
  type GitDiffParams,
  type GitFileParams,
  type GitFilesParams,
  type GitHistoryParams,
  type GitSnapshotParams,
  type GitStagingDiffParams,
  type GitTreeParams,
} from "../src/platform/git-api";
import {
  GitChangeSchema,
  GitChangesSchema,
  GitCommitDetailResponseSchema,
  GitCommitFileChangeSchema,
  GitCommitIdentitySchema,
  GitCommitOutcomeStatusSchema,
  GitCommitParentSchema,
  GitCommitPreviewSchema,
  GitCommitRequestSchema,
  GitCommitResultSchema,
  GitCompareSchema,
  GitConflictReasonSchema,
  GitConflictSchema,
  GitContextSchema,
  GitDiffSchema,
  GitDocumentSchema,
  GitFileSchema,
  GitHistoryEdgeSchema,
  GitHistoryItemSchema,
  GitHistoryPageSchema,
  GitHistoryScopeSchema,
  GitHistoryStatusSchema,
  GitRefBadgeSchema,
  GitSnapshotSideSchema,
  GitSourceSchema,
  GitStageAreaSchema,
  GitStageFileIdentitySchema,
  GitStageRequestSchema,
  GitStagingEntrySchema,
  GitStagingFileDiffSchema,
  GitStagingHunkLineSchema,
  GitStagingHunkSchema,
  GitStagingOverviewSchema,
  type GitChange,
  type GitChanges,
  type GitCommitDetailResponse,
  type GitCommitFileChange,
  type GitCommitIdentity,
  type GitCommitOutcomeStatus,
  type GitCommitParent,
  type GitCommitPreview,
  type GitCommitRequest,
  type GitCommitResult,
  type GitCompare,
  type GitConflict,
  type GitConflictReason,
  type GitContext,
  type GitDiff,
  type GitDocument,
  type GitFile,
  type GitHistoryEdge,
  type GitHistoryItem,
  type GitHistoryPage,
  type GitHistoryScope,
  type GitHistoryStatus,
  type GitRefBadge,
  type GitSnapshotSide,
  type GitSource,
  type GitStageArea,
  type GitStageFileIdentity,
  type GitStageRequest,
  type GitStagingEntry,
  type GitStagingFileDiff,
  type GitStagingHunk,
  type GitStagingHunkLine,
  type GitStagingOverview,
} from "../src/contracts/schemas";

function readWireBody(name: string): unknown {
  const raw = JSON.parse(
    readFileSync(
      new URL(`../../internal/api/testdata/wire/${name}.json`, import.meta.url),
      "utf8",
    ),
  ) as { body: unknown };
  return JSON.parse(
    JSON.stringify(raw.body).replaceAll('"<time>"', '"2026-01-02T03:04:05Z"'),
  );
}

beforeEach(() => {
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => (key === "webmux.token" ? "secret-token" : null),
  });
  vi.stubGlobal("window", new EventTarget());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Git API client & wire contract schemas", () => {
  test("validates recorded wire snapshots across all read, staging, and commit endpoints", async () => {
    const calls: Array<{ url: string; method: string; body?: string }> = [];
    const routes = new Map<string, { status: number; body: unknown }>([
      [
        "GET /api/repositories/repo-1/git/context?worktreeId=wt-1",
        { status: 200, body: readWireBody("git-context-worktree") },
      ],
      [
        "GET /api/repositories/repo-1/git/tree?contextId=ctx-1&includeIgnored=false",
        { status: 200, body: readWireBody("git-tree") },
      ],
      [
        "GET /api/repositories/repo-1/git/files?contextId=ctx-1&q=b.txt",
        { status: 200, body: readWireBody("git-files") },
      ],
      [
        "GET /api/repositories/repo-1/git/file?contextId=ctx-1&nodeId=YS50eHQ&source=commit%3Aecc8da449d18daa48261db409c9e0af26550cf64",
        { status: 200, body: readWireBody("git-file-commit") },
      ],
      [
        "GET /api/repositories/repo-1/git/changes?contextId=ctx-1&group=unstaged",
        { status: 200, body: readWireBody("git-changes") },
      ],
      [
        "GET /api/repositories/repo-1/git/diff?contextId=ctx-1&changeId=chg-1",
        { status: 200, body: readWireBody("git-diff") },
      ],
      [
        "GET /api/repositories/repo-1/git/compare?contextId=ctx-1&baseOid=2de8fe1f09b18871f913bbafa26fee479279c2b6&targetOid=ecc8da449d18daa48261db409c9e0af26550cf64&path=sub%2Fb.txt",
        { status: 200, body: readWireBody("git-compare") },
      ],
      [
        "GET /api/repositories/repo-1/git/snapshots/snap-1/file?side=after",
        { status: 200, body: readWireBody("git-snapshot-file") },
      ],
      [
        "GET /api/repositories/repo-1/git/history?worktreeId=wt-1&scope=all&limit=50",
        { status: 200, body: readWireBody("git-history") },
      ],
      [
        "GET /api/repositories/repo-1/git/commits/ecc8da449d18daa48261db409c9e0af26550cf64?parentOid=2de8fe1f09b18871f913bbafa26fee479279c2b6",
        { status: 200, body: readWireBody("git-commit-detail") },
      ],
      [
        "GET /api/repositories/repo-1/worktrees/wt-1/git/staging",
        { status: 200, body: readWireBody("git-staging") },
      ],
      [
        "GET /api/repositories/repo-1/worktrees/wt-1/git/staging/diff?area=unstaged&path=a.txt",
        { status: 200, body: readWireBody("git-staging-diff") },
      ],
      [
        "POST /api/repositories/repo-1/worktrees/wt-1/git/stage",
        { status: 204, body: null },
      ],
      [
        "POST /api/repositories/repo-1/worktrees/wt-1/git/unstage",
        { status: 204, body: null },
      ],
      [
        "POST /api/repositories/repo-1/worktrees/wt-1/git/commit-preview",
        { status: 200, body: readWireBody("git-commit-preview") },
      ],
      [
        "POST /api/repositories/repo-1/worktrees/wt-1/git/commit",
        { status: 200, body: readWireBody("git-commit") },
      ],
    ]);

    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = init?.method ?? "GET";
        const body = typeof init?.body === "string" ? init.body : undefined;
        calls.push({ url, method, body });
        expect(init?.headers).toMatchObject({
          Authorization: "Bearer secret-token",
        });
        const hit = routes.get(`${method} ${url}`);
        if (!hit) {
          throw new Error(`Unexpected request: ${method} ${url}`);
        }
        return new Response(
          hit.status === 204 ? null : JSON.stringify(hit.body),
          {
            status: hit.status,
            headers:
              hit.status === 204 ? {} : { "Content-Type": "application/json" },
          },
        );
      }),
    );

    const ctx: GitContext = await gitContext("repo-1", "wt-1");
    expect(GitContextSchema.parse(ctx).rootKind).toBe("worktree");

    const treeParams: GitTreeParams = {
      contextId: "ctx-1",
      includeIgnored: false,
    };
    const tree = await gitTree("repo-1", treeParams);
    expect(tree.children.length).toBeGreaterThan(0);

    const filesParams: GitFilesParams = { contextId: "ctx-1", q: "b.txt" };
    const found = await gitFiles("repo-1", filesParams);
    expect(found.children[0]?.path).toBe("sub/b.txt");

    const source: GitSource = GitSourceSchema.parse(
      "commit:ecc8da449d18daa48261db409c9e0af26550cf64",
    );
    const fileParams: GitFileParams = {
      contextId: "ctx-1",
      nodeId: "YS50eHQ",
      source,
    };
    const file: GitFile = await gitFile("repo-1", fileParams);
    const doc: GitDocument = GitDocumentSchema.parse(file.document);
    expect(GitFileSchema.parse(file).document.path).toBe(doc.path);

    const changeParams: GitChangesParams = {
      contextId: "ctx-1",
      group: "unstaged",
    };
    const changes: GitChanges = await gitChanges("repo-1", changeParams);
    const firstChange: GitChange = GitChangeSchema.parse(changes.entries[0]);
    expect(GitChangesSchema.parse(changes).entries[0]?.newPath).toBe(
      firstChange.newPath,
    );

    const diffParams: GitDiffParams = { contextId: "ctx-1", changeId: "chg-1" };
    const diff: GitDiff = await gitDiff("repo-1", diffParams);
    expect(GitDiffSchema.parse(diff).kind).toBe("text");

    const compareParams: GitCompareParams = {
      contextId: "ctx-1",
      baseOid: "2de8fe1f09b18871f913bbafa26fee479279c2b6",
      targetOid: "ecc8da449d18daa48261db409c9e0af26550cf64",
      path: "sub/b.txt",
    };
    const compared: GitCompare = await gitCompare("repo-1", compareParams);
    expect(GitCompareSchema.parse(compared).diff?.change.group).toBe("commit");

    const side: GitSnapshotSide = GitSnapshotSideSchema.parse("after");
    const snapParams: GitSnapshotParams = { side };
    const snapFile = await gitSnapshotFile("repo-1", "snap-1", snapParams);
    expect(snapFile.document.text).toContain("butterfly");

    const scope: GitHistoryScope = GitHistoryScopeSchema.parse("all");
    const historyParams: GitHistoryParams = {
      worktreeId: "wt-1",
      scope,
      limit: 50,
    };
    const history: GitHistoryPage = await gitHistory("repo-1", historyParams);
    const status: GitHistoryStatus = GitHistoryStatusSchema.parse(
      history.status,
    );
    const row: GitHistoryItem = GitHistoryItemSchema.parse(history.rows[0]);
    const edge: GitHistoryEdge = GitHistoryEdgeSchema.parse(row.edges[0]);
    const badge: GitRefBadge = GitRefBadgeSchema.parse(row.refs[0]);
    expect(status).toBe("ready");
    expect(edge.kind).toBe("direct");
    expect(badge.kind).toBe("head");
    expect(GitHistoryPageSchema.parse(history).rows).toHaveLength(2);

    const detail: GitCommitDetailResponse = await gitCommitDetail(
      "repo-1",
      "ecc8da449d18daa48261db409c9e0af26550cf64",
      "2de8fe1f09b18871f913bbafa26fee479279c2b6",
    );
    const parent: GitCommitParent = GitCommitParentSchema.parse(
      detail.parents[0],
    );
    const changedFile: GitCommitFileChange = GitCommitFileChangeSchema.parse(
      detail.files[0],
    );
    expect(parent.shortOid).toBe("2de8fe1f");
    expect(changedFile.newPath).toBe("sub/b.txt");
    expect(GitCommitDetailResponseSchema.parse(detail).message).toContain(
      "commit body line",
    );

    const staging: GitStagingOverview = await gitStaging("repo-1", "wt-1");
    const entry: GitStagingEntry = GitStagingEntrySchema.parse(
      staging.unstaged[0],
    );
    const area: GitStageArea = GitStageAreaSchema.parse(entry.area);
    expect(area).toBe("unstaged");
    expect(GitStagingOverviewSchema.parse(staging).unborn).toBe(false);

    const stagingDiffParams: GitStagingDiffParams = {
      area: "unstaged",
      path: "a.txt",
    };
    const stagingDiff: GitStagingFileDiff = await gitStagingDiff(
      "repo-1",
      "wt-1",
      stagingDiffParams,
    );
    const identity: GitStageFileIdentity = GitStageFileIdentitySchema.parse(
      stagingDiff.identity,
    );
    const hunk: GitStagingHunk = GitStagingHunkSchema.parse(
      stagingDiff.hunks[0],
    );
    const line: GitStagingHunkLine = GitStagingHunkLineSchema.parse(
      hunk.lines[0],
    );
    expect(line.lineEnding).toBe("lf");
    expect(GitStagingFileDiffSchema.parse(stagingDiff).hunks).toHaveLength(1);

    const stageReq: GitStageRequest = GitStageRequestSchema.parse({
      identity,
      hunkIds: [hunk.id],
    });
    await gitStage("repo-1", "wt-1", stageReq);
    await gitUnstage("repo-1", "wt-1", {
      identity: { ...identity, area: "staged" },
    });

    const preview: GitCommitPreview = await gitCommitPreview("repo-1", "wt-1");
    const commitIdentity: GitCommitIdentity = GitCommitIdentitySchema.parse(
      preview.identity,
    );
    expect(commitIdentity.authorEmail).toBe("wire@example.invalid");
    expect(GitCommitPreviewSchema.parse(preview).stagedFiles).toHaveLength(1);

    const commitReq: GitCommitRequest = GitCommitRequestSchema.parse({
      confirmationId: preview.confirmationId,
      message: "feat: commit staged a.txt",
    });
    const result: GitCommitResult = await gitCommit(
      "repo-1",
      "wt-1",
      commitReq,
    );
    const outcomeStatus: GitCommitOutcomeStatus =
      GitCommitOutcomeStatusSchema.parse(result.outcome);
    expect(outcomeStatus).toBe("matched");
    expect(GitCommitResultSchema.parse(result).reviewReasons).toEqual([]);
    expect(calls).toHaveLength(routes.size);
  });

  test("streams raw snapshot blobs with download flag and progress updates", async () => {
    const payload = new Uint8Array([137, 80, 78, 71, 1, 2, 3, 4]);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        expect(String(input)).toBe(
          "/api/repositories/repo-1/git/snapshots/snap-9/raw?side=before&download=1",
        );
        const stream = new ReadableStream<Uint8Array>({
          start(controller) {
            controller.enqueue(payload.subarray(0, 4));
            controller.enqueue(payload.subarray(4));
            controller.close();
          },
        });
        return new Response(stream, {
          status: 200,
          headers: { "Content-Type": "image/png" },
        });
      }),
    );

    const progress: number[] = [];
    const blob = await gitSnapshotRaw(
      "repo-1",
      "snap-9",
      { side: "before", download: true },
      undefined,
      (bytes) => progress.push(bytes),
    );
    expect(progress).toEqual([4, 8]);
    expect(blob.type).toBe("image/png");
    expect(new Uint8Array(await blob.arrayBuffer())).toEqual(payload);
  });

  test("surfaces structured 409 conflict reasons and dispatches webmux:auth on 401", async () => {
    const reasons: GitConflictReason[] = [
      "readonly",
      "busy",
      "stale",
      "context_mismatch",
      "conflicted",
      "blocked",
      "commit_failed",
    ];
    for (const reason of reasons) {
      const parsedReason = GitConflictReasonSchema.parse(reason);
      const conflictBody: GitConflict = GitConflictSchema.parse({
        error: `conflict: ${parsedReason}`,
        reason: parsedReason,
      });
      vi.stubGlobal(
        "fetch",
        vi.fn(
          async () =>
            new Response(JSON.stringify(conflictBody), {
              status: 409,
              headers: { "Content-Type": "application/json" },
            }),
        ),
      );
      let caught: unknown;
      try {
        await gitStaging("repo-1", "wt-1");
      } catch (err) {
        caught = err;
      }
      expect(caught).toBeInstanceOf(APIError);
      expect(gitConflictReason(caught)).toBe(reason);
    }

    expect(
      gitConflictReason(new APIError(409, "other", "unknown_reason")),
    ).toBeUndefined();
    expect(gitConflictReason(new APIError(400, "bad"))).toBeUndefined();
    expect(gitConflictReason(new Error("network"))).toBeUndefined();

    let authFired = 0;
    window.addEventListener("webmux:auth", () => {
      authFired += 1;
    });
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ error: "unauthorized" }), {
            status: 401,
            headers: { "Content-Type": "application/json" },
          }),
      ),
    );
    await expect(gitContext("repo-1")).rejects.toMatchObject({ status: 401 });
    expect(authFired).toBe(1);
  });

  test("rejects invalid stage or commit payloads locally before sending network requests", async () => {
    const fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      gitCommit("repo-1", "wt-1", { confirmationId: "", message: "" }),
    ).rejects.toThrow();
    await expect(
      gitStage("repo-1", "wt-1", {
        identity: { area: "unstaged", path: "a.txt" },
        hunkIds: [""],
      }),
    ).rejects.toThrow();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
