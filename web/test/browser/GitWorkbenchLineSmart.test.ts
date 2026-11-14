import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import type { SwitchSource } from "../../src/components/workspace/SwitchBranch.svelte";
import SwitchBranchDialog from "../../src/components/workspace/SwitchBranchDialog.svelte";
import type {
  Branches,
  GitSmartCheckoutPreview,
  GitSmartCheckoutResult,
  SwitchPreview,
  SwitchTarget,
} from "../../src/contracts/schemas";
import { buildGitWorkbenchFixture } from "../../stories/git-workbench-fixtures";
import Harness from "./GitWorkbenchHarness.svelte";
import { base, branch, chat, repository, worktree } from "./shell-fixture";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

test("Sub-hunk line selector stages and unstages selected lines while preserving remaining lines", async () => {
  await page.viewport(1280, 900);
  render(Harness, {
    initialState: buildGitWorkbenchFixture("commit-staging"),
  });

  // Select unstaged `internal/workspace/staging.go` and toggle sub-hunk line picker.
  await page
    .getByTestId("git-change-row-unstaged:staging.go")
    .getByRole("option")
    .click();
  await page.getByTestId("git-toggle-line-selector").click();
  await expect
    .element(page.getByTestId("git-staging-line-selector"))
    .toBeVisible();

  // Check CRLF and no-newline fidelity badges are rendered on hunk #1 and #2.
  await expect.element(page.getByText("CRLF")).toBeVisible();
  await expect.element(page.getByText("无末尾换行")).toBeVisible();

  // Select 1 delete line in hunk #2 (`oldSnapshotCheck()`), leaving `+verifySnapshotDigest` and hunk #1 unstaged.
  await page
    .getByRole("checkbox", { name: /选择变更块 #2 第 48 行删除/ })
    .click();
  await expect.element(page.getByText("已选 1 行")).toBeVisible();

  await page.getByTestId("git-line-stage-btn").click();

  // `internal/workspace/staging.go` now exists in BOTH staged (1 line) and unstaged (remaining 2 lines)!
  const stagedSplitRow = page.getByTestId(
    "git-change-row-staged:internal/workspace/staging.go",
  );
  await expect.element(stagedSplitRow).toBeVisible();
  await expect
    .element(page.getByTestId("git-change-row-unstaged:staging.go"))
    .toBeVisible();

  // Select the newly staged entry and unstage that 1 line via sub-hunk line selector.
  await stagedSplitRow.getByRole("option").click();
  await expect
    .element(page.getByTestId("git-staging-line-selector"))
    .toBeVisible();
  await page
    .getByRole("checkbox", { name: /选择变更块 #1 第 48 行删除/ })
    .click();
  await page.getByTestId("git-line-unstage-btn").click();

  expect(stagedSplitRow.elements()).toEqual([]);
});

test("Discard Changes strictly protects staged index, confirms overwrite/delete with active session guard, and records recovery OID", async () => {
  await page.viewport(1280, 900);
  const activeAgent = chat({
    id: "agentv3a",
    title: "Claude Code · 编辑工作区",
    status: "running",
  });
  render(Harness, {
    initialState: buildGitWorkbenchFixture("commit-staging"),
    simulatedActiveSessions: [activeAgent],
  });

  // Staged row NEVER renders a Discard button (only Unstage)!
  const stagedRow = page.getByTestId("git-change-row-staged:cli_staged.go");
  await expect.element(stagedRow).toBeVisible();
  expect(
    stagedRow.getByRole("button", { name: /丢弃更改/ }).elements(),
  ).toEqual([]);

  // Multi-select unstaged tracked file + untracked file for batch Discard.
  await page
    .getByRole("checkbox", {
      name: "选择未暂存条目 internal/workspace/staging.go",
    })
    .click();
  await page
    .getByRole("checkbox", {
      name: "选择未暂存条目 docs/git-workbench-notes.md",
    })
    .click();

  await expect.element(page.getByTestId("git-discard-batch-bar")).toBeVisible();
  await page.getByTestId("git-batch-discard-btn").click();

  const discardDialog = page.getByRole("dialog", {
    name: /确认丢弃未暂存更改/,
  });
  await expect.element(discardDialog).toBeVisible();
  await expect
    .element(page.getByTestId("git-discard-tracked-list"))
    .toMatchTextContent(/internal\/workspace\/staging\.go/);
  await expect
    .element(page.getByTestId("git-discard-untracked-list"))
    .toMatchTextContent(/docs\/git-workbench-notes\.md/);

  // Confirm is disabled until active session acknowledgement is checked.
  const confirmBtn = discardDialog.getByRole("button", {
    name: "确认丢弃更改",
  });
  await expect.element(confirmBtn).toBeDisabled();
  await discardDialog
    .getByRole("checkbox", { name: /我已知晓当前工作树有活跃会话/ })
    .click();
  await expect.element(confirmBtn).toBeEnabled();
  await confirmBtn.click();

  // Unstaged & untracked entries are gone, while staged `commit_execute.go` remains untouched!
  await expect.element(stagedRow).toBeVisible();
  expect(
    page.getByTestId("git-change-row-unstaged:staging.go").elements(),
  ).toEqual([]);
  expect(
    page.getByTestId("git-change-row-untracked:notes.md").elements(),
  ).toEqual([]);

  // Recovery OID and recovery command are surfaced in the bottom Git operations status bar!
  await page.getByTestId("git-operations-popover-trigger").click();
  await expect
    .element(page.getByTestId("git-operation-recovery-cmd"))
    .toMatchTextContent(/git restore --source=9a8b7c6d.* --worktree/);
});

const head = "a".repeat(40);
const tip = "b".repeat(40);
const dirtyMain = worktree({
  dirty: true,
  hasUntracked: true,
  changes: { staged: 1, unstaged: 1, untracked: 1, conflicted: 0 },
});
const switchRepo = repository({ worktrees: [dirtyMain] });
const switchBranches: Branches = {
  branches: [
    branch("main", head, [dirtyMain.path]),
    branch("feat/target", tip),
  ],
  bases: [base("refs/remotes/origin/main", head)],
  tags: [],
  defaultBase: head,
  defaultBaseSubject: "",
  defaultBranch: "main",
  defaultParent: "/repos/webmux.worktrees",
};

function buildDirtyPlainPreview(
  target: SwitchTarget,
  sessions: SwitchPreview["sessions"] = [],
): SwitchPreview {
  return {
    confirmationId: "",
    expiresAt: null,
    worktree: dirtyMain,
    target: {
      kind: target.kind,
      branch: "feat/target",
      commit: tip,
      subject: "feat: target branch",
      createsBranch: false,
      upstream: null,
    },
    sessions,
    blockers: [{ code: "changes", message: "有未提交的修改，请先提交或贮藏" }],
    warnings:
      sessions.length > 0
        ? [{ code: "sessions", message: "有 1 个运行中的会话" }]
        : [],
  };
}

function buildSmartPreview(
  includeUntracked: boolean,
  sessions: SwitchPreview["sessions"] = [],
): GitSmartCheckoutPreview {
  return {
    confirmationId: "smart-confirm-1",
    expiresAt: "2026-11-13T12:05:00Z",
    repositoryId: switchRepo.id,
    worktreeId: dirtyMain.id,
    worktree: dirtyMain,
    target: {
      kind: "local",
      branch: "feat/target",
      commit: tip,
      subject: "feat: target branch",
      createsBranch: false,
      upstream: null,
    },
    includeUntracked,
    stagedCount: 1,
    unstagedCount: 1,
    untrackedCount: 1,
    dirtyPaths: ["src/auth.ts", "notes.md"],
    hasActiveAgent: sessions.length > 0,
    sessions,
    blockers: [],
    warnings: [],
  };
}

test("Smart Checkout enforces active CLI agent session disclosure + acknowledgement, and guides conflict stash recovery", async () => {
  await page.viewport(1280, 900);
  const agent = chat({
    id: "agent009",
    title: "Claude Code · 实现登录模块",
    status: "running",
  });

  const conflictResult: GitSmartCheckoutResult = {
    operationId: "op-smart-conflict",
    outcome: "conflict_needs_resolution",
    phase: "unstashing",
    stashOid: "9999aaaabbbbccccddddeeeeffff000011112222",
    stashRestored: false,
    stashDropped: false,
    stashPreserved: true,
    recoveryCommand: "git stash apply 9999aaaabbbbccccddddeeeeffff000011112222",
    summary:
      "已切换至 feat/target，自动恢复修改时遇到 1 个冲突文件；受控贮藏 9999aaa 已完整保留。",
    inProgress: {
      active: true,
      kind: "unstash",
      worktreeId: dirtyMain.id,
      worktreePath: dirtyMain.path,
      headOid: tip,
      headSubject: "feat: target branch",
      currentBranch: "feat/target",
      detached: false,
      sourceLabel: "Smart Checkout 贮藏 9999aaa",
      conflictedCount: 1,
      conflictedFiles: [
        {
          path: "src/auth.ts",
          code: "UU",
          kind: "both_modified",
          binary: false,
        },
      ],
      canContinue: false,
      canAbort: false,
      continueBlockers: [],
    },
    worktree: { ...dirtyMain, branch: "feat/target" },
  };

  const source: SwitchSource = {
    branches: vi.fn(async () => switchBranches),
    commits: vi.fn(async () => ({ commits: [] })),
    preview: vi.fn(async (_tree, target) =>
      buildDirtyPlainPreview(target, [agent]),
    ),
    switch: vi.fn(async () => dirtyMain),
    smartPreview: vi.fn(async (_tree, req) =>
      buildSmartPreview(req.includeUntracked ?? true, [agent]),
    ),
    smartCheckout: vi.fn(async () => conflictResult),
  };
  const onopenconflict = vi.fn();
  const onclose = vi.fn();

  render(SwitchBranchDialog, {
    repository: switchRepo,
    worktree: dirtyMain,
    initialBranch: "feat/target",
    source,
    onchange: vi.fn(),
    onnewworktree: vi.fn(),
    ongoto: vi.fn(),
    onopenconflict,
    onclose,
  });

  const dialog = page.getByRole("dialog", { name: "切换分支：main" });
  // Green "改为在新工作树打开" remains prominent while Smart Checkout body is collapsed behind disclosure!
  await expect
    .element(dialog.getByRole("button", { name: "改为在新工作树打开" }))
    .toBeVisible();
  expect(dialog.getByTestId("smart-checkout-body").elements()).toEqual([]);

  // Expand explicit Smart Checkout disclosure; submit remains disabled until session ack checkbox is checked.
  await dialog.getByTestId("smart-checkout-session-disclosure").click();
  await expect.element(dialog.getByTestId("smart-checkout-body")).toBeVisible();
  const smartSubmit = dialog.getByTestId("smart-checkout-submit-btn");
  await expect.element(smartSubmit).toBeDisabled();

  await dialog.getByTestId("smart-checkout-ack-sessions").click();
  await expect.element(smartSubmit).toBeEnabled();
  await smartSubmit.click();

  expect(source.smartCheckout).toHaveBeenCalledWith(dirtyMain, {
    confirmationId: "smart-confirm-1",
    acknowledgeSessions: true,
    clientRequestId: expect.stringMatching(/^smart_checkout-/),
  });

  // Dialog stays open on conflict, showing preserved stash OID + recovery command + 3-way conflict resolver CTA!
  expect(onclose).not.toHaveBeenCalled();
  await expect
    .element(dialog.getByTestId("smart-checkout-conflict-card"))
    .toBeVisible();
  await expect
    .element(dialog.getByText(/受控贮藏已保留 · 9999aaa/))
    .toBeVisible();
  await expect
    .element(dialog.getByTestId("smart-checkout-recovery-cmd"))
    .toMatchTextContent(/git stash apply 9999aaa/);

  await dialog.getByTestId("smart-checkout-open-conflict-btn").click();
  expect(onopenconflict).toHaveBeenCalledWith(
    expect.objectContaining({ branch: "feat/target" }),
    "src/auth.ts",
  );
  expect(onclose).toHaveBeenCalledOnce();
});
