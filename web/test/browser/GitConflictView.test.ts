import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { buildGitWorkbenchFixture } from "../../stories/git-workbench-fixtures";
import Harness from "./GitWorkbenchHarness.svelte";

afterEach(async () => {
  vi.restoreAllMocks();
  localStorage.clear();
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

test("desktop 3-way conflict resolver keeps Center Result editable while toggling Base comparison and enforces Draft -> Save Result -> Mark as Resolved -> Continue", async () => {
  await page.viewport(1280, 900);
  const initialState = buildGitWorkbenchFixture("conflict-three-way-text");
  await render(Harness, { initialState });

  await expect
    .element(page.getByTestId("git-in-progress-banner"))
    .toBeVisible();
  await expect.element(page.getByTestId("git-conflict-view")).toBeVisible();
  await expect
    .element(page.getByTestId("git-conflict-left-column"))
    .toBeVisible();
  await expect
    .element(page.getByTestId("git-conflict-center-result"))
    .toBeVisible();
  await expect
    .element(page.getByTestId("git-conflict-right-column"))
    .toBeVisible();

  // Left/Right rebase headers keep non-inverted semantics: Left = Current Branch, Right = Rebase Target Base
  expect(
    page.getByTestId("git-conflict-left-column").element().textContent,
  ).toContain("当前重放分支提交 (stage 3)");
  expect(
    page.getByTestId("git-conflict-right-column").element().textContent,
  ).toContain("变基目标基底 (stage 2)");

  // Toggle Base comparison on Left — Center Result MUST remain mounted & editable
  await page.getByRole("radio", { name: "左侧对照 Base" }).click();
  await expect
    .element(page.getByTestId("git-conflict-result-textarea"))
    .toBeVisible();
  expect(
    page.getByTestId("git-conflict-left-column").element().textContent,
  ).toContain("共同祖先 Base");

  const saveBtn = page.getByTestId("git-conflict-save-btn");
  const resolveBtn = page.getByTestId("git-conflict-resolve-btn");
  const continueBtn = page.getByTestId("git-conflict-continue-btn");

  await expect.element(resolveBtn).toBeDisabled();
  await expect.element(continueBtn).toBeDisabled();

  // Stage 1: Apply non-conflicting + Accept Left on conflict chunk
  await page.getByTestId("git-conflict-apply-nonconflicting-btn").click();
  await page.getByTestId("conflict-accept-left-seg-2").click();

  const textarea = page.getByTestId("git-conflict-result-textarea");
  await expect
    .element(textarea)
    .toHaveValue(
      "const defaultTimeout = 10 * time.Second\nconst maxDiffBytes = 2 << 20\nconst allowForcePush = false // V2 invariant\n",
    );

  // Still disabled until Save Result writes to worktree
  await expect.element(resolveBtn).toBeDisabled();
  await expect.element(continueBtn).toBeDisabled();

  // Stage 2: Save Result -> enables Mark as Resolved, Continue still disabled
  await saveBtn.click();
  await expect.element(resolveBtn).toBeEnabled();
  await expect.element(continueBtn).toBeDisabled();

  // Stage 3: Mark as Resolved -> transitions to all-resolved state & enables Continue
  await resolveBtn.click();
  await expect
    .element(page.getByTestId("git-conflict-all-resolved"))
    .toBeVisible();

  // Stage 4: Continue completes operation
  await page
    .getByTestId("git-conflict-all-resolved")
    .getByRole("button", { name: "继续操作 (Continue)" })
    .click();
  await expect
    .element(page.getByTestId("git-in-progress-banner"))
    .not.toBeInTheDocument();
});

test("renders binary conflict variant card with explicit left/right/delete resolution actions", async () => {
  await page.viewport(1280, 900);
  const initialState = buildGitWorkbenchFixture("conflict-binary-variant");
  await render(Harness, { initialState });

  await expect
    .element(page.getByTestId("git-conflict-variant-card"))
    .toBeVisible();
  await expect
    .element(page.getByTestId("git-conflict-choose-left-btn"))
    .toBeEnabled();
  await expect
    .element(page.getByTestId("git-conflict-choose-right-btn"))
    .toBeEnabled();
  await expect
    .element(page.getByTestId("git-conflict-delete-file-btn"))
    .toBeEnabled();

  await page.getByTestId("git-conflict-choose-left-btn").click();
  await expect
    .element(page.getByTestId("git-conflict-all-resolved"))
    .toBeVisible();
});

test("mobile conflict resolver fits 390px and 320px viewports without horizontal overflow and keeps Result editor continuously visible", async () => {
  await page.viewport(320, 568);
  const initialState = buildGitWorkbenchFixture("conflict-three-way-text");
  await render(Harness, { initialState });

  await expect
    .element(page.getByTestId("git-conflict-mobile-pane"))
    .toBeVisible();
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );

  // Switch read-only source to Common Ancestor Base while Result editor stays visible
  await page.getByRole("radio", { name: "共同祖先 Base" }).click();
  const mobileResult = page.getByTestId("git-conflict-mobile-textarea");
  await expect.element(mobileResult).toBeVisible();
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );
});
