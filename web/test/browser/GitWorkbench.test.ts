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
    changedPaths: [
      "web/src/git-graph-layout.ts",
      "web/src/git-graph-colors.ts",
    ],
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
    refs: [
      { name: "feat/git", kind: "local", worktreeId: "wt-feat" },
      { name: "v0.9.0", kind: "tag" },
    ],
    changedPaths: ["web/src/components/GitLogPane.svelte"],
    branches: ["feat/git", "v0.9.0"],
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
        {
          newPath: "web/src/git-graph-colors.ts",
          status: "M",
          additions: 12,
          deletions: 2,
          hunks: [
            {
              id: "h-log-2",
              header: "@@ -1,1 +1,2 @@",
              oldStart: 1,
              oldLines: 1,
              newStart: 1,
              newLines: 2,
              lines: [
                {
                  kind: "context",
                  oldLine: 1,
                  newLine: 1,
                  text: "export const LANE_COLORS = [",
                },
                {
                  kind: "add",
                  newLine: 2,
                  text: '  "var(--git-lane-0)",',
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
          hunks: [
            {
              id: "h-feat-1",
              header: "@@ -0,0 +1,1 @@",
              oldStart: 0,
              oldLines: 0,
              newStart: 1,
              newLines: 1,
              lines: [
                {
                  kind: "add",
                  newLine: 1,
                  text: '<script lang="ts">',
                },
              ],
            },
          ],
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
    GitBranchItemSchema.parse({
      name: "v0.9.0",
      kind: "tag",
      head: featRow.oid,
      shortHead: featRow.shortOid,
      subject: "release 0.9.0",
      committedAt: featRow.committerAt,
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

test("desktop Log pane filters by branch/tag without mutating checkedOutBranch, switches merge parents, and navigates Pierre Diff", async () => {
  await page.viewport(1280, 900);
  render(Harness, { initialState: createFixtureState("log") });

  const filesListbox = page.getByRole("listbox", { name: "提交变更文件" });
  const diffPanel = page.getByTestId("git-log-diff-panel");

  await expect
    .element(page.getByRole("button", { name: "分支快选" }))
    .toBeVisible();
  await expect
    .element(filesListbox.getByText("web/src/git-graph-layout.ts"))
    .toBeVisible();
  await expect
    .element(diffPanel.getByText("web/src/git-graph-layout.ts"))
    .toBeVisible();
  await expect
    .element(diffPanel.getByRole("button", { name: "文件内查找" }))
    .toBeVisible();

  // Keyboard ArrowDown selects second file, Enter maximizes Diff, Escape restores docked mode.
  const firstFileOpt = filesListbox.getByRole("option", {
    name: /web\/src\/git-graph-layout\.ts/,
  });
  await firstFileOpt.click();
  await userEvent.keyboard("{ArrowDown}");
  await expect
    .element(diffPanel.getByText("web/src/git-graph-colors.ts"))
    .toBeVisible();

  await userEvent.keyboard("{Enter}");
  await expect
    .element(page.getByTestId("git-log-diff-panel"))
    .toHaveAttribute("data-diff-presentation", "maximized");
  await userEvent.keyboard("{Escape}");
  await expect
    .element(page.getByTestId("git-log-diff-panel"))
    .toHaveAttribute("data-diff-presentation", "docked");

  // "当前任务" scope is now a first-class SegmentedControl item (#547).
  const scopeGroup = page.getByRole("radiogroup", { name: "提交范围" });
  await scopeGroup.getByRole("radio", { name: "当前任务" }).click();
  await expect
    .element(scopeGroup.getByRole("radio", { name: "当前任务" }))
    .toHaveAttribute("aria-checked", "true");
  await scopeGroup.getByRole("radio", { name: "所有历史" }).click();

  // Switch merge commit parent to parent #2.
  await page.getByRole("radio", { name: /父节点 #2/ }).click();
  await expect
    .element(filesListbox.getByText("docs/design-system.md"))
    .toBeVisible();

  // Click tag in left tree: filters history ONLY, never mutates checkedOutBranch!
  const tree = page.getByRole("complementary", { name: "历史引用筛选树" });
  await tree.getByRole("button", { name: "v0.9.0" }).click();
  await expect
    .element(page.getByRole("button", { name: "分支快选" }))
    .toBeVisible();
  await expect.element(page.getByText("浏览筛选：v0.9.0")).toBeVisible();
  await expect.element(page.getByText("筛选省略中间节点")).toBeVisible();

  // Two-commit compare from commit row menu: mark base -> compare -> swap -> exit.
  await tree.getByRole("button", { name: "全部引用 (All)" }).click();
  await page.getByRole("button", { name: "提交操作 3333333" }).click();
  await page.getByRole("menuitem", { name: /设为比较起点/ }).click();
  await expect.element(page.getByText(/已设比较起点.*3333333/)).toBeVisible();

  await page.getByRole("button", { name: "提交操作 1111111" }).click();
  await page.getByRole("menuitem", { name: /与起点 \(3333333\) 比较/ }).click();
  await expect
    .element(
      page
        .getByRole("region", { name: "变更文件列表" })
        .getByText("两版本比较"),
    )
    .toBeVisible();
  await expect
    .element(filesListbox.getByText("web/src/git-graph-layout.ts"))
    .toBeVisible();

  await page.getByRole("button", { name: "交换比较方向" }).click();
  await expect
    .element(
      page
        .getByRole("region", { name: "变更文件列表" })
        .getByText("1111111 → 3333333"),
    )
    .toBeVisible();

  await page.getByRole("button", { name: "退出两版本比较" }).click();
  await expect
    .element(
      page
        .getByRole("region", { name: "变更文件列表" })
        .getByText("两版本比较"),
    )
    .not.toBeInTheDocument();
});

test("Commit pane separates reading/reviewed/staged states, stages single hunk, and shows full index preview", async () => {
  await page.viewport(1280, 900);
  render(Harness, { initialState: createFixtureState("commit") });

  // Mark reviewed without staging.
  await page.getByRole("button", { name: "标记已审阅 web/src/app.ts" }).click();
  await expect
    .element(page.getByRole("button", { name: /^标记已审阅 / }).first())
    .toBeVisible();

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

  // Explicitly assert no unimplemented V3 action buttons (e.g. Force Push / Bisect) exist.
  await expect
    .element(page.getByRole("button", { name: /Force Push|Bisect/i }))
    .not.toBeInTheDocument();
});

test("narrow mobile viewport uses 3-step list -> detail -> Unified Pierre diff navigation and 16px form inputs", async () => {
  await page.viewport(390, 844);
  render(Harness, { initialState: createFixtureState("log") });

  const searchInput = page.getByRole("textbox", { name: "搜索提交关键字" });
  await expect.element(searchInput).toBeVisible();
  const fontSize = Number.parseFloat(
    getComputedStyle(searchInput.element()).fontSize,
  );
  expect(fontSize).toBeGreaterThanOrEqual(16);

  // Step 1 -> Step 2: tap commit row -> enters detail & changed files step.
  await page
    .getByRole("option", { name: /Merge branch 'feat\/git' into main/ })
    .click();
  const backToList = page.getByRole("button", { name: "返回提交列表" });
  await expect.element(backToList).toBeVisible();

  // Step 2 -> Step 3: tap changed file -> enters mobile Unified Pierre Diff step.
  await page
    .getByRole("listbox", { name: "提交变更文件" })
    .getByRole("option", { name: /web\/src\/git-graph-layout\.ts/ })
    .click();
  const diffPanel = page.getByTestId("git-log-diff-panel");
  await expect
    .element(diffPanel)
    .toHaveAttribute("data-diff-presentation", "mobile");
  await expect
    .element(diffPanel.getByRole("button", { name: "返回提交详情" }))
    .toBeVisible();

  // Step 3 -> Step 2 -> Step 1
  await diffPanel.getByRole("button", { name: "返回提交详情" }).click();
  await expect.element(backToList).toBeVisible();
  await backToList.click();
  await expect.element(searchInput).toBeVisible();
});
