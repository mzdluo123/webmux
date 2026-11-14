import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import WorkbenchTree from "../../src/components/shell/WorkbenchTree.svelte";
import GitWorkbenchTopBar from "../../src/components/workspace/GitWorkbenchTopBar.svelte";
import SessionListScopeMenu from "../../src/components/workspace/SessionListScopeMenu.svelte";
import WorkspaceActions from "../../src/components/workspace/WorkspaceActions.svelte";
import GitWorktreeGuardState from "../../src/components/workspace/GitWorktreeGuardState.svelte";
import RepositoryPanelHeaderActions from "../../src/components/workspace/RepositoryPanelHeaderActions.svelte";
import WorktreeControls from "../../src/components/workspace/WorktreeControls.svelte";
import { GitWorktreeContextSchema } from "../../src/contracts/git-workbench-schemas";
import { GitOperationSchema } from "../../src/contracts/schemas";
import CodeWorkspaceHeaderHarness from "./CodeWorkspaceHeaderHarness.svelte";
import GitWorkbenchTopBarHarness from "./GitWorkbenchTopBarHarness.svelte";
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

  render(WorkbenchTree, {
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

test("home toolbar keeps the Git workbench entry outside the ⋯ menu", async () => {
  // The button is permanent — it must render even before repositories settle
  // so its late arrival cannot shift the row.
  const onGit = vi.fn();
  render(WorkspaceActions, {
    compact: false,
    refreshing: false,
    onchange: vi.fn(),
    onrefresh: vi.fn(),
    ongit: onGit,
  });
  const button = page.getByRole("button", { name: "Git 工作台" });
  await expect.element(button).toBeVisible();
  await userEvent.click(button);
  expect(onGit).toHaveBeenCalledTimes(1);
  await cleanup();

  const repoAvail = repository({ id: "repo1", name: "webmux" });
  const props = {
    onnewworktree: vi.fn(),
    onhome: vi.fn(),
    onmanage: vi.fn(),
    onsettings: vi.fn(),
    ondetails: vi.fn(),
    onworktreedetails: vi.fn(),
    onswitch: vi.fn(),
    ontogglepin: vi.fn(),
    onremove: vi.fn(),
    onoperation: vi.fn(),
  };

  // Repository scope: the menu no longer repeats the entry the toolbar covers.
  render(SessionListScopeMenu, { ...props, repo: repoAvail, ongit: vi.fn() });
  await page.getByRole("button", { name: "仓库菜单：webmux" }).click();
  expect(page.getByRole("menuitem", { name: "Git 工作台" }).elements()).toEqual(
    [],
  );
  await userEvent.keyboard("{Escape}");
  await cleanup();

  // Picked worktree: the menu's worktree section keeps only the distinct
  // commit entry; "在 Git 工作台打开" is covered by the toolbar button.
  const dirty = worktree({
    id: "dirty",
    dirty: true,
    changes: { staged: 0, unstaged: 2, untracked: 1, conflicted: 0 },
  });
  render(SessionListScopeMenu, {
    ...props,
    repo: repoAvail,
    tree: dirty,
    ongit: vi.fn(),
  });

  await page.getByRole("button", { name: "仓库菜单：webmux" }).click();
  expect(
    page.getByRole("menuitem", { name: "在 Git 工作台打开" }).elements(),
  ).toEqual([]);
  await expect
    .element(page.getByRole("menuitem", { name: "提交改动…" }))
    .toBeVisible();
  await userEvent.keyboard("{Escape}");
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

test("session code workspace header renders '在 Git 中查看' button with accessible label", async () => {
  const onOpenGit = vi.fn();
  render(CodeWorkspaceHeaderHarness, { onOpenGit });

  const btn = page.getByRole("button", { name: "在 Git 中查看" });
  await expect.element(btn).toBeVisible();
  await expect.element(btn).toHaveAttribute("title", "在 Git 中查看");
  await userEvent.click(btn);
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
      reworkCount: 2,
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
