import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import WorkbenchTree from "../../src/components/shell/WorkbenchTree.svelte";
import GitWorkbenchTopBar from "../../src/components/workspace/GitWorkbenchTopBar.svelte";
import GitWorktreeGuardState from "../../src/components/workspace/GitWorktreeGuardState.svelte";
import RepositoryPanelHeaderActions from "../../src/components/workspace/RepositoryPanelHeaderActions.svelte";
import WorktreeControls from "../../src/components/workspace/WorktreeControls.svelte";
import { GitWorktreeContextSchema } from "../../src/contracts/git-workbench-schemas";
import CodeWorkspaceHeaderHarness from "./CodeWorkspaceHeaderHarness.svelte";
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

  const targetBadge = page.getByTestId("git-target-branch-badge");
  await expect.element(targetBadge).toBeVisible();
  expect(targetBadge.element().textContent).toContain("检出：main");

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
