import { afterEach, expect, test } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { buildGitWorkbenchFixture } from "../../stories/git-workbench-fixtures";
import Harness from "./GitWorkbenchHarness.svelte";
import RevisionCompareHarness from "./GitRevisionCompareHarness.svelte";

afterEach(async () => {
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

test("Stash pane renders tracked + untracked (S^3) files and Safe Pop / Apply / Drop actions", async () => {
  await page.viewport(1280, 900);
  render(Harness, {
    initialState: buildGitWorkbenchFixture("stash-view-untracked"),
  });

  await expect.element(page.getByTestId("git-stash-pane")).toBeVisible();
  await expect
    .element(
      page.getByText(/WIP auth session guard \+ untracked notes/).first(),
    )
    .toBeVisible();
  await expect.element(page.getByText("含未跟踪 (S^3)")).toBeVisible();
  await expect
    .element(page.getByText("docs/stash-design-draft.md"))
    .toBeVisible();
  await expect.element(page.getByText("未跟踪 (S^3)")).toBeVisible();

  await expect.element(page.getByTestId("git-stash-pop-btn")).toBeVisible();
  await expect.element(page.getByTestId("git-stash-apply-btn")).toBeVisible();
  await expect.element(page.getByTestId("git-stash-drop-btn")).toBeVisible();
});

test("Commit pane protects non-empty draft when toggling Amend and disables Commit & Push on pushed HEAD", async () => {
  await page.viewport(1280, 900);
  const initial = buildGitWorkbenchFixture("commit-staging");
  initial.commitMessage = "wip: custom local draft";
  initial.commitAmendPushed = true;
  render(Harness, { initialState: initial });

  await page.getByRole("button", { name: /高级提交选项/ }).click();
  await page.getByTestId("git-commit-amend-checkbox").click();

  // Draft protection banner appears without silently overwriting custom draft.
  await expect
    .element(page.getByTestId("git-amend-draft-prompt"))
    .toBeVisible();
  await expect
    .element(page.getByTestId("git-amend-pushed-warning"))
    .toBeVisible();

  // One-click Commit & Push button is disabled when amending a pushed commit!
  await expect
    .element(page.getByRole("button", { name: "提交并推送…" }))
    .toBeDisabled();
});

test("Branches pane renders Tags and redacted Remotes management section", async () => {
  await page.viewport(1280, 900);
  render(Harness, {
    initialState: buildGitWorkbenchFixture("branches-tags-remotes"),
  });

  const branchesTab = page.getByRole("tabpanel");
  await expect.element(branchesTab.getByText("v1.2.0")).toBeVisible();
  await expect.element(page.getByTestId("git-remotes-section")).toBeVisible();
  await expect
    .element(
      page
        .getByTestId("git-remotes-section")
        .getByText("https://user:***@github.com/mzdluo123/webmux.git")
        .first(),
    )
    .toBeVisible();
});

test("Revision compare keeps result viewport stationary on mobile and desktop and retains focus on reload", async () => {
  await page.viewport(390, 844);
  const fixture = buildGitWorkbenchFixture("commit-staging");
  render(RevisionCompareHarness, {
    refs: fixture.branches,
    initialPhase: "error",
  });
  await document.fonts.ready;

  const resultLoc = page.getByTestId("git-rev-compare-result");
  await expect.element(resultLoc).toBeVisible();
  const resultEl = resultLoc.element() as HTMLElement;
  expect(resultEl.tabIndex).toBe(-1);

  await expect
    .element(page.getByTestId("git-rev-compare-target-input"))
    .not.toBeInTheDocument();
  const mobileWorktreeTop = resultEl.getBoundingClientRect().top;

  const worktreeRadio = page.getByRole("radio", { name: "当前工作区文件" });
  const revRadio = page.getByRole("radio", { name: "指定提交" });
  await worktreeRadio.click();
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(revRadio).toHaveAttribute("aria-checked", "true");

  const targetInput = page.getByTestId("git-rev-compare-target-input");
  await expect.element(targetInput).toBeVisible();
  const mobileRevTop = resultEl.getBoundingClientRect().top;
  expect(mobileRevTop).toBeCloseTo(mobileWorktreeTop, 0);

  await userEvent.tab();
  expect(document.activeElement).toBe(targetInput.element());
  await userEvent.tab();
  const retryBtn = page.getByRole("button", { name: "重试加载差异" });
  expect(document.activeElement).toBe(retryBtn.element());

  await retryBtn.click();
  await expect.element(retryBtn).not.toBeInTheDocument();
  expect(document.activeElement).toBe(resultEl);

  await page.viewport(1280, 900);
  const desktopRevTop = resultEl.getBoundingClientRect().top;
  await worktreeRadio.click();
  await expect.element(targetInput).not.toBeInTheDocument();
  const desktopWorktreeTop = resultEl.getBoundingClientRect().top;
  expect(desktopWorktreeTop).toBeCloseTo(desktopRevTop, 0);
});
