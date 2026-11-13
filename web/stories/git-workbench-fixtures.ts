import { buildCommitPreviewModel } from "../src/domains/workspace/git-workbench-state";
import {
  GitBranchItemSchema,
  GitCommitDetailSchema,
  GitCommitOutcomeSchema,
  GitHistoryRowSchema,
  GitPushPreviewModelSchema,
  type GitWorkbenchViewState,
  GitWorkingChangeSchema,
  GitWorktreeContextSchema,
} from "../src/contracts/git-workbench-schemas";
import { longPath, longToken } from "./fixtures";

export type GitWorkbenchScenario =
  | "log-merge"
  | "commit-staging"
  | "commit-preview"
  | "branches-occupied"
  | "push-dialog"
  | "no-worktree"
  | "empty-unborn"
  | "index-lock-busy"
  | "commit-needs-review";

export function buildGitWorkbenchFixture(
  scenario: GitWorkbenchScenario,
  stressLongText = false,
): GitWorkbenchViewState {
  const repoName = stressLongText ? `webmux-${longToken}` : "webmux";
  const wtPath = stressLongText ? longPath : "/work/webmux";

  const context = GitWorktreeContextSchema.parse({
    repositoryId: "repo-story",
    repositoryName: repoName,
    worktreeId: scenario === "no-worktree" ? null : "wt-main",
    worktreePath: scenario === "no-worktree" ? null : wtPath,
    worktreeKind: scenario === "no-worktree" ? "bare" : "main",
    checkedOutBranch:
      scenario === "no-worktree"
        ? null
        : stressLongText
          ? `feature/${longToken}`
          : "main",
    headOid:
      scenario === "empty-unborn"
        ? null
        : "1111111111111111111111111111111111111111",
    headState:
      scenario === "no-worktree"
        ? "readonly"
        : scenario === "empty-unborn"
          ? "unborn"
          : "branch",
    ahead: 2,
    behind: 1,
    upstream: "origin/main",
    authorName: "Rain Chan",
    authorEmail: "rain@example.invalid",
    signEnabled: true,
    executableHooks: ["pre-commit", "post-commit"],
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
    subject: stressLongText
      ? `Merge branch 'feat/${longToken}' into main`
      : "Merge branch 'feat/git-workbench' into main",
    authorName: "Rain Chan",
    authorEmail: "rain@example.invalid",
    authorAt: "2026-04-12T08:00:00Z",
    committerName: "Release Bot",
    committerEmail: "bot@example.invalid",
    committerAt: "2026-04-12T08:05:00Z",
    refs: [
      { name: "main", kind: "head", worktreeId: "wt-main" },
      { name: "origin/main", kind: "remote" },
    ],
    changedPaths: [
      "internal/workspace/history.go",
      "web/src/git-graph-layout.ts",
    ],
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
        targetOid: "2222222222222222222222222222222222222222",
        kind: "indirect",
      },
    ],
    directParentsMissing: true,
    subject: "feat(git): multi-lane commit topology & indirect filter edges",
    authorName: "Rain Chan",
    authorEmail: "rain@example.invalid",
    authorAt: "2026-04-11T16:30:00Z",
    committerName: "Rain Chan",
    committerEmail: "rain@example.invalid",
    committerAt: "2026-04-11T16:30:00Z",
    refs: [
      { name: "feat/git-workbench", kind: "local", worktreeId: "wt-feat" },
    ],
    changedPaths: ["web/src/git-graph-layout.ts"],
    branches: ["main", "feat/git-workbench"],
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
    subject: "refactor(codeview): session-independent repository scope",
    authorName: "Workspace Maintainer",
    authorEmail: "maint@example.invalid",
    authorAt: "2026-04-10T10:00:00Z",
    committerName: "Workspace Maintainer",
    committerEmail: "maint@example.invalid",
    committerAt: "2026-04-10T10:00:00Z",
    refs: [{ name: "v0.9.0", kind: "tag" }],
    changedPaths: ["internal/codeview/git_context.go"],
    branches: ["main", "feat/git-workbench"],
  });

  const mergeDetail = GitCommitDetailSchema.parse({
    oid: mergeRow.oid,
    shortOid: mergeRow.shortOid,
    subject: mergeRow.subject,
    body: "Integrates multi-lane commit topology, real index hunk staging, and worktree occupancy guidance.",
    authorName: mergeRow.authorName,
    authorEmail: mergeRow.authorEmail,
    authorAt: mergeRow.authorAt,
    committerName: mergeRow.committerName,
    committerEmail: mergeRow.committerEmail,
    committerAt: mergeRow.committerAt,
    distinctCommitter: true,
    parents: [
      {
        oid: baseRow.oid,
        shortOid: baseRow.shortOid,
        subject: baseRow.subject,
      },
      {
        oid: featRow.oid,
        shortOid: featRow.shortOid,
        subject: featRow.subject,
      },
    ],
    selectedParent: baseRow.oid,
    refs: mergeRow.refs,
    filesByParent: {
      [baseRow.oid]: [
        {
          newPath: stressLongText
            ? `${longPath}/git-graph-layout.ts`
            : "web/src/git-graph-layout.ts",
          status: "A",
          additions: 151,
          deletions: 0,
          hunks: [
            {
              id: "h-log-a",
              header: "@@ -0,0 +1,4 @@",
              oldStart: 0,
              oldLines: 0,
              newStart: 1,
              newLines: 4,
              lines: [
                {
                  kind: "add",
                  newLine: 1,
                  text: "export const GIT_GRAPH_LANE_WIDTH = 14;",
                },
                {
                  kind: "add",
                  newLine: 2,
                  text: "export const GIT_GRAPH_ROW_HEIGHT = 28;",
                },
              ],
            },
          ],
        },
        {
          newPath: "internal/workspace/history.go",
          status: "M",
          additions: 94,
          deletions: 12,
          hunks: [
            {
              id: "h-log-b",
              header: "@@ -42,3 +42,5 @@",
              oldStart: 42,
              oldLines: 3,
              newStart: 42,
              newLines: 5,
              lines: [
                {
                  kind: "context",
                  oldLine: 42,
                  newLine: 42,
                  text: "func (s *Service) QueryHistory() {",
                },
                {
                  kind: "delete",
                  oldLine: 43,
                  text: "\treturn nil",
                },
                {
                  kind: "add",
                  newLine: 43,
                  text: "\t// Real %P parent topology",
                },
              ],
            },
          ],
        },
      ],
      [featRow.oid]: [
        {
          newPath: "internal/codeview/git_context.go",
          status: "M",
          additions: 18,
          deletions: 2,
          hunks: [
            {
              id: "h-log-p2",
              header: "@@ -10,2 +10,4 @@",
              oldStart: 10,
              oldLines: 2,
              newStart: 10,
              newLines: 4,
              lines: [
                {
                  kind: "add",
                  newLine: 11,
                  text: "// Compared against second parent",
                },
              ],
            },
          ],
        },
      ],
    },
  });

  const workingChanges = [
    GitWorkingChangeSchema.parse({
      id: "staged:cli_staged.go",
      path: "internal/workspace/commit_execute.go",
      group: "staged",
      status: "M",
      hunkStageAvailable: true,
      preStagedFromCli: true,
      hunks: [
        {
          id: "h-staged-cli",
          header: "@@ -18,2 +18,4 @@",
          oldStart: 18,
          oldLines: 2,
          newStart: 18,
          newLines: 4,
          lines: [
            {
              kind: "context",
              oldLine: 18,
              newLine: 18,
              text: "GIT_REFLOG_ACTION=webmux-commit",
            },
            {
              kind: "add",
              newLine: 19,
              text: "core.logAllRefUpdates=always",
            },
          ],
        },
      ],
    }),
    GitWorkingChangeSchema.parse({
      id: "unstaged:staging.go",
      path: stressLongText
        ? `${longPath}/staging.go`
        : "internal/workspace/staging.go",
      group: "unstaged",
      status: "M",
      hunkStageAvailable: true,
      hunks: [
        {
          id: "h-u-1",
          header: "@@ -10,3 +10,4 @@ func StageHunk",
          oldStart: 10,
          oldLines: 3,
          newStart: 10,
          newLines: 4,
          lines: [
            {
              kind: "context",
              oldLine: 10,
              newLine: 10,
              text: "gate := s.gate.lock(loc.Path)",
            },
            {
              kind: "add",
              newLine: 11,
              text: "defer gate.unlock()",
              lineEnding: "crlf",
            },
          ],
        },
        {
          id: "h-u-2",
          header: "@@ -48,2 +49,3 @@ func UnstageHunk",
          oldStart: 48,
          oldLines: 2,
          newStart: 49,
          newLines: 3,
          lines: [
            {
              kind: "delete",
              oldLine: 48,
              text: "oldSnapshotCheck()",
            },
            {
              kind: "add",
              newLine: 49,
              text: "verifySnapshotDigest(req.Expected)",
              lineEnding: "none",
            },
          ],
        },
      ],
    }),
    GitWorkingChangeSchema.parse({
      id: "untracked:notes.md",
      path: "docs/git-workbench-notes.md",
      group: "untracked",
      status: "?",
      hunkStageAvailable: false,
      degradeReason: "未跟踪新文件仅支持整文件暂存",
    }),
  ];

  const branches = [
    GitBranchItemSchema.parse({
      name: context.checkedOutBranch ?? "main",
      kind: "local",
      head: mergeRow.oid,
      shortHead: mergeRow.shortOid,
      subject: mergeRow.subject,
      committedAt: mergeRow.committerAt,
      upstream: "origin/main",
      ahead: 2,
      behind: 1,
    }),
    GitBranchItemSchema.parse({
      name: "feat/git-workbench",
      kind: "local",
      head: featRow.oid,
      shortHead: featRow.shortOid,
      subject: featRow.subject,
      committedAt: featRow.committerAt,
      favorite: true,
      upstream: "origin/feat/git-workbench",
      ahead: 1,
      behind: 0,
      occupiedBy: {
        worktreeId: "wt-feat",
        worktreePath: stressLongText
          ? `${longPath}/wt-feat`
          : "/work/webmux.worktrees/feat-git",
        activeSessions: 2,
      },
    }),
    GitBranchItemSchema.parse({
      name: "origin/main",
      kind: "remote",
      head: baseRow.oid,
      shortHead: baseRow.shortOid,
      subject: baseRow.subject,
      committedAt: baseRow.committerAt,
    }),
    GitBranchItemSchema.parse({
      name: "v0.9.0",
      kind: "tag",
      head: baseRow.oid,
      shortHead: baseRow.shortOid,
      subject: baseRow.subject,
      committedAt: baseRow.committerAt,
    }),
  ];

  const mode =
    scenario === "commit-staging" ||
    scenario === "commit-preview" ||
    scenario === "empty-unborn" ||
    scenario === "index-lock-busy" ||
    scenario === "commit-needs-review"
      ? "commit"
      : scenario === "branches-occupied"
        ? "branches"
        : "log";

  const base: GitWorkbenchViewState = {
    mode,
    context,
    historyState:
      scenario === "empty-unborn"
        ? "unborn"
        : scenario === "no-worktree"
          ? "ready"
          : "ready",
    historyReason:
      scenario === "empty-unborn"
        ? "当前仓库分支尚未创建首个提交（unborn HEAD），但仍可在提交面板暂存初始文件。"
        : "",
    scope: "all",
    branchFilter: null,
    searchQuery: "",
    pathFilter: "",
    commits: scenario === "empty-unborn" ? [] : [mergeRow, featRow, baseRow],
    commitDetails:
      scenario === "empty-unborn" ? {} : { [mergeDetail.oid]: mergeDetail },
    selectedCommitOid: scenario === "empty-unborn" ? null : mergeRow.oid,
    selectedParentByCommit: {},
    selectedCommitFilePath: null,
    workingChanges: scenario === "no-worktree" ? [] : workingChanges,
    activeChangeId: scenario === "no-worktree" ? null : "unstaged:staging.go",
    reviewedPaths: ["internal/workspace/commit_execute.go"],
    commitMessage: "feat(git): stage S workbench foundations",
    branches,
    commitPreview: null,
    lastCommitOutcome:
      scenario === "index-lock-busy"
        ? GitCommitOutcomeSchema.parse({
            status: "busy_locked",
            summary:
              "检测到外部进程持有 .git/index.lock，仓库正忙；WebMux 绝不强删外部锁文件，请稍后重试。",
            reasons: [".git/index.lock exists"],
          })
        : scenario === "commit-needs-review"
          ? GitCommitOutcomeSchema.parse({
              status: "needs_review",
              commitOid: "abcd12349876543210abcdef1234567890abcdef",
              expectedTreeOid: "tree-expected-a1b2",
              actualTreeOid: "tree-rewritten-c3d4",
              summary:
                "本地提交已完成，但 pre-commit 钩子格式化并改写了树对象；两阶段推送已自动中止，请核对新提交。",
              reasons: [
                "expected tree-expected-a1b2, actual tree-rewritten-c3d4",
              ],
            })
          : null,
    pushPreview:
      scenario === "push-dialog"
        ? GitPushPreviewModelSchema.parse({
            repositoryName: repoName,
            sourceBranch: context.checkedOutBranch ?? "main",
            remote: "origin",
            targetBranch: context.checkedOutBranch ?? "main",
            pushUrl: stressLongText
              ? `https://user:***@github.com/${longToken}/webmux.git`
              : "https://user:***@github.com/mzdluo123/webmux.git",
            publish: false,
            commits: [
              mergeDetail,
              GitCommitDetailSchema.parse({
                oid: featRow.oid,
                shortOid: featRow.shortOid,
                subject: featRow.subject,
                body: "Adds multi-lane commit graph layout and keyboard navigation.",
                authorName: featRow.authorName,
                authorEmail: featRow.authorEmail,
                authorAt: featRow.authorAt,
                committerName: featRow.committerName,
                committerEmail: featRow.committerEmail,
                committerAt: featRow.committerAt,
                distinctCommitter: false,
                parents: [
                  {
                    oid: baseRow.oid,
                    shortOid: baseRow.shortOid,
                    subject: baseRow.subject,
                  },
                ],
                selectedParent: baseRow.oid,
                refs: featRow.refs,
                filesByParent: {
                  [baseRow.oid]: [
                    {
                      newPath: "web/src/domains/workspace/git-graph-layout.ts",
                      status: "M",
                      additions: 48,
                      deletions: 6,
                      hunks: [
                        {
                          id: "h-push-feat",
                          header: "@@ -12,2 +12,4 @@",
                          oldStart: 12,
                          oldLines: 2,
                          newStart: 12,
                          newLines: 4,
                          lines: [
                            {
                              kind: "add",
                              newLine: 13,
                              text: "export const GIT_GRAPH_DOT_RADIUS = 4;",
                            },
                          ],
                        },
                      ],
                    },
                  ],
                },
              }),
            ],
            blockers: [],
            warnings: [
              {
                code: "remote_behind",
                message: "推送前已脱敏远端凭据，请核对待推提交差异。",
              },
            ],
          })
        : null,
    statusBanner:
      scenario === "no-worktree"
        ? "无工作树只读仓库上下文：历史与分支可正常浏览，工作区暂存与提交已禁用"
        : "就绪 · 当前工作树 /work/webmux",
  };

  if (scenario === "commit-preview") {
    base.commitPreview = buildCommitPreviewModel(base, true);
  }

  return base;
}
