import { afterEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import {
  GitCommitOutcomeSchema,
  GitCommitPreviewModelSchema,
  type GitWorkbenchViewState,
} from "../../src/contracts/git-workbench-schemas";
import {
  buildGitWorkbenchFixture,
  type GitWorkbenchScenario,
} from "../../stories/git-workbench-fixtures";
import GitCommitPreviewDialog from "../../src/components/workspace/GitCommitPreviewDialog.svelte";
import GitStashApplyDialog from "../../src/components/workspace/GitStashApplyDialog.svelte";
import ComposerHarness from "./GitCommitComposerHarness.svelte";
import Harness from "./GitWorkbenchHarness.svelte";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

function fixture(scenario: GitWorkbenchScenario): GitWorkbenchViewState {
  return buildGitWorkbenchFixture(scenario);
}

test("Ctrl+Enter in Commit Subject or Body only opens preview dialog and never commits directly (UI-10)", async () => {
  await page.viewport(1280, 900);
  render(Harness, { initialState: fixture("commit-staging") });

  const subjectInput = page.getByRole("textbox", { name: "提交说明标题" });
  await expect.element(subjectInput).toBeVisible();
  await subjectInput.click();
  await userEvent.keyboard("{Control>}{Enter}{/Control}");

  // Must open full index preview dialog rather than executing commit!
  const previewDialog = page.getByRole("dialog");
  await expect.element(previewDialog).toBeVisible();
  await expect
    .element(previewDialog.getByRole("heading", { name: /确认本地提交/ }))
    .toBeVisible();
  await expect.element(previewDialog.getByText("终端预暂存")).toBeVisible();

  // Expand staged file inside preview dialog to inspect HEAD -> index diff (A01).
  await previewDialog
    .getByRole("button", { name: /internal\/workspace\/commit_execute\.go/ })
    .click();
  await expect
    .element(previewDialog.getByTestId("git-commit-preview-diff"))
    .toBeVisible();

  await previewDialog.getByRole("button", { name: "取消" }).click();
  await expect.element(previewDialog).not.toBeInTheDocument();
});

test("stale commit previews pin the disabled action and let users inspect the refreshed list", async () => {
  await page.viewport(390, 844);
  const original = GitCommitPreviewModelSchema.parse({
    repositoryName: "fixture",
    worktreePath: "/fixture",
    headState: "branch",
    branch: "main",
    headOid: "abc",
    expectedTreeOid: "before",
    authorName: "Demo",
    authorEmail: "demo@example.invalid",
    signEnabled: false,
    executableHooks: [],
    stagedFiles: [
      {
        id: "change",
        path: "before.ts",
        group: "staged",
        status: "M",
        hunkStageAvailable: false,
      },
    ],
    unstagedCount: 0,
    untrackedCount: 0,
    conflictedCount: 0,
    followUpPush: false,
    blockers: [],
    warnings: [],
  });
  const preview = await render(GitCommitPreviewDialog, {
    model: original,
    commitMessage: "Keep this draft",
    onConfirm: () => {},
    onRefreshPreview: () => {},
    onClose: () => {},
  });
  const confirm = page.getByTestId("git-commit-preview-confirm-btn");
  await expect.element(confirm).toBeEnabled();
  await expect
    .poll(() => confirm.element().getBoundingClientRect().bottom)
    .toBeLessThan(844);
  const initial = confirm.element().getBoundingClientRect().top;
  await preview.rerender({
    model: { ...original, stale: true, dialogError: "暂存已变化。".repeat(70) },
  });
  await expect.element(confirm).toBeDisabled();
  const refresh = page.getByRole("button", { name: /重新生成.*预览/ });
  await expect.element(refresh).toBeVisible();
  expect(confirm.element().getBoundingClientRect().top).toBeCloseTo(initial, 0);
  await refresh.click();
  await preview.rerender({
    model: {
      ...original,
      expectedTreeOid: "after",
      stagedFiles: [{ ...original.stagedFiles[0], path: "after.ts" }],
    },
  });
  await expect.element(confirm).toBeEnabled();
  await expect
    .element(page.getByText("after.ts", { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByTestId("git-commit-preview-message"))
    .toHaveTextContent("Keep this draft");
  expect(confirm.element().getBoundingClientRect().top).toBeCloseTo(initial, 0);
});

test("needs_review commit outcome in Commit & Push aborts auto-opening Push dialog (A05)", async () => {
  await page.viewport(1280, 900);
  const simulated = GitCommitOutcomeSchema.parse({
    status: "needs_review",
    commitOid: "deadbeef1234567890abcdef1234567890abcdef",
    expectedTreeOid: "tree-before-hook",
    actualTreeOid: "tree-after-hook",
    summary:
      "本地提交已完成，但 pre-commit 钩子格式化改写了暂存树；已自动中止推送。",
    reasons: ["expected tree-before-hook, got tree-after-hook"],
  });

  render(Harness, {
    initialState: fixture("commit-staging"),
    simulatedCommitOutcome: simulated,
  });

  await page.getByRole("button", { name: "提交并推送…" }).click();
  await page.getByRole("button", { name: "确认提交并继续预览推送" }).click();

  // Push dialog MUST NOT auto-open; needs_review outcome banner must be shown instead!
  await expect
    .element(page.getByRole("dialog", { name: "推送确认" }))
    .not.toBeInTheDocument();
  await expect
    .element(
      page.getByText(
        /\[needs_review\][\s\S]*pre-commit 钩子格式化改写了暂存树/,
      ),
    )
    .toBeVisible();
});

test("TopBar popover, Log branch tree, and Branches pane expose identical context-menu and Shift+F10 actions (UI-05, U08)", async () => {
  await page.viewport(1280, 900);
  render(Harness, { initialState: fixture("log-merge") });

  // 1. Log left branch tree: keyboard Shift+F10 on occupied branch feat/git-workbench
  const logTree = page.getByRole("complementary", { name: "历史引用筛选树" });
  const logBranchBtn = logTree
    .getByRole("button", { name: /feat\/git-workbench/ })
    .first();
  logBranchBtn.element().focus();
  await userEvent.keyboard("{Shift>}{F10}{/Shift}");
  await expect
    .element(page.getByRole("menuitem", { name: /打开所在工作树/ }))
    .toBeVisible();
  await expect
    .element(page.getByRole("menuitem", { name: /检出分支/ }))
    .not.toBeInTheDocument();
  await userEvent.keyboard("{Escape}");

  // 2. TopBar searchable branch popover: right-click contextmenu on same occupied branch
  await page.getByTestId("git-topbar-branch-popover-trigger").click();
  const topSearch = page.getByRole("textbox", { name: "顶栏搜索分支" });
  await expect.element(topSearch).toBeVisible();
  const popMenuBtn = page
    .getByRole("button", { name: "顶栏分支操作 feat/git-workbench" })
    .first();
  popMenuBtn
    .element()
    .parentElement?.dispatchEvent(
      new MouseEvent("contextmenu", { bubbles: true, cancelable: true }),
    );
  await expect
    .element(page.getByRole("menuitem", { name: /打开所在工作树/ }))
    .toBeVisible();
  await userEvent.keyboard("{Escape}");
  await userEvent.keyboard("{Escape}");

  // 3. Branches pane: trailing '...' menu on same occupied branch
  await page.getByRole("tab", { name: /分支/ }).click();
  await page
    .getByRole("button", { name: "分支操作 feat/git-workbench" })
    .first()
    .click();
  await expect
    .element(page.getByRole("menuitem", { name: /打开所在工作树/ }))
    .toBeVisible();
});

test("wide ~900px Push dialog lists all outgoing commits and switches file Pierre Diff (UI-06)", async () => {
  await page.viewport(1280, 900);
  render(Harness, { initialState: fixture("push-dialog") });

  const dialog = page.getByRole("dialog");
  await expect.element(dialog).toBeVisible();
  await expect.element(dialog.getByText("待推送提交 (2)")).toBeVisible();

  const width = dialog.element().getBoundingClientRect().width;
  expect(width).toBeGreaterThanOrEqual(820);
  expect(width).toBeLessThanOrEqual(920);

  // Switch selected outgoing commit and verify file list + Pierre Diff update to second commit's file.
  await dialog
    .getByRole("button", { name: /multi-lane commit topology/ })
    .click();
  const singleFileBtn = dialog.getByRole("button", {
    name: /domains\/workspace\/git-graph-layout\.ts/,
  });
  await expect.element(singleFileBtn).toBeVisible();
  const fileListViewport = singleFileBtn
    .element()
    .closest('[data-slot="scroll-area-viewport"]') as HTMLElement;
  expect(fileListViewport.getBoundingClientRect().height).toBeLessThan(64);
  await expect.element(dialog.getByTestId("git-push-file-diff")).toBeVisible();
});

test("Commit draft conflict resolution preserves local selection and scroll on keep-local and resets to 0 on adopt-remote", async () => {
  await page.viewport(390, 844);
  const base = fixture("commit-staging");
  const localBody = Array.from(
    { length: 22 },
    (_, idx) => `Local paragraph line ${idx + 1} with detailed notes`,
  ).join("\n");
  const remoteBody = Array.from(
    { length: 20 },
    (_, idx) => `Remote revision line ${idx + 1} from another browser`,
  ).join("\n");
  const conflict = {
    localSubject: "feat(git): local draft subject",
    localBody,
    remoteDraft: {
      subject: "feat(git): remote draft subject",
      body: remoteBody,
      revision: 3,
      updatedAt: "2026-11-19T00:00:00Z",
    },
    message: "远端已有更新的提交草稿",
  };

  await render(ComposerHarness, {
    props: {
      context: base.context,
      initialMessage: `feat(git): local draft subject\n\n${localBody}`,
      initialConflict: conflict,
    },
  });

  const bodyField = page.getByRole("textbox", { name: "补充正文描述" });
  await expect.element(bodyField).toBeVisible();
  const textarea = bodyField.element() as HTMLTextAreaElement;
  expect(getComputedStyle(textarea).maxHeight).not.toBe("none");

  textarea.scrollTop = 96;
  textarea.setSelectionRange(24, 42);
  const savedScrollTop = textarea.scrollTop;
  expect(savedScrollTop).toBeGreaterThan(0);

  await page.getByTestId("git-draft-keep-local").click();
  await expect
    .element(page.getByTestId("git-commit-draft-conflict"))
    .not.toBeInTheDocument();
  expect(textarea.selectionStart).toBe(24);
  expect(textarea.selectionEnd).toBe(42);
  expect(textarea.scrollTop).toBeCloseTo(savedScrollTop, 0);

  await cleanup();
  await render(ComposerHarness, {
    props: {
      context: base.context,
      initialMessage: `feat(git): local draft subject\n\n${localBody}`,
      initialConflict: conflict,
    },
  });

  const adoptBodyField = page.getByRole("textbox", { name: "补充正文描述" });
  await expect.element(adoptBodyField).toBeVisible();
  const adoptTextarea = adoptBodyField.element() as HTMLTextAreaElement;
  adoptTextarea.scrollTop = 96;
  adoptTextarea.setSelectionRange(24, 42);
  expect(adoptTextarea.scrollTop).toBeGreaterThan(0);

  await page.getByTestId("git-draft-adopt-remote").click();
  await expect
    .element(page.getByTestId("git-commit-draft-conflict"))
    .not.toBeInTheDocument();
  expect(adoptTextarea.value).toBe(remoteBody);
  expect(adoptTextarea.selectionStart).toBe(0);
  expect(adoptTextarea.selectionEnd).toBe(0);
  expect(adoptTextarea.scrollTop).toBe(0);

  await page.viewport(1280, 900);
  expect(getComputedStyle(adoptTextarea).maxHeight).toBe("none");
});

test("Stash apply dialog resets outer ResponsiveDialog scroll on mode toggle even when dialogError is non-empty", async () => {
  await page.viewport(1280, 420);
  const entry = {
    stashRef: "stash@{0}",
    index: 0,
    oid: "1111111111111111111111111111111111111111",
    shortOid: "1111111",
    parents: [],
    hasUntracked: true,
    summary: "WIP stash layout check",
    createdAt: "2026-11-19T00:00:00Z",
  };
  const preview = {
    confirmationId: "conf-stash-1",
    expiresAt: null,
    repositoryId: "repo-1",
    worktreeId: "wt-1",
    worktreePath: "/tmp/wt-1",
    mode: "apply" as const,
    entry,
    reinstateIndex: false,
    detached: false,
    stagedCount: 0,
    unstagedCount: 2,
    untrackedCount: 1,
    overlappingPaths: ["a.ts", "b.ts", "c.ts"],
    sessions: [],
    blockers: Array.from({ length: 8 }, (_, idx) => ({
      code: `blocker-${idx}`,
      message: `冲突阻塞项 #${idx + 1}：需要清理本地改动后再继续恢复贮藏`,
    })),
    warnings: [],
  };
  const { rerender } = await render(GitStashApplyDialog, {
    props: {
      mode: "apply",
      reinstateIndex: false,
      target: entry,
      preview,
      dialogError: "Worktree changed during preview",
      onModeChange: () => {},
      onReinstateIndexChange: () => {},
      onConfirm: () => {},
      onClose: () => {},
    },
  });

  const body = page.getByTestId("git-stash-apply-dialog");
  await expect.element(body).toBeVisible();
  expect(body.element().querySelector('[data-slot="scroll-area"]')).toBeNull();
  const outerViewport = body
    .element()
    .closest('[data-slot="scroll-area-viewport"]') as HTMLElement;
  outerViewport.scrollTop = 120;
  expect(outerViewport.scrollTop).toBeGreaterThan(0);

  await rerender({
    mode: "pop",
    reinstateIndex: false,
    target: entry,
    preview,
    dialogError: "Worktree changed during preview",
    onModeChange: () => {},
    onReinstateIndexChange: () => {},
    onConfirm: () => {},
    onClose: () => {},
  });
  expect(outerViewport.scrollTop).toBe(0);
});
