import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import {
  GitBranchItemSchema,
  GitCommitDetailSchema,
  GitHistoryRowSchema,
  type GitWorkbenchViewState,
  GitWorkingChangeSchema,
  GitWorktreeContextSchema,
} from "../../src/contracts/git-workbench-schemas";
import Harness from "./GitWorkbenchHarness.svelte";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

function createFixtureState(
  mode: GitWorkbenchViewState["mode"] = "log",
): GitWorkbenchViewState {
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
    behind: 0,
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
    changedPaths: ["web/src/git-graph-layout.ts"],
    branches: ["main"],
  });

  const featRow = GitHistoryRowSchema.parse({
    oid: "3333333333333333333333333333333333333333",
    shortOid: "3333333",
    parents: ["2222222222222222222222222222222222222222"],
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
    subject: "feat(git): multi-lane commit graph",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    authorAt: "2026-04-11T09:00:00Z",
    committerName: "Alice",
    committerEmail: "alice@example.com",
    committerAt: "2026-04-11T09:00:00Z",
    refs: [{ name: "feat/git", kind: "local", worktreeId: "wt-feat" }],
    changedPaths: ["web/src/components/GitLogPane.svelte"],
    branches: ["feat/git"],
  });

  const mergeDetail = GitCommitDetailSchema.parse({
    oid: mergeRow.oid,
    shortOid: mergeRow.shortOid,
    subject: mergeRow.subject,
    body: "Brings Git workbench layout.",
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
        subject: "chore: base",
      },
      {
        oid: "3333333333333333333333333333333333333333",
        shortOid: "3333333",
        subject: "feat(git): multi-lane commit graph",
      },
    ],
    selectedParent: "2222222222222222222222222222222222222222",
    refs: mergeRow.refs,
    filesByParent: {
      "2222222222222222222222222222222222222222": [
        {
          newPath: "web/src/git-graph-layout.ts",
          status: "A",
          additions: 150,
          deletions: 0,
          hunks: [
            {
              id: "h-log-1",
              header: "@@ -0,0 +1,2 @@",
              oldStart: 0,
              oldLines: 0,
              newStart: 1,
              newLines: 2,
              lines: [
                {
                  kind: "add",
                  newLine: 1,
                  text: "export const GIT_GRAPH_LANE_WIDTH = 14;",
                },
              ],
            },
          ],
        },
      ],
      "3333333333333333333333333333333333333333": [
        {
          newPath: "docs/design-system.md",
          status: "M",
          additions: 6,
          deletions: 1,
          hunks: [],
        },
      ],
    },
  });

  const featDetail = GitCommitDetailSchema.parse({
    oid: featRow.oid,
    shortOid: featRow.shortOid,
    subject: featRow.subject,
    authorName: "Alice",
    authorEmail: "alice@example.com",
    authorAt: "2026-04-11T09:00:00Z",
    committerName: "Alice",
    committerEmail: "alice@example.com",
    committerAt: "2026-04-11T09:00:00Z",
    distinctCommitter: false,
    parents: [
      {
        oid: "2222222222222222222222222222222222222222",
        shortOid: "2222222",
        subject: "chore: base",
      },
    ],
    selectedParent: "2222222222222222222222222222222222222222",
    refs: featRow.refs,
    filesByParent: {
      "2222222222222222222222222222222222222222": [
        {
          newPath: "web/src/components/GitLogPane.svelte",
          status: "A",
          additions: 220,
          deletions: 0,
          hunks: [],
        },
      ],
    },
  });

  const changes = [
    GitWorkingChangeSchema.parse({
      id: "staged:cli.go",
      path: "internal/workspace/cli.go",
      group: "staged",
      status: "M",
      hunkStageAvailable: true,
      preStagedFromCli: true,
      hunks: [
        {
          id: "h-cli",
          header: "@@ -1,1 +1,2 @@",
          oldStart: 1,
          oldLines: 1,
          newStart: 1,
          newLines: 2,
          lines: [{ kind: "add", newLine: 2, text: "// pre-staged in cli" }],
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
          lines: [{ kind: "add", newLine: 1, text: "const first = true;" }],
        },
        {
          id: "h-2",
          header: "@@ -10,2 +11,3 @@",
          oldStart: 10,
          oldLines: 2,
          newStart: 11,
          newLines: 3,
          lines: [{ kind: "add", newLine: 11, text: "const second = true;" }],
        },
      ],
    }),
  ];

  const branches = [
    GitBranchItemSchema.parse({
      name: "main",
      kind: "local",
      head: mergeRow.oid,
      shortHead: mergeRow.shortOid,
      subject: mergeRow.subject,
      committedAt: mergeRow.committerAt,
      upstream: "origin/main",
      ahead: 2,
      behind: 0,
    }),
    GitBranchItemSchema.parse({
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
        activeSessions: 3,
      },
    }),
  ];

  return {
    mode,
    context,
    historyState: "ready",
    historyReason: "",
    scope: "all",
    branchFilter: null,
    searchQuery: "",
    pathFilter: "",
    commits: [mergeRow, featRow],
    commitDetails: {
      [mergeDetail.oid]: mergeDetail,
      [featDetail.oid]: featDetail,
    },
    selectedCommitOid: mergeRow.oid,
    selectedParentByCommit: {},
    selectedCommitFilePath: null,
    workingChanges: changes,
    activeChangeId: "unstaged:app.ts",
    reviewedPaths: [],
    commitMessage: "feat(git): stage S foundations",
    branches,
    commitPreview: null,
    lastCommitOutcome: null,
    pushPreview: null,
    statusBanner: "",
  };
}

test("desktop Log pane filters by branch without mutating checkedOutBranch and switches merge parents", async () => {
  await page.viewport(1280, 900);
  render(Harness, { initialState: createFixtureState("log") });

  await expect.element(page.getByText("当前检出：main")).toBeVisible();
  await expect
    .element(page.getByText("web/src/git-graph-layout.ts"))
    .toBeVisible();

  // Switch merge commit parent to parent #2.
  await page.getByRole("radio", { name: /父节点 #2/ }).click();
  await expect.element(page.getByText("docs/design-system.md")).toBeVisible();

  // Click branch in left tree: filters history ONLY, never mutates checkedOutBranch!
  const tree = page.getByRole("complementary", { name: "历史引用筛选树" });
  await tree.getByRole("button", { name: "feat/git" }).click();
  await expect.element(page.getByText("当前检出：main")).toBeVisible();
  await expect.element(page.getByText("浏览筛选：feat/git")).toBeVisible();
  await expect.element(page.getByText("筛选省略中间节点")).toBeVisible();
});

test("Commit pane separates reading/reviewed/staged states, stages single hunk, and shows full index preview", async () => {
  await page.viewport(1280, 900);
  render(Harness, { initialState: createFixtureState("commit") });

  // Mark reviewed without staging.
  await page.getByRole("button", { name: "标记已审阅 web/src/app.ts" }).click();
  await expect.element(page.getByText("已审阅", { exact: true })).toBeVisible();

  // Stage a single hunk from web/src/app.ts.
  const hunkButtons = page.getByRole("button", { name: "暂存此块" });
  await hunkButtons.first().click();

  // Open Commit & Push two-phase preview: must list BOTH CLI pre-staged file AND newly staged file!
  await page.getByRole("button", { name: "提交并推送…" }).click();
  await expect.element(page.getByText(/确认提交并进入推送预览/)).toBeVisible();
  await expect.element(page.getByText("终端预暂存")).toBeVisible();

  // Confirm step 1 (commit matched) -> transitions into Push preview dialog!
  await page.getByRole("button", { name: "确认提交并继续预览推送" }).click();
  await expect
    .element(page.getByText(/推送确认 · main → origin\/main/))
    .toBeVisible();
});

test("Branches pane guides occupied worktree instead of conflicting checkout and renders no fake V2/V3 buttons", async () => {
  await page.viewport(1280, 900);
  const onOpenWorktree = vi.fn();
  render(Harness, {
    initialState: createFixtureState("branches"),
    onOpenWorktree,
  });

  await page
    .getByRole("button", { name: /feat\/git/ })
    .first()
    .click();
  await expect.element(page.getByText(/当前活跃会话：3 个/)).toBeVisible();

  await page.getByRole("button", { name: "打开所在工作树" }).click();
  expect(onOpenWorktree).toHaveBeenCalledWith("wt-feat", "/work/webmux-feat");

  // Explicitly assert no fake V2/V3 action buttons exist.
  await expect
    .element(page.getByRole("button", { name: /Stash|Rebase|Cherry-pick/i }))
    .not.toBeInTheDocument();
});

test("narrow mobile viewport uses step navigation and 16px form inputs", async () => {
  await page.viewport(390, 844);
  render(Harness, { initialState: createFixtureState("log") });

  const searchInput = page.getByRole("textbox", { name: "搜索提交关键字" });
  await expect.element(searchInput).toBeVisible();
  const fontSize = Number.parseFloat(
    getComputedStyle(searchInput.element()).fontSize,
  );
  expect(fontSize).toBeGreaterThanOrEqual(16);

  // Tap commit row -> enters detail step with back button.
  await page
    .getByRole("option", { name: /Merge branch 'feat\/git' into main/ })
    .click();
  const backButton = page.getByRole("button", { name: "返回提交列表" });
  await expect.element(backButton).toBeVisible();
  await backButton.click();
  await expect.element(searchInput).toBeVisible();
});
