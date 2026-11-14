import { afterEach, expect, test } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import {
  GitCommitOutcomeSchema,
  type GitWorkbenchViewState,
} from "../../src/contracts/git-workbench-schemas";
import {
  buildGitWorkbenchFixture,
  type GitWorkbenchScenario,
} from "../../stories/git-workbench-fixtures";
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
    .element(page.getByText(/推送确认 · main/))
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
  await expect
    .element(
      dialog.getByRole("button", {
        name: /domains\/workspace\/git-graph-layout\.ts/,
      }),
    )
    .toBeVisible();
  await expect.element(dialog.getByTestId("git-push-file-diff")).toBeVisible();
});
