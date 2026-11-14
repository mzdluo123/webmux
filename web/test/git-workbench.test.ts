import { describe, expect, test } from "vite-plus/test";
import {
  computeGitGraphLayout,
  GIT_GRAPH_LANE_WIDTH,
  GIT_GRAPH_LEFT_PAD,
  GIT_GRAPH_ROW_HEIGHT,
  laneTone,
  laneToX,
} from "../src/domains/workspace/git-graph-layout";
import {
  applyStageHunk,
  applyUnstageHunk,
  buildCommitPreviewModel,
  filterLogCommits,
  groupWorkingChanges,
  resolveCommitParentFiles,
  selectLogBranchFilter,
  toggleReviewedPath,
} from "../src/domains/workspace/git-workbench-state";
import {
  GitBranchItemSchema,
  GitCommitDetailSchema,
  GitCommitOutcomeSchema,
  GitCommitPreviewModelSchema,
  GitHistoryRowSchema,
  GitHistoryScopeSchema,
  GitPushPreviewModelSchema,
  GitWorkbenchModeSchema,
  type GitWorkbenchViewState,
  GitWorkingChangeSchema,
  GitWorktreeContextSchema,
} from "../src/contracts/git-workbench-schemas";

function makeBaseState(): GitWorkbenchViewState {
  const context = GitWorktreeContextSchema.parse({
    repositoryId: "repo-1",
    repositoryName: "webmux",
    worktreeId: "wt-main",
    worktreePath: "/work/webmux",
    worktreeKind: "main",
    checkedOutBranch: "main",
    headOid: "1111111111111111111111111111111111111111",
    headState: "branch",
    ahead: 2,
    behind: 1,
    upstream: "origin/main",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    signEnabled: true,
    executableHooks: ["pre-commit"],
  });

  const mergeRow = GitHistoryRowSchema.parse({
    oid: "1111111111111111111111111111111111111111",
    shortOid: "1111111",
    parents: [
      "2222222222222222222222222222222222222222",
      "3333333333333333333333333333333333333333",
    ],
    lane: 0,
    activeLanes: [0, 1],
    edges: [
      {
        fromLane: 0,
        toLane: 0,
        targetOid: "2222222222222222222222222222222222222222",
        kind: "direct",
      },
      {
        fromLane: 0,
        toLane: 1,
        targetOid: "3333333333333333333333333333333333333333",
        kind: "direct",
      },
    ],
    directParentsMissing: false,
    subject: "Merge branch 'feat/git' into main",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    authorAt: "2026-04-12T08:00:00Z",
    committerName: "Bob",
    committerEmail: "bob@example.com",
    committerAt: "2026-04-12T08:05:00Z",
    refs: [{ name: "main", kind: "head", worktreeId: "wt-main" }],
    changedPaths: ["internal/workspace/history.go"],
    branches: ["main"],
  });

  const featRow = GitHistoryRowSchema.parse({
    oid: "3333333333333333333333333333333333333333",
    shortOid: "3333333",
    parents: ["4444444444444444444444444444444444444444"],
    lane: 1,
    activeLanes: [0, 1],
    edges: [
      {
        fromLane: 1,
        toLane: 0,
        targetOid: "9999999999999999999999999999999999999999",
        kind: "indirect",
      },
    ],
    directParentsMissing: true,
    subject: "feat: implement git graph lanes",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    authorAt: "2026-04-11T09:00:00Z",
    committerName: "Alice",
    committerEmail: "alice@example.com",
    committerAt: "2026-04-11T09:00:00Z",
    refs: [{ name: "feat/git", kind: "local", worktreeId: "wt-feat" }],
    changedPaths: ["web/src/git-graph-layout.ts"],
    branches: ["main", "feat/git"],
  });

  const baseRow = GitHistoryRowSchema.parse({
    oid: "2222222222222222222222222222222222222222",
    shortOid: "2222222",
    parents: ["9999999999999999999999999999999999999999"],
    lane: 0,
    activeLanes: [0],
    edges: [
      {
        fromLane: 0,
        toLane: 0,
        targetOid: "9999999999999999999999999999999999999999",
        kind: "boundary",
      },
    ],
    directParentsMissing: false,
    subject: "chore: prepare base commit",
    authorName: "Bob",
    authorEmail: "bob@example.com",
    authorAt: "2026-04-10T07:00:00Z",
    committerName: "Bob",
    committerEmail: "bob@example.com",
    committerAt: "2026-04-10T07:00:00Z",
    refs: [],
    changedPaths: ["README.md"],
    branches: ["main"],
  });

  const mergeDetail = GitCommitDetailSchema.parse({
    oid: mergeRow.oid,
    shortOid: mergeRow.shortOid,
    subject: mergeRow.subject,
    body: "Brings multi-lane graph into main.",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    authorAt: "2026-04-12T08:00:00Z",
    committerName: "Bob",
    committerEmail: "bob@example.com",
    committerAt: "2026-04-12T08:05:00Z",
    distinctCommitter: true,
    parents: [
      {
        oid: "2222222222222222222222222222222222222222",
        shortOid: "2222222",
        subject: "chore: prepare base commit",
      },
      {
        oid: "3333333333333333333333333333333333333333",
        shortOid: "3333333",
        subject: "feat: implement git graph lanes",
      },
    ],
    selectedParent: "2222222222222222222222222222222222222222",
    refs: mergeRow.refs,
    filesByParent: {
      "2222222222222222222222222222222222222222": [
        {
          newPath: "web/src/git-graph-layout.ts",
          status: "A",
          additions: 120,
          deletions: 0,
        },
      ],
      "3333333333333333333333333333333333333333": [
        {
          newPath: "README.md",
          status: "M",
          additions: 4,
          deletions: 1,
        },
      ],
    },
  });

  const changes = [
    GitWorkingChangeSchema.parse({
      id: "staged:cli-pre.go",
      path: "internal/workspace/cli_pre.go",
      group: "staged",
      status: "M",
      hunkStageAvailable: true,
      preStagedFromCli: true,
      hunks: [
        {
          id: "h-cli",
          header: "@@ -1,2 +1,3 @@",
          oldStart: 1,
          oldLines: 2,
          newStart: 1,
          newLines: 3,
          lines: [
            {
              kind: "context",
              oldLine: 1,
              newLine: 1,
              text: "package workspace",
            },
            { kind: "add", newLine: 2, text: "// staged via terminal" },
          ],
        },
      ],
    }),
    GitWorkingChangeSchema.parse({
      id: "unstaged:app.ts",
      path: "web/src/app.ts",
      group: "unstaged",
      status: "M",
      hunkStageAvailable: true,
      hunks: [
        {
          id: "h-1",
          header: "@@ -1,2 +1,3 @@",
          oldStart: 1,
          oldLines: 2,
          newStart: 1,
          newLines: 3,
          lines: [{ kind: "add", newLine: 1, text: "const a = 1;" }],
        },
        {
          id: "h-2",
          header: "@@ -10,2 +11,3 @@",
          oldStart: 10,
          oldLines: 2,
          newStart: 11,
          newLines: 3,
          lines: [{ kind: "add", newLine: 11, text: "const b = 2;" }],
        },
      ],
    }),
    GitWorkingChangeSchema.parse({
      id: "untracked:notes.md",
      path: "notes.md",
      group: "untracked",
      status: "?",
      hunkStageAvailable: false,
    }),
  ];

  const branch = GitBranchItemSchema.parse({
    name: "feat/git",
    kind: "local",
    head: featRow.oid,
    shortHead: featRow.shortOid,
    subject: featRow.subject,
    committedAt: featRow.committerAt,
    favorite: true,
    upstream: "origin/feat/git",
    ahead: 1,
    behind: 0,
    occupiedBy: {
      worktreeId: "wt-feat",
      worktreePath: "/work/webmux-feat",
      activeSessions: 2,
    },
  });

  return {
    mode: GitWorkbenchModeSchema.parse("log"),
    context,
    historyState: "ready",
    historyReason: "",
    scope: GitHistoryScopeSchema.parse("all"),
    branchFilter: null,
    searchQuery: "",
    pathFilter: "",
    commits: [mergeRow, featRow, baseRow],
    commitDetails: { [mergeDetail.oid]: mergeDetail },
    selectedCommitOid: mergeRow.oid,
    selectedParentByCommit: {},
    selectedCommitFilePath: null,
    workingChanges: changes,
    activeChangeId: "unstaged:app.ts",
    reviewedPaths: [],
    commitMessage: "feat(git): stage S workbench",
    branches: [branch],
    commitPreview: null,
    lastCommitOutcome: null,
    pushPreview: null,
    statusBanner: "",
  };
}

describe("git workbench state & contracts", () => {
  test("clicking a branch in Log only filters history and never mutates checkedOutBranch", () => {
    const initial = makeBaseState();
    const filteredState = selectLogBranchFilter(initial, "feat/git");

    expect(filteredState.branchFilter).toBe("feat/git");
    expect(filteredState.context.checkedOutBranch).toBe("main");
    expect(filteredState.selectedCommitOid).toBe(
      "3333333333333333333333333333333333333333",
    );

    const visible = filterLogCommits(
      filteredState.commits,
      filteredState.branchFilter,
      filteredState.searchQuery,
      filteredState.pathFilter,
    );
    expect(visible.map((r) => r.shortOid)).toEqual(["3333333"]);
  });

  test("switching merge commit parent updates changed files for that parent", () => {
    const state = makeBaseState();
    const detail =
      state.commitDetails["1111111111111111111111111111111111111111"]!;

    const p1 = resolveCommitParentFiles(detail);
    expect(p1.selectedParent).toBe("2222222222222222222222222222222222222222");
    expect(p1.files.map((f) => f.newPath)).toEqual([
      "web/src/git-graph-layout.ts",
    ]);

    const p2 = resolveCommitParentFiles(
      detail,
      "3333333333333333333333333333333333333333",
    );
    expect(p2.selectedParent).toBe("3333333333333333333333333333333333333333");
    expect(p2.files.map((f) => f.newPath)).toEqual(["README.md"]);
  });

  test("partial hunk staging keeps reading, reviewed and staged states independent and builds full index preview", () => {
    const state = makeBaseState();

    // Mark reviewed without staging.
    const reviewed = toggleReviewedPath(state.reviewedPaths, "web/src/app.ts");
    expect(reviewed).toEqual(["web/src/app.ts"]);
    expect(groupWorkingChanges(state.workingChanges).staged).toHaveLength(1);

    // Stage only hunk h-1 of web/src/app.ts: file must exist in BOTH Staged and Unstaged!
    const afterStageOne = applyStageHunk(
      state.workingChanges,
      "unstaged:app.ts",
      "h-1",
    );
    const grouped = groupWorkingChanges(afterStageOne);
    expect(grouped.staged.map((c) => c.path)).toEqual([
      "internal/workspace/cli_pre.go",
      "web/src/app.ts",
    ]);
    expect(grouped.unstaged.map((c) => c.path)).toEqual(["web/src/app.ts"]);
    expect(
      grouped.staged.find((c) => c.path === "web/src/app.ts")?.hunks,
    ).toHaveLength(1);
    expect(
      grouped.unstaged.find((c) => c.path === "web/src/app.ts")?.hunks,
    ).toHaveLength(1);

    // Full index commit preview includes both CLI pre-staged file and newly staged hunk.
    const preview = GitCommitPreviewModelSchema.parse(
      buildCommitPreviewModel(
        { ...state, workingChanges: afterStageOne, reviewedPaths: reviewed },
        true,
      ),
    );
    expect(preview.followUpPush).toBe(true);
    expect(preview.stagedFiles).toHaveLength(2);
    expect(preview.unstagedCount).toBe(1);
    expect(preview.untrackedCount).toBe(1);
    expect(preview.blockers).toEqual([]);
    expect(preview.warnings.map((w) => w.code)).toContain("cli_prestaged");

    // Unstage hunk h-1 returns it to unstaged without touching reviewedPaths.
    const afterUnstage = applyUnstageHunk(
      afterStageOne,
      "staged:web/src/app.ts",
      "h-1",
    );
    const regrouped = groupWorkingChanges(afterUnstage);
    expect(regrouped.staged.map((c) => c.path)).toEqual([
      "internal/workspace/cli_pre.go",
    ]);
    expect(
      regrouped.unstaged.find((c) => c.path === "web/src/app.ts")?.hunks,
    ).toHaveLength(2);
    expect(toggleReviewedPath(reviewed, "web/src/app.ts")).toEqual([]);
  });

  test("validates commit outcome and push preview schemas", () => {
    const outcome = GitCommitOutcomeSchema.parse({
      status: "needs_review",
      commitOid: "abcd1234",
      expectedTreeOid: "tree-1",
      actualTreeOid: "tree-2",
      summary: "pre-commit 钩子改写了提交树，已暂停自动推送",
      reasons: ["expected tree-1, got tree-2"],
    });
    expect(outcome.status).toBe("needs_review");

    const state = makeBaseState();
    const pushModel = GitPushPreviewModelSchema.parse({
      repositoryName: "webmux",
      sourceBranch: "main",
      remote: "origin",
      targetBranch: "main",
      pushUrl: "https://github.com/example/webmux.git",
      publish: false,
      commits: [
        state.commitDetails["1111111111111111111111111111111111111111"],
      ],
      blockers: [],
      warnings: [],
    });
    expect(pushModel.commits).toHaveLength(1);
  });
});

describe("git graph layout geometry", () => {
  test("computes multi-lane coordinates, dashed indirect edges, and boundary stubs", () => {
    const state = makeBaseState();
    const layout = computeGitGraphLayout(state.commits);

    expect(layout).toHaveLength(3);
    expect(laneToX(0)).toBe(GIT_GRAPH_LEFT_PAD);
    expect(laneToX(1)).toBe(GIT_GRAPH_LEFT_PAD + GIT_GRAPH_LANE_WIDTH);
    expect(laneTone(7)).toBe(1);

    // Row 0: merge commit with 2 direct parent segments and no floating fork rail.
    const row0 = layout[0]!;
    expect(row0.isMerge).toBe(true);
    expect(row0.height).toBe(GIT_GRAPH_ROW_HEIGHT);
    expect(row0.rails).toEqual([]);
    expect(row0.segments).toHaveLength(2);
    expect(row0.segments.every((s) => !s.dashed && !s.isBoundary)).toBe(true);

    // Row 1: filtered row with omitted intermediate parent -> dashed indirect segment,
    // incoming rail on lane 1, and full-height pass-through rail on lane 0.
    const row1 = layout[1]!;
    expect(row1.directParentsMissing).toBe(true);
    expect(row1.rails).toEqual([
      {
        lane: 1,
        x: laneToX(1),
        y1: 0,
        y2: GIT_GRAPH_ROW_HEIGHT / 2,
        tone: laneTone(1),
      },
      {
        lane: 0,
        x: laneToX(0),
        y1: 0,
        y2: GIT_GRAPH_ROW_HEIGHT,
        tone: laneTone(0),
      },
    ]);
    expect(row1.segments[0]!.dashed).toBe(true);
    expect(row1.segments[0]!.kind).toBe("indirect");

    // Row 2: page boundary commit -> incoming rail on lane 0 + boundary continuation segment.
    const row2 = layout[2]!;
    expect(row2.rails).toEqual([
      {
        lane: 0,
        x: laneToX(0),
        y1: 0,
        y2: GIT_GRAPH_ROW_HEIGHT / 2,
        tone: laneTone(0),
      },
    ]);
    expect(row2.segments[0]!.isBoundary).toBe(true);
    expect(row2.segments[0]!.kind).toBe("boundary");
  });

  test("connects boundary edges across appended pages, draws root incoming rails, and does not leak closed lanes", () => {
    const state = makeBaseState();
    const baseTemplate = state.commits[2]!;
    const rootOid = "9999999999999999999999999999999999999999";
    const newTipOid = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";

    // Simulate appending page 2 containing rootOid (with empty activeLanes as emitted by backend)
    // plus an unrelated branch tip reusing lane 1 after featRow closed lane 1 -> 0.
    const newBranchTip = {
      ...baseTemplate,
      oid: newTipOid,
      shortOid: "aaaaaaa",
      parents: [rootOid],
      lane: 1,
      activeLanes: [0, 1],
      edges: [
        {
          fromLane: 1,
          toLane: 0,
          targetOid: rootOid,
          kind: "direct" as const,
        },
      ],
    };
    const rootCommit = {
      ...baseTemplate,
      oid: rootOid,
      shortOid: "9999999",
      parents: [],
      lane: 0,
      activeLanes: [],
      edges: [],
    };

    const layout = computeGitGraphLayout([
      state.commits[0]!,
      state.commits[1]!,
      newBranchTip,
      state.commits[2]!,
      rootCommit,
    ]);

    // newBranchTip reuses lane 1 after featRow closed lane 1 -> 0: no incoming rail on lane 1.
    const tipLayout = layout[2]!;
    expect(tipLayout.rails.some((r) => r.lane === 1)).toBe(false);
    expect(tipLayout.rails).toEqual([
      {
        lane: 0,
        x: laneToX(0),
        y1: 0,
        y2: GIT_GRAPH_ROW_HEIGHT,
        tone: laneTone(0),
      },
    ]);

    // baseRow's former boundary edge to rootOid becomes solid direct once rootCommit is loaded.
    const baseLayout = layout[3]!;
    expect(baseLayout.segments[0]!.isBoundary).toBe(false);
    expect(baseLayout.segments[0]!.kind).toBe("direct");

    // rootCommit receives an incoming rail into its dot despite activeLanes: [].
    const rootLayout = layout[4]!;
    expect(rootLayout.rails).toEqual([
      {
        lane: 0,
        x: laneToX(0),
        y1: 0,
        y2: GIT_GRAPH_ROW_HEIGHT / 2,
        tone: laneTone(0),
      },
    ]);
    expect(rootLayout.segments).toEqual([]);
  });
});
