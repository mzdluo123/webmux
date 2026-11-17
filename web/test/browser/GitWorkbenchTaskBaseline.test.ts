import { afterEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { buildGitWorkbenchFixture } from "../../stories/git-workbench-fixtures";
import Harness from "./GitWorkbenchHarness.svelte";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

test("clean agent-committed worktree supports task net-diff review, delivery summary and direct Push preview without uncommitted changes", async () => {
  await page.viewport(1440, 920);
  const initialState = buildGitWorkbenchFixture("task-baseline-review");
  render(Harness, { initialState });

  const reviewScopeGroup = page.getByRole("radiogroup", { name: "审阅范围" });
  await expect
    .element(
      reviewScopeGroup.getByRole("radio", { name: /相对任务起点的变化/ }),
    )
    .toHaveAttribute("aria-checked", "true");

  const taskPane = page.getByTestId("git-task-review-pane");
  await expect.element(taskPane.getByText("2222222")).toBeVisible();
  await expect.element(taskPane.getByText("新建工作树自动记录")).toBeVisible();
  await expect
    .element(
      taskPane.getByText(/任务提交 2 笔 · 当前未提交 0\s+项 · 单份净变更/),
    )
    .toBeVisible();

  const summary = page.getByTestId("git-task-delivery-summary");
  await expect.element(summary.getByText("工作区干净")).toBeVisible();
  await expect
    .element(summary.getByText(/推送不会自动移除工作树/))
    .toBeVisible();

  // Switch to uncommitted scope: clean worktree shows compact delivery banner & one-click return to task review.
  await reviewScopeGroup.getByRole("radio", { name: /未提交改动/ }).click();
  await expect
    .element(page.getByText("当前工作区无未提交改动 · 任务交付概览"))
    .toBeVisible();
  await page.getByRole("button", { name: /审阅相对任务起点的变化/ }).click();
  await expect
    .element(
      reviewScopeGroup.getByRole("radio", { name: /相对任务起点的变化/ }),
    )
    .toHaveAttribute("aria-checked", "true");

  // Open wide Push preview directly from clean task review (A09).
  await summary.getByRole("button", { name: "推送提交…" }).click();
  await expect
    .element(page.getByRole("dialog", { name: /推送确认/ }))
    .toBeVisible();
});

test("dirty-start notice, clearing baseline, manual HEAD reset and Log row baseline selection work coherently", async () => {
  await page.viewport(1440, 920);
  const state = buildGitWorkbenchFixture("task-baseline-review");
  state.taskBaseline = {
    baselineId: "bl-wt-main",
    repositoryId: "repo-story",
    worktreeId: "wt-main",
    worktreePath: "/work/webmux",
    source: "manual_select",
    unborn: false,
    baseOid: "2222222222222222222222222222222222222222",
    capturedAt: "2026-04-10T10:05:00Z",
    cleanAtStart: false,
    preExistingChanges: {
      staged: 0,
      unstaged: 2,
      untracked: 1,
      conflicted: 0,
    },
  };
  if (state.taskReview) {
    state.taskReview = {
      ...state.taskReview,
      baseline: state.taskBaseline,
    };
  }
  render(Harness, { initialState: state });

  // Honest dirty-start disclosure is displayed.
  await expect
    .element(page.getByText("相对所选提交的全部变化（含设定前已存在修改）"))
    .toBeVisible();

  // Clear baseline -> transitions to no_task_baseline with explicit action to set HEAD.
  const taskPane = page.getByTestId("git-task-review-pane");
  await taskPane.getByRole("button", { name: "清除基线" }).click();
  await expect
    .element(taskPane.getByText(/当前工作树尚未记录任务起点基线/))
    .toBeVisible();
  await taskPane
    .getByRole("button", { name: "将当前 HEAD 设为任务起点" })
    .click();
  await expect.element(taskPane.getByText("1111111")).toBeVisible();

  // Switch to Log mode -> set an older commit as task baseline from row action menu.
  await page.getByRole("tab", { name: /历史/ }).click();
  await page.getByRole("button", { name: "提交操作 3333333" }).click();
  await page
    .getByRole("menuitem", { name: /设为当前工作树任务基线 \(3333333\)/ })
    .click();
  await expect.element(page.getByText("任务起点")).toBeVisible();
});

test("phone viewport navigates task review files -> diff cleanly without horizontal overflow", async () => {
  await page.viewport(390, 844);
  const state = buildGitWorkbenchFixture("task-baseline-review");
  render(Harness, { initialState: state });

  const taskPane = page.getByTestId("git-task-review-pane");
  await expect.element(taskPane).toBeVisible();

  const overflow = document.documentElement.scrollWidth > window.innerWidth;
  expect(overflow).toBe(false);

  await taskPane
    .getByRole("option", { name: /web\/src\/git-graph-layout\.ts/ })
    .click();
  const diffPanel = page.getByTestId("git-log-diff-panel");
  await expect
    .element(diffPanel)
    .toHaveAttribute("data-diff-presentation", "mobile");
  const backBtn = diffPanel.getByRole("button", { name: "返回任务变更列表" });
  await expect.element(backBtn).toBeVisible();
  await backBtn.click();
  await expect.element(taskPane).toBeVisible();
});
