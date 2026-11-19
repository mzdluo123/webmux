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
  const line = page.getByRole("checkbox", {
    name: /选择变更块 #2 第 48 行删除/,
  });
  const toolbar = page.getByTestId("git-line-selector-action-bar").element();
  const height = toolbar.getBoundingClientRect().height;
  await line.click();
  await expect.element(page.getByText("已选 1 行")).toBeVisible();
  expect(toolbar.getBoundingClientRect().height).toBe(height);

  const clear = page.getByTestId("git-line-clear-btn");
  await expect.element(clear).not.toHaveAttribute("aria-disabled", "true");
  expect(clear.element().getAttribute("tabindex")).toBeNull();
  await clear.click();
  await expect.element(line).not.toBeChecked();
  await expect.element(clear).toHaveFocus();
  await expect.element(clear).toHaveAttribute("aria-disabled", "true");
  await expect.element(clear).toHaveAttribute("tabindex", "-1");
  expect(toolbar.getBoundingClientRect().height).toBe(height);

  await line.click();
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

  const resolver = dialog.getByTestId("smart-checkout-open-conflict-btn");
  await expect.element(resolver).toHaveFocus();
  await resolver.click();
  expect(onopenconflict).toHaveBeenCalledWith(
    expect.objectContaining({ branch: "feat/target" }),
    "src/auth.ts",
  );
  expect(onclose).toHaveBeenCalledOnce();
});

test("preserve next checkbox pointer position and single 1px borders when batch selection appears", async () => {
  await page.viewport(1280, 900);
  render(Harness, { initialState: buildGitWorkbenchFixture("commit-staging") });
  const first = page.getByRole("checkbox", {
    name: "选择未暂存条目 internal/workspace/staging.go",
  });
  const second = page.getByRole("checkbox", {
    name: "选择未暂存条目 docs/git-workbench-notes.md",
  });
  await expect.element(second).toBeVisible();
  await document.fonts.ready;
  const initial = second.element().getBoundingClientRect().top;
  await first.click();
  const batchBar = page.getByTestId("git-discard-batch-bar");
  await expect.element(batchBar).toBeVisible();
  const toolbarStyle = getComputedStyle(
    batchBar.element().firstElementChild as HTMLElement,
  );
  expect(toolbarStyle.borderTopWidth).toBe("1px");
  expect(toolbarStyle.borderBottomWidth).toBe("0px");
  const composerStyle = getComputedStyle(
    page.getByTestId("git-commit-composer").element(),
  );
  expect(composerStyle.borderTopWidth).toBe("1px");
  const selected = second.element().getBoundingClientRect().top;
  await first.click();
  await expect.element(batchBar).not.toBeInTheDocument();
  const cleared = second.element().getBoundingClientRect().top;
  expect(selected).toBeCloseTo(initial, 0);
  expect(cleared).toBeCloseTo(initial, 0);
});

test("scroll file list to keep bottom row visible when batch action bar mounts", async () => {
  await page.viewport(1280, 900);
  const fixture = buildGitWorkbenchFixture("commit-staging");
  fixture.workingChanges = [
    ...fixture.workingChanges,
    ...Array.from({ length: 20 }, (_, idx) => ({
      id: `untracked:extra-${idx}.md`,
      path: `docs/extra-${idx}.md`,
      group: "untracked" as const,
      status: "?" as const,
      hunkStageAvailable: false,
      hunks: [],
    })),
  ];
  render(Harness, { initialState: fixture });
  const listboxLoc = page.getByRole("listbox", { name: "工作树变更条目" });
  await expect.element(listboxLoc).toBeVisible();
  const viewport = listboxLoc
    .element()
    .closest<HTMLElement>('[data-slot="scroll-area-viewport"]')!;
  await document.fonts.ready;

  // Place the final row at the bottom edge before the batch bar appears.
  viewport.scrollTop = viewport.scrollHeight;
  expect(viewport.scrollTop).toBeGreaterThan(0);
  const scrollBefore = viewport.scrollTop;
  const bottomCheckbox = page.getByRole("checkbox", {
    name: "选择未暂存条目 docs/extra-19.md",
  });
  await expect.element(bottomCheckbox).toBeVisible();
  await bottomCheckbox.click();
  await expect.element(page.getByTestId("git-discard-batch-bar")).toBeVisible();

  const viewportRect = viewport.getBoundingClientRect();
  const checkboxRect = bottomCheckbox.element().getBoundingClientRect();
  expect(viewport.scrollTop).toBeGreaterThan(scrollBefore);
  expect(checkboxRect.bottom).toBeLessThanOrEqual(viewportRect.bottom + 0.5);
  expect(checkboxRect.top).toBeGreaterThanOrEqual(viewportRect.top - 0.5);
});

test("GitDiffSelectionBar maps hunk-fallback reader lines to real file line numbers and stages, unstages, or discards selected lines", async () => {
  await page.viewport(1280, 900);
  render(Harness, {
    initialState: buildGitWorkbenchFixture("commit-staging"),
  });

  await page
    .getByTestId("git-change-row-unstaged:staging.go")
    .getByRole("option")
    .click();

  const gutterCell = (side: "deletions" | "additions", lineNumber: number) =>
    document
      .querySelector('[data-testid="git-staging-diff-panel"] diffs-container')
      ?.shadowRoot?.querySelector<HTMLElement>(
        `[data-${side}] [data-column-number="${lineNumber}"]`,
      );

  await expect
    .poll(() => gutterCell("deletions", 2), { timeout: 10000 })
    .toBeTruthy();

  // Reader compact deletion line 2 corresponds to hunk #2 `oldLine: 48`.
  await page.elementLocator(gutterCell("deletions", 2)!).click();

  const bar = page.getByTestId("git-diff-selection-bar");
  await expect.element(bar).toBeVisible();
  await expect
    .element(bar)
    .toMatchTextContent(/internal\/workspace\/staging\.go/);
  await expect.element(bar).toMatchTextContent(/L48/);
  await expect.element(bar).toMatchTextContent(/含 1 行可操作改动/);

  // Stage that single line directly from GitDiffSelectionBar.
  await page.getByTestId("git-selection-stage-lines").click();
  await expect.element(bar).not.toBeInTheDocument();

  const stagedSplitRow = page.getByTestId(
    "git-change-row-staged:internal/workspace/staging.go",
  );
  await expect.element(stagedSplitRow).toBeVisible();

  // Select the staged row and unstage via GitDiffSelectionBar.
  await stagedSplitRow.getByRole("option").click();
  await expect
    .poll(() => gutterCell("deletions", 1), { timeout: 10000 })
    .toBeTruthy();
  await page.elementLocator(gutterCell("deletions", 1)!).click();
  await expect.element(bar).toBeVisible();
  await expect.element(bar).toMatchTextContent(/L48/);
  await page.getByTestId("git-selection-unstage-lines").click();
  expect(stagedSplitRow.elements()).toEqual([]);

  // Select unstaged addition line in hunk #2 (`newLine: 49`) and open discard preview.
  await page
    .getByTestId("git-change-row-unstaged:staging.go")
    .getByRole("option")
    .click();
  await expect
    .poll(() => gutterCell("additions", 3), { timeout: 10000 })
    .toBeTruthy();
  await page.elementLocator(gutterCell("additions", 3)!).click();
  await expect.element(bar).toBeVisible();
  await expect.element(bar).toMatchTextContent(/L49/);
  await page.getByTestId("git-selection-discard-lines").click();
  await expect
    .element(page.getByRole("dialog", { name: /确认丢弃未暂存更改/ }))
    .toBeVisible();
});
