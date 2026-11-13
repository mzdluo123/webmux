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
  "stash-view-untracked",
  "stash-pop-conflict-preserved",
  "commit-amend-pushed-warning",
  "branches-tags-and-remotes",
  "file-history-and-blame",
  "sub-hunk-line-staging",
  "discard-preview-dialog",
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
        story === "abort-operation-dialog" ||
        story === "file-history-and-blame" ||
        story === "discard-preview-dialog"
      ) {
        await expectDialogFits(page, dialog, name);
      }
      expect.soft(failures, name).toEqual([]);
    });
  });
}

for (const viewport of [
  { width: 1280, height: 720 },
  { width: 390, height: 844 },
  { width: 320, height: 568 },
]) {
  test(`native git controls align at ${viewport.width}px`, async ({ page }) => {
    await page.setViewportSize(viewport);
    for (const story of ["log-merge-topology", "commit-hunk-staging"]) {
      await page.goto(
        `/iframe.html?id=workspace-git-workbench--${story}&viewMode=story&globals=theme:light`,
      );
      await expect(page.getByTestId("git-workbench-story")).toBeVisible();
      await expectNoOverflow(page, `${story} ${viewport.width}`);
      const mismatches = await page.evaluate(() => {
        const controls = [
          ...document.querySelectorAll<HTMLElement>(
            '[data-slot="button"], input[data-slot="input"], [data-slot="native-select"]',
          ),
        ].filter((el) => el.getBoundingClientRect().width > 0);
        const rows = new Map<string, number[]>();
        for (const el of controls) {
          const box = el.getBoundingClientRect();
          const parent = el.parentElement!;
          const key = `${[...document.querySelectorAll("*")].indexOf(parent)}:${Math.round(box.top)}`;
          const heights = rows.get(key) ?? [];
          heights.push(box.height);
          rows.set(key, heights);
        }
        return [...rows.values()].filter(
          (heights) =>
            heights.length > 1 &&
            Math.max(...heights) - Math.min(...heights) > 1,
        );
      });
      expect(mismatches).toEqual([]);
    }
  });
}
