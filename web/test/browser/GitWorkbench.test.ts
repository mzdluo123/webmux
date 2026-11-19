import { tick } from "svelte";
import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import {
  GitBranchItemSchema,
  GitCommitDetailSchema,
  GitHistoryRowSchema,
  type GitWorkbenchViewState,
  GitWorkingChangeSchema,
  GitWorktreeContextSchema,
} from "../../src/contracts/git-workbench-schemas";
import GitLogListPane from "../../src/components/workspace/GitLogListPane.svelte";
import GitUncommittedChangesPane from "../../src/components/workspace/GitUncommittedChangesPane.svelte";
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

  await expect
    .element(
      diffPanel.getByRole("radiogroup", {
        name: "历史差异布局模式",
      }),
    )
    .not.toBeInTheDocument();
  await expect
    .element(diffPanel.getByRole("button", { name: "变更前全文" }))
    .not.toBeInTheDocument();
  await expect
    .element(diffPanel.getByRole("button", { name: "变更后全文" }))
    .not.toBeInTheDocument();
  await expect
    .element(diffPanel.getByRole("button", { name: "放大差异到主工作区" }))
    .not.toBeInTheDocument();
  await expect
    .element(page.getByRole("button", { name: "放大查看当前文件差异" }))
    .not.toBeInTheDocument();
  await expect.element(diffPanel.getByText("修改前")).toBeVisible();
  await expect.element(diffPanel.getByText("修改后")).toBeVisible();

  // Keyboard ArrowDown selects second file, Enter expands collapsed dock without leaving docked mode.
  await diffPanel.getByRole("button", { name: "折叠底部差异面板" }).click();
  const firstFileOpt = filesListbox.getByRole("option", {
    name: /web\/src\/git-graph-layout\.ts/,
  });
  await firstFileOpt.click();
  await userEvent.keyboard("{ArrowDown}");
  await userEvent.keyboard("{Enter}");
  await expect
    .element(diffPanel.getByText("web/src/git-graph-colors.ts"))
    .toBeVisible();
  await expect
    .element(diffPanel.getByRole("button", { name: "文件内查找" }))
    .toBeVisible();
  await expect
    .element(diffPanel)
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
  await tree.getByRole("button", { name: "全部引用" }).click();
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

  // Mark reviewed without staging or changing the inline control's weight.
  const review = page.getByRole("button", {
    name: "标记已审阅 web/src/app.ts",
  });
  const before = review.element().getBoundingClientRect();
  await review.click();
  await expect.element(review).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("tab", { name: "提交", exact: true }).click();
  const after = review.element().getBoundingClientRect();
  expect(after.width).toBe(before.width);
  expect(after.height).toBe(before.height);
  const reviewedStyle = getComputedStyle(review.element());
  expect(reviewedStyle.borderTopWidth).toBe("0px");
  expect(reviewedStyle.backgroundColor).toBe("rgba(0, 0, 0, 0)");
  await expect
    .element(page.getByRole("button", { name: /^标记已审阅 / }).first())
    .toBeVisible();

  // Stage a single hunk from web/src/app.ts, verifying split diff columns on desktop and no unified switcher.
  const stagingDiff = page.getByTestId("git-staging-diff-panel");
  await expect
    .element(stagingDiff.getByRole("radiogroup", { name: "差异视图模式" }))
    .not.toBeInTheDocument();
  await expect.element(stagingDiff.getByText("修改前")).toBeVisible();
  await expect.element(stagingDiff.getByText("修改后")).toBeVisible();
  const hunkButtons = page.getByRole("button", { name: "暂存此块" });
  await hunkButtons.first().click();

  // Open Commit & Push two-phase preview: must list BOTH CLI pre-staged file AND newly staged file!
  await page.getByRole("button", { name: "提交并推送…" }).click();
  await expect.element(page.getByText(/确认提交并进入推送预览/)).toBeVisible();
  await expect.element(page.getByText("终端预暂存")).toBeVisible();

  // Confirm step 1 (commit matched) -> transitions into Push preview dialog!
  await page.getByRole("button", { name: "确认提交并继续预览推送" }).click();
  await expect
    .element(
      page
        .getByRole("dialog", { name: "推送确认" })
        .getByText("origin/main", { exact: true }),
    )
    .toBeVisible();
});

test("Branches pane opens the occupied worktree instead of a conflicting checkout", async () => {
  await page.viewport(1280, 900);
  const onOpenWorktree = vi.fn();
  render(Harness, {
    initialState: createFixtureState("branches"),
    onOpenWorktree,
  });

  await page.getByRole("option", { name: /^feat\/git/ }).click();
  await expect.element(page.getByText(/当前活跃会话：3 个/)).toBeVisible();

  await page.getByRole("button", { name: "打开所在工作树" }).click();
  expect(onOpenWorktree).toHaveBeenCalledWith("wt-feat", "/work/webmux-feat");
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

test("commit graph SVGs touch edge-to-edge without vertical gaps across wrapped and compact rows", async () => {
  for (const [width, height] of [
    [1280, 900],
    [390, 844],
  ] as const) {
    await page.viewport(width, height);
    render(Harness, { initialState: createFixtureState("log") });

    await expect
      .element(page.getByRole("listbox", { name: "提交记录" }))
      .toBeVisible();

    const svgs = [
      ...document.querySelectorAll<SVGSVGElement>("svg[data-graph-oid]"),
    ];
    expect(svgs.length).toBeGreaterThanOrEqual(2);

    for (let i = 0; i < svgs.length; i++) {
      const svgRect = svgs[i]!.getBoundingClientRect();
      const rowRect =
        svgs[i]!.closest("[data-commit-row]")!.getBoundingClientRect();
      expect(Math.abs(svgRect.top - rowRect.top)).toBeLessThan(0.5);
      expect(Math.abs(svgRect.bottom - rowRect.bottom)).toBeLessThan(0.5);

      const circle = svgs[i]!.querySelector("circle")!;
      const circleRect = circle.getBoundingClientRect();
      expect(Math.abs(circleRect.width - circleRect.height)).toBeLessThan(0.5);

      if (i + 1 < svgs.length) {
        const nextRect = svgs[i + 1]!.getBoundingClientRect();
        expect(Math.abs(nextRect.top - svgRect.bottom)).toBeLessThan(0.5);
      }
    }

    await cleanup();
  }
});

test("GitLogListPane retains reading scroll position across cursor append and status notices and guards aria-disabled pagination", async () => {
  await page.viewport(390, 844);
  const makeRow = (index: number) => {
    const hex = (index + 1).toString(16).padStart(40, "0");
    const parentHex = (index + 2).toString(16).padStart(40, "0");
    return GitHistoryRowSchema.parse({
      oid: hex,
      shortOid: hex.slice(0, 7),
      parents: [parentHex],
      lane: 0,
      activeLanes: [0],
      edges: [
        {
          fromLane: 0,
          toLane: 0,
          targetOid: parentHex,
          kind: "direct",
        },
      ],
      directParentsMissing: false,
      subject: `commit row ${index + 1}`,
      authorName: "Alice",
      authorEmail: "alice@example.com",
      authorAt: "2026-04-12T08:00:00Z",
      committerName: "Alice",
      committerEmail: "alice@example.com",
      committerAt: "2026-04-12T08:00:00Z",
      refs: [],
      changedPaths: [],
      branches: ["main"],
    });
  };

  const firstPage = Array.from({ length: 20 }, (_, i) => makeRow(i));
  const secondPage = Array.from({ length: 30 }, (_, i) => makeRow(i));
  let savedScrollTop = 0;
  const onListScrollChange = vi.fn((top: number) => {
    savedScrollTop = top;
  });
  const onLoadMoreHistory = vi.fn();

  const baseProps = {
    desktop: false,
    historyState: "ready" as const,
    nextCursor: "cur-2",
    historyLoadingMore: false,
    visibleCommits: firstPage,
    selectedCommitOid: firstPage[0]!.oid,
    listScrollTop: 0,
    scope: "all" as const,
    branchFilter: null,
    searchQuery: "",
    authorFilter: "",
    sinceFilter: "",
    untilFilter: "",
    pathFilter: "",
    refsChanged: false,
    selectionNotice: null,
    compareBaseMarkOid: null,
    onListScrollChange,
    onLoadMoreHistory,
    onSelectCommit: () => {},
    onScopeChange: () => {},
    onSearchChange: () => {},
    onPathFilterChange: () => {},
  };

  const view = await render(GitLogListPane, baseProps);
  view.container.style.height = "320px";
  view.container.style.width = "390px";

  const listbox = page.getByRole("listbox", { name: "提交记录" });
  await expect.element(listbox).toBeVisible();
  const listboxEl = listbox.element() as HTMLElement;
  await expect
    .poll(() => listboxEl.scrollHeight > listboxEl.clientHeight)
    .toBe(true);

  listboxEl.scrollTop = 180;
  listboxEl.dispatchEvent(new Event("scroll"));
  expect(savedScrollTop).toBe(180);

  const loadMoreBtn = page.getByRole("button", { name: /加载更多/ });
  const loadMoreEl = loadMoreBtn.element() as HTMLButtonElement;
  loadMoreEl.focus({ preventScroll: true });
  expect(document.activeElement).toBe(loadMoreEl);
  expect(loadMoreEl.getAttribute("tabindex")).toBeNull();

  loadMoreEl.click();
  expect(onLoadMoreHistory).toHaveBeenCalledTimes(1);

  await view.rerender({
    ...baseProps,
    historyLoadingMore: true,
    listScrollTop: savedScrollTop,
  });

  const loadingBtn = page.getByRole("button", { name: /正在加载/ });
  const loadingEl = loadingBtn.element() as HTMLButtonElement;
  expect(document.activeElement).toBe(loadingEl);
  await expect.element(loadingBtn).toHaveAttribute("aria-disabled", "true");
  await expect.element(loadingBtn).toHaveAttribute("tabindex", "-1");

  // Guard blocks duplicate click and keyboard activation while loading.
  loadingEl.click();
  await userEvent.keyboard("{Enter}");
  expect(onLoadMoreHistory).toHaveBeenCalledTimes(1);

  // Appending cursor page preserves reading position and transfers focus when exhausted.
  await view.rerender({
    ...baseProps,
    visibleCommits: secondPage,
    nextCursor: "",
    historyLoadingMore: false,
    listScrollTop: savedScrollTop,
  });

  await expect.poll(() => Math.round(listboxEl.scrollTop)).toBe(180);
  expect(document.activeElement).toBe(listboxEl);

  // Status notice preserves saved scroll position and restores it on return.
  onListScrollChange.mockClear();
  await view.rerender({
    ...baseProps,
    historyState: "no_task_baseline",
    scope: "task",
    visibleCommits: [],
    nextCursor: "",
    listScrollTop: savedScrollTop,
  });
  listboxEl.scrollTop = 0;
  listboxEl.dispatchEvent(new Event("scroll"));
  expect(onListScrollChange).not.toHaveBeenCalled();

  await view.rerender({
    ...baseProps,
    historyState: "ready",
    scope: "all",
    visibleCommits: secondPage,
    nextCursor: "",
    listScrollTop: savedScrollTop,
  });
  await expect.poll(() => Math.round(listboxEl.scrollTop)).toBe(180);
});

test("GitUncommittedChangesPane folds all four change groups uniformly, marks closing rows inert, unmounts settled rows, and skips animation on refresh", async () => {
  await page.viewport(1280, 900);
  const fixture = createFixtureState("commit");
  const allGroupChanges = [
    GitWorkingChangeSchema.parse({
      id: "conflicted:conflict.ts",
      path: "web/src/conflict.ts",
      group: "conflicted",
      status: "U",
      hunkStageAvailable: false,
      hunks: [],
    }),
    ...fixture.workingChanges,
    GitWorkingChangeSchema.parse({
      id: "untracked:notes.md",
      path: "docs/notes.md",
      group: "untracked",
      status: "?",
      hunkStageAvailable: false,
      hunks: [],
    }),
  ];
  const onSelectChange = vi.fn();
  const baseProps = {
    context: fixture.context,
    changes: allGroupChanges,
    activeChange: allGroupChanges[1]!,
    reviewedPaths: [],
    commitMessage: "",
    onSelectChange,
    onToggleReviewed: () => {},
    onStageChange: () => {},
    onUnstageChange: () => {},
    onCommitMessageChange: () => {},
    onOpenCommitPreview: () => {},
    onOpenPush: () => {},
    onCreateBranchFromHead: () => {},
    onResolveDraftKeepLocal: () => {},
    onResolveDraftAdoptRemote: () => {},
    onResolveDraftMergeBoth: () => {},
    onToggleCommitAmend: () => {},
    onResolveAmendDraft: () => {},
    onCommitAuthorOverrideChange: () => {},
    onToggleCommitSignoff: () => {},
  };

  const view = await render(GitUncommittedChangesPane, { props: baseProps });
  view.container.style.height = "640px";
  view.container.style.width = "420px";

  const groups = ["conflicted", "staged", "unstaged", "untracked"] as const;
  for (const group of groups) {
    const section = document.querySelector<HTMLElement>(
      `[data-change-section="${group}"]`,
    )!;
    const trigger = section.querySelector<HTMLButtonElement>(
      '[data-slot="collapsible-trigger"]',
    )!;
    const panel = trigger.closest('[data-slot="collapsible"]')!
      .lastElementChild as HTMLDivElement;

    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(
      section.querySelectorAll("[data-working-change-row]").length,
    ).toBeGreaterThan(0);

    // Collapsing marks panel inert immediately, animates height, and unmounts rows once settled.
    trigger.click();
    await tick();
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(panel.inert).toBe(true);
    await expect.poll(() => panel.getAnimations().length).toBeGreaterThan(0);
    await expect
      .poll(() => section.querySelectorAll("[data-working-change-row]").length)
      .toBe(0);

    // Expanding mounts rows and animates height back open across every group.
    trigger.click();
    await tick();
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(panel.inert).toBe(false);
    await expect.poll(() => panel.getAnimations().length).toBeGreaterThan(0);
    await expect.poll(() => panel.getAnimations().length).toBe(0);
    expect(
      section.querySelectorAll("[data-working-change-row]").length,
    ).toBeGreaterThan(0);
  }

  // While the first group is mid-collapse, Home jumps immediately to the next open group's row.
  const conflictedSec = document.querySelector<HTMLElement>(
    '[data-change-section="conflicted"]',
  )!;
  conflictedSec
    .querySelector<HTMLButtonElement>('[data-slot="collapsible-trigger"]')!
    .click();
  await tick();
  const unstagedRow = document.querySelector<HTMLButtonElement>(
    '[data-change-section="unstaged"] [data-working-change-row]',
  )!;
  unstagedRow.focus();
  await userEvent.keyboard("{Home}");
  expect(onSelectChange).toHaveBeenLastCalledWith("staged:cli.go");
  await expect
    .poll(
      () => conflictedSec.querySelectorAll("[data-working-change-row]").length,
    )
    .toBe(0);

  // Rapid mid-collapse click reverses back open without dropping the rows.
  const stagedSec = document.querySelector<HTMLElement>(
    '[data-change-section="staged"]',
  )!;
  const stagedTrigger = stagedSec.querySelector<HTMLButtonElement>(
    '[data-slot="collapsible-trigger"]',
  )!;
  const stagedPanel = stagedTrigger.closest('[data-slot="collapsible"]')!
    .lastElementChild as HTMLDivElement;
  stagedTrigger.click();
  await expect
    .poll(() => stagedPanel.getAnimations().length)
    .toBeGreaterThan(0);
  stagedTrigger.click();
  await tick();
  expect(stagedTrigger.getAttribute("aria-expanded")).toBe("true");
  expect(stagedPanel.inert).toBe(false);
  await expect.poll(() => stagedPanel.getAnimations().length).toBe(0);
  expect(
    stagedSec.querySelectorAll("[data-working-change-row]").length,
  ).toBeGreaterThan(0);

  // Background refresh updates file list without starting disclosure animations.
  const refreshedChanges = [
    ...allGroupChanges,
    GitWorkingChangeSchema.parse({
      id: "untracked:second.md",
      path: "docs/second.md",
      group: "untracked",
      status: "?",
      hunkStageAvailable: false,
      hunks: [],
    }),
  ];
  await view.rerender({ ...baseProps, changes: refreshedChanges });
  await expect
    .element(page.getByRole("option", { name: /docs\/second\.md/ }))
    .toBeVisible();
  for (const el of document.querySelectorAll<HTMLElement>(
    '[data-slot="collapsible"] > div:last-child',
  )) {
    expect(el.getAnimations()).toHaveLength(0);
  }
});
