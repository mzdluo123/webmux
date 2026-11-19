import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import GitWorkbenchTopBar from "../../src/components/workspace/GitWorkbenchTopBar.svelte";
import WorkspaceActions from "../../src/components/workspace/WorkspaceActions.svelte";
import GitWorktreeGuardState from "../../src/components/workspace/GitWorktreeGuardState.svelte";
import RepositoryPanelHeaderActions from "../../src/components/workspace/RepositoryPanelHeaderActions.svelte";
import WorktreeControls from "../../src/components/workspace/WorktreeControls.svelte";
import { GitWorktreeContextSchema } from "../../src/contracts/git-workbench-schemas";
import { GitOperationSchema } from "../../src/contracts/schemas";
import { emptyGitViewState } from "../../src/hooks/workspace/git-workbench-actions";
import { createGitWorkbenchSubControllers } from "../../src/hooks/workspace/git-workbench-subcontrollers.svelte";
import CodeWorkspaceHeaderHarness from "./CodeWorkspaceHeaderHarness.svelte";
import GitWorkbenchTopBarHarness from "./GitWorkbenchTopBarHarness.svelte";
import WorkbenchTreeHarness from "./WorkbenchTreeHarness.svelte";
import { repository, session, worktree } from "./shell-fixture";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

function createContext(
  overrides: Partial<Parameters<typeof GitWorktreeContextSchema.parse>[0]> = {},
) {
  return GitWorktreeContextSchema.parse({
    repositoryId: "repo1",
    repositoryName: "webmux",
    worktreeId: "main",
    worktreePath: "/repos/webmux",
    worktreeKind: "main",
    checkedOutBranch: "main",
    headOid: "1".repeat(40),
    headState: "branch",
    ahead: 1,
    behind: 0,
    upstream: "origin/main",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    signEnabled: false,
    executableHooks: [],
    ...overrides,
  });
}

test("repository menu and repository panel expose Git workbench entry when available", async () => {
  const onOpenGit = vi.fn();
  const repoAvail = repository({ id: "repo1", name: "webmux" });
  const repoBroken = repository({
    id: "repo2",
    name: "broken-repo",
    availability: "unavailable",
    error: "路径缺失",
    worktrees: [],
  });

  render(WorkbenchTreeHarness, {
    repositories: [repoAvail, repoBroken],
    sessions: [],
    records: [],
    home: { repo1: ["main"] },
    scope: "",
    current: "",
    canCreate: true,
    configIcons: new Map(),
    onscope: vi.fn(),
    onopen: vi.fn(),
    onnewworktree: vi.fn(),
    onmanageworktrees: vi.fn(),
    onremoveworktree: vi.fn(),
    onworktreedetails: vi.fn(),
    onswitchbranch: vi.fn(),
    onrename: vi.fn(),
    onclose: vi.fn(),
    onresume: vi.fn(),
    onremove: vi.fn(),
    onopengit: onOpenGit,
  });

  const tree = page.getByRole("tree", { name: "仓库与会话" });
  await tree.getByRole("button", { name: "仓库操作：webmux" }).click();
  const menuItem = page.getByRole("menuitem", { name: "Git 工作台" });
  await expect.element(menuItem).toBeVisible();
  await menuItem.click();
  expect(onOpenGit).toHaveBeenCalledWith("repo1");

  await tree.getByRole("button", { name: "仓库操作：broken-repo" }).click();
  expect(page.getByRole("menuitem", { name: "Git 工作台" }).elements()).toEqual(
    [],
  );
  await userEvent.keyboard("{Escape}");

  await cleanup();

  const onGitHeader = vi.fn();
  render(RepositoryPanelHeaderActions, {
    mode: "actions",
    onfetch: vi.fn(),
    ongit: onGitHeader,
  });
  const headerBtn = page.getByRole("button", { name: "Git 工作台" });
  await expect.element(headerBtn).toBeVisible();
  await userEvent.click(headerBtn);
  expect(onGitHeader).toHaveBeenCalledTimes(1);
});

test("Workbench scope navigation omits Git when unavailable", async () => {
  render(WorkspaceActions, { refreshing: false, onrefresh: vi.fn() });
  expect(page.getByRole("button", { name: "Git 工作台" }).elements()).toEqual(
    [],
  );
});

test("Workbench scope navigation keeps Git and refresh directly reachable", async () => {
  const ongit = vi.fn();
  const onrefresh = vi.fn();
  render(WorkspaceActions, { refreshing: false, onrefresh, ongit });
  await page.getByRole("button", { name: "Git 工作台" }).click();
  expect(ongit).toHaveBeenCalledTimes(1);
  await page.getByRole("button", { name: "刷新 Git 状态" }).click();
  expect(onrefresh).toHaveBeenCalledTimes(1);
});

test("worktree menu offers Git workbench entry and only shows commit action when dirty", async () => {
  const cleanWt = worktree({ id: "clean", dirty: false, hasUntracked: false });
  const onGitClean = vi.fn();
  const onCommitClean = vi.fn();

  render(WorktreeControls, {
    title: "main",
    kind: "worktree",
    ondetails: vi.fn(),
    worktree: cleanWt,
    ongit: onGitClean,
    oncommit: onCommitClean,
  });

  await userEvent.click(page.getByRole("button", { name: "工作树菜单 main" }));
  await expect
    .element(page.getByRole("menuitem", { name: "在 Git 工作台打开" }))
    .toBeVisible();
  expect(page.getByRole("menuitem", { name: "提交改动…" }).elements()).toEqual(
    [],
  );
  await page.getByRole("menuitem", { name: "在 Git 工作台打开" }).click();
  expect(onGitClean).toHaveBeenCalledTimes(1);

  await cleanup();

  const dirtyWt = worktree({
    id: "dirty",
    dirty: true,
    changes: { staged: 0, unstaged: 2, untracked: 1, conflicted: 0 },
  });
  const onCommitDirty = vi.fn();
  render(WorktreeControls, {
    title: "wt/dirty",
    kind: "worktree",
    ondetails: vi.fn(),
    worktree: dirtyWt,
    ongit: vi.fn(),
    oncommit: onCommitDirty,
  });

  await userEvent.click(
    page.getByRole("button", { name: "工作树菜单 wt/dirty" }),
  );
  const commitItem = page.getByRole("menuitem", { name: "提交改动…" });
  await expect.element(commitItem).toBeVisible();
  await commitItem.click();
  expect(onCommitDirty).toHaveBeenCalledTimes(1);
});

test("session code workspace keeps Git navigation and all file inspectors in the code menu", async () => {
  const onOpenGit = vi.fn();
  const onInspectGitFile = vi.fn();
  const inspectablePath = "src/app.ts";
  render(CodeWorkspaceHeaderHarness, {
    inspectablePath,
    onOpenGit,
    onInspectGitFile,
  });

  const menuTrigger = page.getByRole("button", {
    name: "代码菜单",
    exact: true,
  });
  await expect.element(menuTrigger).toHaveAttribute("title", "代码菜单");
  await expect
    .element(page.getByRole("button", { name: "在 Git 中查看", exact: true }))
    .not.toBeInTheDocument();

  for (const [name, tab] of [
    ["文件历史 (--follow)", "history"],
    ["逐行溯源 (Blame)", "blame"],
    ["与分支/提交比较", "compare"],
  ]) {
    await menuTrigger.click();
    await page.getByRole("menuitem", { name, exact: true }).click();
    expect(onInspectGitFile).toHaveBeenLastCalledWith(inspectablePath, tab);
  }
  expect(onInspectGitFile).toHaveBeenCalledTimes(3);

  await menuTrigger.click();
  await page
    .getByRole("menuitem", { name: "在 Git 中查看", exact: true })
    .click();
  expect(onOpenGit).toHaveBeenCalledTimes(1);
});

test("GitWorkbenchTopBar keeps fixed checked-out target distinct from history branchFilter and reports return-session state", async () => {
  const mainWt = worktree({ id: "main", branch: "main" });
  const repo = repository({ id: "repo1", name: "webmux", worktrees: [mainWt] });
  const activeSession = session({ id: "abcdefgh", title: "修复路由" });
  const onReturnSession = vi.fn();

  // `context` collides with testing-library's Svelte mount options, so pass under `props`.
  render(GitWorkbenchTopBar, {
    props: {
      repositories: [repo],
      repository: repo,
      worktreeId: "main",
      worktree: mainWt,
      context: createContext({ checkedOutBranch: "main" }),
      branchFilter: "feat/other-branch",
      returnSessionId: "abcdefgh",
      returnSession: activeSession,
      sessionsReady: true,
      onSelectRepository: vi.fn(),
      onSelectWorktree: vi.fn(),
      onAddRepository: vi.fn(),
      onNewWorktree: vi.fn(),
      onManageWorktrees: vi.fn(),
      onFetch: vi.fn(),
      onUpdate: vi.fn(),
      onPush: vi.fn(),
      onReturnSession,
      onReturnWorkbench: vi.fn(),
    },
  });

  const targetBadge = page.getByRole("button", { name: "切换操作目标工作树" });
  await expect.element(targetBadge).toBeVisible();
  expect(targetBadge.element().textContent).toContain("main");

  const filterBadge = page.getByTestId("git-filter-branch-badge");
  await expect.element(filterBadge).toBeVisible();
  expect(filterBadge.element().textContent).toContain(
    "历史筛选：feat/other-branch",
  );

  const returnBtn = page.getByRole("button", { name: "返回会话" });
  await expect.element(returnBtn).toBeVisible();
  await userEvent.click(returnBtn);
  expect(onReturnSession).toHaveBeenCalledWith("abcdefgh");

  await cleanup();

  // Closed/archived session shows non-blocking status notice while keeping top bar interactive.
  render(GitWorkbenchTopBar, {
    props: {
      repositories: [repo],
      repository: repo,
      worktreeId: "main",
      worktree: mainWt,
      context: createContext({ checkedOutBranch: "main" }),
      branchFilter: null,
      returnSessionId: "abcdefgh",
      returnSession: undefined,
      returnSessionArchived: true,
      sessionsReady: true,
      onSelectRepository: vi.fn(),
      onSelectWorktree: vi.fn(),
      onAddRepository: vi.fn(),
      onNewWorktree: vi.fn(),
      onManageWorktrees: vi.fn(),
      onFetch: vi.fn(),
      onUpdate: vi.fn(),
      onPush: vi.fn(),
      onReturnSession: vi.fn(),
      onReturnWorkbench: vi.fn(),
    },
  });

  const notice = page.getByTestId("git-return-session-notice");
  await expect.element(notice).toBeVisible();
  expect(notice.element().textContent).toContain(
    "原会话已归档，Git 工作台继续独立可用",
  );
});

test("GitWorktreeGuardState enforces explicit worktree selection on repo-only Commit and disables writes on broken worktrees", async () => {
  const mainWt = worktree({ id: "main", branch: "main", dirty: true });
  const lockedWt = worktree({
    id: "locked1",
    kind: "linked",
    branch: "wt/locked",
    path: "/repos/webmux.worktrees/locked",
    locked: true,
  });
  const repo = repository({
    id: "repo1",
    name: "webmux",
    worktrees: [mainWt, lockedWt],
  });
  const onSelectWorktree = vi.fn();

  render(GitWorktreeGuardState, {
    kind: "pick-worktree",
    repositories: [repo],
    repository: repo,
    onAddRepository: vi.fn(),
    onSelectRepository: vi.fn(),
    onSelectWorktree,
    onNewWorktree: vi.fn(),
    onManageWorktrees: vi.fn(),
  });

  await expect
    .element(page.getByTestId("git-guard-pick-worktree"))
    .toBeVisible();
  await expect
    .element(page.getByRole("heading", { name: "请选择要写入的工作树" }))
    .toBeVisible();

  // Only writable worktrees are offered; lockedWt is excluded.
  expect(page.getByText("/repos/webmux.worktrees/locked").elements()).toEqual(
    [],
  );
  await userEvent.click(page.getByRole("button", { name: /main.*有改动/ }));
  expect(onSelectWorktree).toHaveBeenCalledWith("main");

  await cleanup();

  const onManage = vi.fn();
  render(GitWorktreeGuardState, {
    kind: "broken-worktree",
    repositories: [repo],
    repository: repo,
    worktree: lockedWt,
    onAddRepository: vi.fn(),
    onSelectRepository: vi.fn(),
    onSelectWorktree: vi.fn(),
    onNewWorktree: vi.fn(),
    onManageWorktrees: onManage,
  });

  const brokenGuard = page.getByTestId("git-guard-broken-worktree");
  await expect.element(brokenGuard).toBeVisible();
  expect(brokenGuard.element().textContent).toContain("处于锁定状态");
  await userEvent.click(
    page.getByRole("button", { name: "管理或修复工作树…" }),
  );
  expect(onManage).toHaveBeenCalledTimes(1);
});

test("GitWorkbenchTopBar keeps unified shadcn button sizing, readable switcher labels, and radio dropdowns even with long failed operation summaries", async () => {
  await document.fonts.ready;
  const mainWt = worktree({ id: "main", branch: "main" });
  const featWt = worktree({
    id: "feat1",
    kind: "linked",
    branch: "feat/long-worktree-branch-name-check",
    path: "/repos/webmux.worktrees/feat1",
  });
  const repo1 = repository({
    id: "repo1",
    name: "webmux",
    worktrees: [mainWt, featWt],
  });
  const repo2 = repository({
    id: "repo2",
    name: "another-repository-with-a-very-long-display-name",
    worktrees: [worktree({ id: "main2", branch: "main" })],
  });
  const failedUpdate = GitOperationSchema.parse({
    id: "op-update-failed-123456",
    repositoryId: "repo1",
    worktreeId: "main",
    clientRequestId: "update-1",
    kind: "fast_forward",
    status: "failed",
    preview: {
      worktreePath: "/repos/webmux",
      headState: "branch",
      branch: "main",
      headOid: "1".repeat(40),
    },
    parents: [],
    fetchResults: [],
    reviewReasons: [],
    summary:
      "fatal: Not possible to fast-forward, aborting: local branch main and origin/main have diverged across multiple commits",
    sanitizedOutput: "fatal: Not possible to fast-forward, aborting.",
    startedAt: "2026-11-14T06:00:00Z",
    finishedAt: "2026-11-14T06:00:01Z",
  });

  const onSelectRepository = vi.fn();
  const onSelectWorktree = vi.fn();

  render(GitWorkbenchTopBarHarness, {
    props: {
      repositories: [repo1, repo2],
      repository: repo1,
      worktreeId: "main",
      worktree: mainWt,
      context: createContext({ checkedOutBranch: "main" }),
      operations: [failedUpdate],
      returnSession: session({ id: "sess1234", title: "提交改动" }),
      onSelectRepository,
      onSelectWorktree,
    },
  });

  for (const [w, h] of [
    [1280, 900],
    [768, 800],
    [390, 844],
    [320, 568],
  ] as const) {
    await page.viewport(w, h);

    const repoBtn = page
      .getByRole("button", { name: "切换仓库" })
      .element() as HTMLElement;
    const wtBtn = page
      .getByRole("button", { name: "切换操作目标工作树" })
      .element() as HTMLElement;
    const branchBtn = page
      .getByRole("button", { name: "分支快选" })
      .element() as HTMLElement;
    const opsBtn = page
      .getByTestId("git-operations-popover-trigger")
      .element() as HTMLElement;

    const repoRect = repoBtn.getBoundingClientRect();
    const wtRect = wtBtn.getBoundingClientRect();
    const branchRect = branchBtn.getBoundingClientRect();
    const opsRect = opsBtn.getBoundingClientRect();

    // Switchers never collapse to icon-only boxes or clip their icons outside the border.
    expect(repoRect.width).toBeGreaterThanOrEqual(72);
    expect(wtRect.width).toBeGreaterThanOrEqual(72);
    expect(repoBtn.scrollWidth).toBeLessThanOrEqual(repoBtn.clientWidth + 1);
    expect(wtBtn.scrollWidth).toBeLessThanOrEqual(wtBtn.clientWidth + 1);
    expect(branchBtn.scrollWidth).toBeLessThanOrEqual(
      branchBtn.clientWidth + 1,
    );

    // Every toolbar control shares the same height on desktop and compact touch.
    expect(Math.abs(repoRect.height - wtRect.height)).toBeLessThanOrEqual(1);
    expect(Math.abs(repoRect.height - branchRect.height)).toBeLessThanOrEqual(
      1,
    );
    expect(Math.abs(repoRect.height - opsRect.height)).toBeLessThanOrEqual(1);

    expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
      window.innerWidth,
    );
  }

  await page.viewport(1280, 900);
  await page.getByRole("button", { name: "切换仓库" }).click();
  await page
    .getByTestId("git-repository-switcher-popover")
    .getByRole("menuitemradio", { name: /another-repository/ })
    .click();
  expect(onSelectRepository).toHaveBeenCalledWith("repo2");

  await page.getByRole("button", { name: "切换操作目标工作树" }).click();
  await page
    .getByTestId("git-worktree-switcher-popover")
    .getByRole("menuitemradio", { name: /feat\/long-worktree/ })
    .click();
  expect(onSelectWorktree).toHaveBeenCalledWith("feat1");
});

test("resetAllTargets resets push dialog, cancels in-flight staging and fileInspector requests, and isolates cross-target replies", async () => {
  const pushPreviewGate = Promise.withResolvers<Response>();
  const stageGate = Promise.withResolvers<Response>();
  const historyGate = Promise.withResolvers<Response>();

  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    if (url.endsWith("/remotes")) return Response.json({ remotes: [] });
    if (url.endsWith("/push-preview")) return pushPreviewGate.promise;
    if (url.endsWith("/staging")) {
      return Response.json({
        unborn: false,
        headOid: "1".repeat(40),
        staged: [],
        unstaged: [
          {
            path: "src/app.ts",
            area: "unstaged",
            status: "M",
            hunkStageAvailable: true,
            identity: {
              area: "unstaged",
              path: "src/app.ts",
              baseOid: "1".repeat(40),
              targetSha256: "2".repeat(64),
            },
          },
        ],
        conflicted: [],
      });
    }
    if (url.endsWith("/commit-preview")) {
      return Response.json({
        confirmationId: "preflight-1",
        expiresAt: null,
        repositoryId: "repo-a",
        worktreeId: "wt-a",
        worktreePath: "/repos/a",
        headState: "branch",
        branch: "main",
        headOid: "1".repeat(40),
        headParents: [],
        headPush: { state: "unpushed", branches: [], more: 0 },
        amend: false,
        amendPushed: false,
        signoff: false,
        expectedTreeOid: "3".repeat(40),
        indexDigest: "digest-1",
        stagedFiles: [],
        identity: {
          authorName: "Alice",
          authorEmail: "alice@example.com",
          committerName: "Alice",
          committerEmail: "alice@example.com",
          signEnabled: false,
        },
        unstagedCount: 1,
        untrackedCount: 0,
        conflictedCount: 0,
        executableHooks: [],
        blockers: [],
        warnings: [],
      });
    }
    if (url.includes("/reviews")) {
      return Response.json({
        scope: "uncommitted",
        status: "ready",
        totalFiles: 0,
        reviewedFiles: 0,
        files: [],
        applied: false,
      });
    }
    if (url.endsWith("/stage")) return stageGate.promise;
    if (url.includes("/file-history")) return historyGate.promise;
    throw new Error(`Unexpected fetch: ${url}`);
  });

  let activeRepo = "repo-a";
  let activeWt: string | undefined = "wt-a";
  let view = emptyGitViewState("repo-a");

  const sub = createGitWorkbenchSubControllers({
    getRepoId: () => activeRepo,
    getEffectiveWorktreeId: () => activeWt,
    getWorktreeTargetId: () => activeWt,
    getDefaultBranch: () => "main",
    getRepository: () => undefined,
    getSessions: () => [],
    getView: () => view,
    patchView: (patch) => {
      view = { ...view, ...patch };
    },
    persist: () => {},
    updateUrl: () => {},
    onSelectChange: () => {},
    onReloadTarget: async () => {},
  });

  try {
    await sub.staging.refresh("repo-a", "wt-a");
    const changeId = view.workingChanges[0]?.id;
    expect(changeId).toBeDefined();

    sub.push.openDialog("main");
    expect(sub.push.open).toBe(true);

    const stagePromise = sub.staging.stageOrUnstage("stage", changeId!);
    expect(sub.staging.stagingBusy).toBe(true);

    sub.fileInspector.openInspector({
      path: "src/app.ts",
      tab: "history",
      compareBaseOid: "1".repeat(40),
    });
    expect(sub.fileInspector.open).toBe(true);
    expect(sub.fileInspector.historyLoading).toBe(true);
    expect(sub.fileInspector.compareBaseOid).toBe("1".repeat(40));

    // Switch target to repo-b/wt-b while push, stage, and file-history are in flight.
    activeRepo = "repo-b";
    activeWt = "wt-b";
    view = emptyGitViewState("repo-b");
    sub.resetAllTargets("repo-b", "wt-b");

    expect(sub.push.open).toBe(false);
    expect(sub.push.preview).toBeNull();
    expect(sub.staging.stagingBusy).toBe(false);
    expect(sub.fileInspector.open).toBe(false);
    expect(sub.fileInspector.historyLoading).toBe(false);
    expect(sub.fileInspector.compareBaseOid).toBe("");

    // Resolve stale in-flight responses for repo-a and verify repo-b stays clean.
    pushPreviewGate.resolve(
      Response.json({
        confirmationId: "late-push",
        expiresAt: null,
        branch: "main",
        commit: "1".repeat(40),
        remote: "origin",
        pushUrl: "https://example.invalid/a.git",
        target: "main",
        publish: false,
        commits: { total: 0, items: [] },
        blockers: [],
        warnings: [],
      }),
    );
    stageGate.resolve(new Response(null, { status: 204 }));
    historyGate.resolve(
      Response.json({
        status: "ready",
        path: "src/app.ts",
        commits: [
          {
            oid: "9".repeat(40),
            shortOid: "9999999",
            parents: [],
            subject: "late",
            authorName: "Alice",
            authorEmail: "alice@example.com",
            authorAt: "2026-11-19T00:00:00Z",
            path: "src/app.ts",
            status: "M",
          },
        ],
        nextCursor: "",
      }),
    );

    await stagePromise;
    expect(sub.push.preview).toBeNull();
    expect(sub.fileInspector.historyRows).toEqual([]);
    expect(view.statusBanner).toBe("");
  } finally {
    sub.disposeAll();
    vi.restoreAllMocks();
  }
});
