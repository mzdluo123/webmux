import { expect, test } from "@playwright/test";
import { checkEachLayout, expectDialogFits, expectNoOverflow } from "./layouts";

for (const story of [
  "log-merge-topology",
  "commit-hunk-staging",
  "commit-full-index-preview",
  "task-baseline-clean-committed-review",
  "branches-worktree-occupied",
  "push-confirmation-dialog",
  "no-worktree-readonly",
  "empty-unborn-repository",
  "index-lock-busy",
  "commit-needs-review",
  "operation-uncertain-recovery",
  "commit-draft-conflict-409",
  "rework-session-handoff",
  "conflict-three-way-text",
  "conflict-binary-variant",
  "divergence-rebase-preview",
  "abort-operation-dialog",
] as const) {
  test(`git workbench ${story} fits every layout`, async ({ page }) => {
    test.slow();
    const failures: string[] = [];
    page.on("pageerror", (error) => failures.push(error.message));
    await page.route("**/api/**", (route) => route.abort());
    await page.goto(
      `/iframe.html?id=workspace-git-workbench--${story}&viewMode=story&globals=theme:light`,
    );
    await expect(page.getByTestId("git-workbench-story")).toBeVisible();
    const dialog = page.getByRole("dialog");

    await checkEachLayout(page, async ({ name }) => {
      await expectNoOverflow(page, name);
      if (
        story === "commit-full-index-preview" ||
        story === "push-confirmation-dialog" ||
        story === "rework-session-handoff" ||
        story === "divergence-rebase-preview" ||
        story === "abort-operation-dialog"
      ) {
        await expectDialogFits(page, dialog, name);
      }
      expect.soft(failures, name).toEqual([]);
    });
  });
}
