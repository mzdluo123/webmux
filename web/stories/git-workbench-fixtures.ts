import { buildCommitPreviewModel } from "../src/domains/workspace/git-workbench-state";
import { GitOperationSchema } from "../src/contracts/git-api-schemas";
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
import { applyGitStoryV2Scenario } from "./git-workbench-v2-fixtures";

export type GitWorkbenchScenario =
  | "log-merge"
  | "commit-staging"
  | "commit-preview"
  | "task-baseline-review"
  | "branches-occupied"
  | "push-dialog"
  | "no-worktree"
  | "empty-unborn"
  | "index-lock-busy"
  | "commit-needs-review"
  | "operation-uncertain"
  | "commit-draft-conflict"
  | "rework-session-handoff"
  | "conflict-three-way-text"
  | "conflict-binary-variant"
  | "divergence-rebase-preview"
  | "abort-operation-dialog";

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
    worktreeKind:
      scenario === "no-worktree"
        ? "bare"
        : scenario === "task-baseline-review"
          ? "linked"
          : "main",
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
    scenario === "task-baseline-review" ||
    scenario === "empty-unborn" ||
    scenario === "index-lock-busy" ||
    scenario === "commit-needs-review" ||
    scenario === "operation-uncertain" ||
    scenario === "commit-draft-conflict" ||
    scenario === "rework-session-handoff"
      ? "commit"
      : scenario === "branches-occupied"
        ? "branches"
        : "log";

  const sampleOperations = [
    GitOperationSchema.parse({
      id: "8f31c0d9e4a27b56129043ab89cd01ef",
      repositoryId: "repo-story",
      worktreeId: "wt-main",
      clientRequestId: "commit-demo-1",
      kind: scenario === "operation-uncertain" ? "push" : "commit",
      status:
        scenario === "operation-uncertain"
          ? "uncertain"
          : scenario === "commit-needs-review"
            ? "needs_review"
            : "succeeded",
      confirmationId: "cf9102ab34cd56ef78901234567890ab",
      startedAt: "2026-04-12T08:04:10Z",
      finishedAt: "2026-04-12T08:04:12Z",
      summary:
        scenario === "operation-uncertain"
          ? "服务重启时推送结果无法仅凭本地引用确认，请先获取远程并核对远端分支状态"
          : scenario === "commit-needs-review"
            ? "提交已完成，但 pre-commit 钩子格式化并改写了树对象"
            : "已创建提交 11111111：Merge branch 'feat/git-workbench' into main",
      sanitizedOutput:
        scenario === "operation-uncertain"
          ? "remote: https://user:***@github.com/mzdluo123/webmux.git\nconnection reset before remote ref ack"
          : "[main 1111111] Merge branch 'feat/git-workbench' into main\n 2 files changed, 64 insertions(+), 8 deletions(-)",
      fetchResults: [],
      preview: {
        worktreePath: wtPath,
        headState: "branch",
        branch: "main",
        headOid: "2222222222222222222222222222222222222222",
        expectedTreeOid: "7316cdbc05a7070b8a86588a9bbaaf90fddca4e4",
      },
      actualOid: "1111111111111111111111111111111111111111",
      actualTreeOid: "7316cdbc05a7070b8a86588a9bbaaf90fddca4e4",
      parents: ["2222222222222222222222222222222222222222"],
      subject: "Merge branch 'feat/git-workbench' into main",
      reviewReasons:
        scenario === "commit-needs-review"
          ? [
              "实际生成的提交树摘要与预览不一致（pre-commit 钩子修改了暂存文件）",
            ]
          : [],
    }),
    GitOperationSchema.parse({
      id: "4a19b2e8c7f031256908da34bc78ef90",
      repositoryId: "repo-story",
      worktreeId: "wt-main",
      clientRequestId: "fetch-demo-0",
      kind: "fetch",
      status: "succeeded",
      startedAt: "2026-04-12T08:01:00Z",
      finishedAt: "2026-04-12T08:01:02Z",
      summary: "已获取全部远程（1 个）",
      fetchResults: [{ remote: "origin", status: "ok", message: "" }],
      preview: {},
      parents: [],
      reviewReasons: [],
    }),
  ];

  const sampleTaskBaseline =
    scenario === "no-worktree"
      ? null
      : {
          baselineId: "bl-wt-main",
          repositoryId: "repo-story",
          worktreeId: "wt-main",
          worktreePath: wtPath,
          source: "worktree_create" as const,
          unborn: scenario === "empty-unborn",
          ...(scenario === "empty-unborn" ? {} : { baseOid: baseRow.oid }),
          capturedAt: "2026-04-10T10:05:00Z",
          cleanAtStart: true,
          preExistingChanges: {
            staged: 0,
            unstaged: 0,
            untracked: 0,
            conflicted: 0,
          },
        };
  const sampleTaskFiles = mergeDetail.filesByParent[baseRow.oid] ?? [];
  const effectiveWorkingChanges =
    scenario === "no-worktree" || scenario === "task-baseline-review"
      ? []
      : workingChanges;

  const base: GitWorkbenchViewState = {
    mode,
    context,
    lastFetchedAt: "2026-04-12T08:01:02Z",
    taskBaseline: sampleTaskBaseline,
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
    reviewScope: scenario === "task-baseline-review" ? "task" : "uncommitted",
    taskReview: sampleTaskBaseline
      ? {
          status: "ready",
          reason: "",
          baseline: sampleTaskBaseline,
          headOid: context.headOid ?? "",
          headUnborn: context.headState === "unborn",
          commitCount: scenario === "empty-unborn" ? 0 : 2,
          uncommittedCount: effectiveWorkingChanges.length,
          files: scenario === "empty-unborn" ? [] : sampleTaskFiles,
          selectedPath: sampleTaskFiles[0]?.newPath ?? null,
          searchQuery: "",
          nextCursor: "",
          loadingMore: false,
          mutating: false,
        }
      : undefined,
    branchFilter: null,
    searchQuery: "",
    pathFilter: "",
    commits: scenario === "empty-unborn" ? [] : [mergeRow, featRow, baseRow],
    commitDetails:
      scenario === "empty-unborn" ? {} : { [mergeDetail.oid]: mergeDetail },
    selectedCommitOid: scenario === "empty-unborn" ? null : mergeRow.oid,
    selectedParentByCommit: {},
    selectedCommitFilePath: null,
    workingChanges: effectiveWorkingChanges,
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
              operationId: "8f31c0d9e4a27b56129043ab89cd01ef",
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
          : scenario === "operation-uncertain"
            ? GitCommitOutcomeSchema.parse({
                operationId: "8f31c0d9e4a27b56129043ab89cd01ef",
                status: "uncertain",
                summary:
                  "服务重启时推送结果无法仅凭本地引用确认，请先获取远程并核对远端分支状态。",
                reasons: [],
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
    operations: scenario === "no-worktree" ? [] : sampleOperations,
    statusBanner:
      scenario === "no-worktree"
        ? "无工作树只读仓库上下文：历史与分支可正常浏览，工作区暂存与提交已禁用"
        : scenario === "operation-uncertain"
          ? "操作结果待核实（uncertain）：已禁用盲目一键重试，请先刷新核实"
          : "已创建提交 11111111：Merge branch 'feat/git-workbench' into main",
  };

  if (scenario === "commit-preview") {
    base.commitPreview = buildCommitPreviewModel(base, true);
  }

  return applyGitStoryV2Scenario(base, scenario, stressLongText);
}
