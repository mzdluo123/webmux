import { execFileSync } from "node:child_process";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import {
  createShell,
  expect,
  expectTerminalLine,
  runCommand,
  test,
  viewSwitch,
} from "./fixtures";

function git(
  root: string,
  args: string[],
  envOverrides: Record<string, string> = {},
): string {
  return execFileSync("git", ["-C", root, ...args], {
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "Alice Author",
      GIT_AUTHOR_EMAIL: "alice@example.invalid",
      GIT_COMMITTER_NAME: "Alice Author",
      GIT_COMMITTER_EMAIL: "alice@example.invalid",
      ...envOverrides,
    },
  }).toString();
}

async function assertNoHorizontalOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    ),
  ).toBe(false);
}

async function createRepoWithBareRemote(
  request: APIRequestContext,
  workDir: string,
  slug: string,
) {
  const remotePath = join(workDir, `${slug}-origin.git`);
  const mainPath = join(workDir, slug);
  await mkdir(remotePath, { recursive: true });
  git(remotePath, ["init", "--bare", "-b", "main"]);

  await mkdir(mainPath, { recursive: true });
  git(mainPath, ["init", "-b", "main"]);
  git(mainPath, ["config", "user.name", "Alice Author"]);
  git(mainPath, ["config", "user.email", "alice@example.invalid"]);
  git(mainPath, ["remote", "add", "origin", remotePath]);

  await writeFile(
    join(mainPath, "service.ts"),
    [
      "// Header section",
      "export const host = 'localhost';",
      "",
      "// Port section",
      "export const port = 7681;",
      "",
      "// Footer section",
      "export const mode = 'dev';",
      "",
    ].join("\n"),
  );
  git(mainPath, ["add", "."]);
  git(mainPath, ["commit", "-m", "feat: seed service"]);
  git(mainPath, ["push", "-u", "origin", "main"]);

  const res = await request.post("/api/repositories", {
    data: { path: mainPath },
  });
  expect(res.status(), await res.text()).toBe(201);
  const repo = await res.json();
  const mainWt = repo.worktrees.find(
    (w: { kind: string }) => w.kind === "main",
  );
  expect(mainWt).toBeTruthy();
  return { repo, mainWt, mainPath, remotePath };
}

test("creates stash with untracked files, inspects S^3 diff, preserves stash on conflicting Safe Pop into GitConflictView (no Continue), and cleanly pops & drops stashes (#550)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "v2-stash-flow",
  );

  // 1. Modify tracked file + add untracked file in worktree
  await writeFile(
    join(mainPath, "service.ts"),
    [
      "// Header section",
      "export const host = '0.0.0.0';",
      "",
      "// Port section",
      "export const port = 9090;",
      "",
      "// Footer section",
      "export const mode = 'dev';",
      "",
    ].join("\n"),
  );
  await writeFile(join(mainPath, "scratch-note.txt"), "untracked stash note\n");

  const viewportWidth = page.viewportSize()?.width ?? 1440;
  const isWideStash = viewportWidth >= 1024;
  const isDesktopConflict = viewportWidth >= 1280;

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  // 2. Open Create Stash dialog from Commit pane quick button (auto-checks includeUntracked)
  await page.getByTestId("git-commit-stash-changes-btn").click();
  const createDialog = page.getByTestId("git-stash-create-dialog");
  await expect(createDialog).toBeVisible();
  await page.getByLabel("贮藏备注说明（选填）").fill("WIP: port 9090 & note");
  await expect(page.getByTestId("git-stash-create-confirm-btn")).toBeEnabled();
  await page.getByTestId("git-stash-create-confirm-btn").click();
  await expect(createDialog).toHaveCount(0);

  expect(git(mainPath, ["status", "--porcelain"]).trim()).toBe("");
  const firstStashOid = git(mainPath, [
    "rev-parse",
    "--verify",
    "refs/stash@{0}",
  ]).trim();

  // 3. Switch to Stash view and inspect tracked + untracked S^3 files
  await page.getByTestId("git-mode-tab-stash").click();
  await expect(page.getByTestId("git-stash-pane")).toBeVisible();
  await expect(page.locator('[data-stash-row="stash@{0}"]')).toContainText(
    "WIP: port 9090 & note",
  );
  await assertNoHorizontalOverflow(page);

  if (!isWideStash) {
    await page.locator('[data-stash-row="stash@{0}"]').click();
  }
  await expect(
    page.locator('[data-stash-file-row="scratch-note.txt"]'),
  ).toHaveAttribute("data-untracked", "true");
  await page.locator('[data-stash-file-row="scratch-note.txt"]').click();
  await expect(page.getByTestId("git-stash-pane")).toContainText(
    "untracked stash note",
  );
  if (!isWideStash) {
    await page.getByRole("button", { name: "返回贮藏文件列表" }).click();
  }

  // 4. Create an overlapping edit in worktree so Safe Pop hits a merge conflict!
  await writeFile(
    join(mainPath, "service.ts"),
    [
      "// Header section",
      "export const host = 'localhost';",
      "",
      "// Port section",
      "export const port = 8888;",
      "",
      "// Footer section",
      "export const mode = 'prod';",
      "",
    ].join("\n"),
  );
  git(mainPath, ["add", "service.ts"]);
  git(mainPath, ["commit", "-m", "feat: conflicting port 8888"]);

  await page.getByTestId("git-stash-pop-btn").click();
  const applyDialog = page.getByTestId("git-stash-apply-dialog");
  await expect(applyDialog).toBeVisible();
  await expect(page.getByTestId("git-stash-apply-confirm-btn")).toBeEnabled();
  await page.getByTestId("git-stash-apply-confirm-btn").click();

  // 5. Safe Two-Step Pop invariant: stash@{0} MUST still exist on disk & GitConflictView opens WITHOUT Continue!
  await expect(page.getByTestId("git-conflict-view")).toBeVisible();
  expect(
    git(mainPath, ["rev-parse", "--verify", "refs/stash@{0}"]).trim(),
  ).toBe(firstStashOid);
  await expect(page.getByTestId("git-continue-operation-btn")).toHaveCount(0);
  await expect(page.getByTestId("git-conflict-continue-btn")).toHaveCount(0);
  await assertNoHorizontalOverflow(page);

  // Verify Left = 当前工作树分支 / Right = 贮藏变更, resolve & mark resolved
  await page.getByTestId("git-conflict-apply-nonconflicting-btn").click();
  if (isDesktopConflict) {
    await expect(page.getByTestId("git-conflict-left-column")).toContainText(
      "当前工作树分支",
    );
    await expect(page.getByTestId("git-conflict-right-column")).toContainText(
      "贮藏变更",
    );
    await page
      .getByTestId("git-conflict-right-column")
      .getByRole("button", { name: "接受右侧" })
      .click();
  } else {
    await page
      .getByTestId("git-conflict-mobile-pane")
      .getByRole("button", { name: "接受右侧" })
      .click();
  }
  await page.getByTestId("git-conflict-save-btn").click();
  await page.getByTestId("git-conflict-resolve-btn").click();
  await expect(page.getByTestId("git-conflict-all-resolved")).toBeVisible();
  expect(git(mainPath, ["ls-files", "-u"]).trim()).toBe("");

  // Close conflict view and drop the preserved stash after verifying recovery command
  await page.getByTestId("git-conflict-back-btn").click();
  await page.getByTestId("git-mode-tab-stash").click();
  if (!isWideStash) {
    await page.locator('[data-stash-row="stash@{0}"]').click();
  }
  await page.getByTestId("git-stash-drop-btn").click();
  const dropDialog = page.getByTestId("git-stash-drop-dialog");
  await expect(dropDialog).toBeVisible();
  await expect(page.getByTestId("git-stash-drop-recovery-cmd")).toContainText(
    `git stash apply ${firstStashOid}`,
  );
  await page.getByTestId("git-stash-drop-confirm-btn").click();
  await expect(dropDialog).toHaveCount(0);
  expect(git(mainPath, ["stash", "list"]).trim()).toBe("");

  // 6. Clean Stash Create + Clean Safe Pop removes stash entry automatically
  git(mainPath, ["reset", "--hard", "HEAD"]);
  git(mainPath, ["clean", "-fd"]);
  await writeFile(join(mainPath, "clean-pop.txt"), "clean pop payload\n");
  await page.getByTestId("git-stash-open-create-btn").click();
  await page.getByTestId("git-stash-include-untracked-checkbox").click();
  await page.getByLabel("贮藏备注说明（选填）").fill("clean pop item");
  await page.getByTestId("git-stash-create-confirm-btn").click();
  await expect(page.locator('[data-stash-row="stash@{0}"]')).toContainText(
    "clean pop item",
  );
  if (!isWideStash) {
    await page.locator('[data-stash-row="stash@{0}"]').click();
  }
  await page.getByTestId("git-stash-pop-btn").click();
  await page.getByTestId("git-stash-apply-confirm-btn").click();
  await expect(page.getByTestId("git-stash-apply-dialog")).toHaveCount(0);
  expect(git(mainPath, ["stash", "list"]).trim()).toBe("");
  expect(await readFile(join(mainPath, "clean-pop.txt"), "utf8")).toBe(
    "clean pop payload\n",
  );
});

test("amends unpushed commit with Author override & Sign-off, then enforces pushed-HEAD Amend warning, disables Commit & Push, and requires acknowledgement (#550)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "v2-commit-amend",
  );

  // Create an unpushed local commit first
  await writeFile(join(mainPath, "feature.ts"), "export const v = 1;\n");
  git(mainPath, ["add", "feature.ts"]);
  git(mainPath, ["commit", "-m", "feat: unpushed draft v1"]);

  // Stage an update to fold into --amend
  await writeFile(join(mainPath, "feature.ts"), "export const v = 2;\n");
  git(mainPath, ["add", "feature.ts"]);

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();

  // Type a draft first so toggling Amend exercises draft protection prompt!
  await page.getByLabel("提交说明标题").fill("my temporary local note");
  await page
    .getByTestId("git-commit-advanced-options")
    .getByRole("button", { name: /高级提交选项/ })
    .click();
  await page.getByTestId("git-commit-amend-checkbox").click();

  await expect(page.getByTestId("git-amend-draft-prompt")).toBeVisible();
  await page.getByRole("button", { name: "替换为 HEAD 说明" }).click();
  await expect(page.getByLabel("提交说明标题")).toHaveValue(
    "feat: unpushed draft v1",
  );
  await expect(page.getByTestId("git-amend-pushed-warning")).toHaveCount(0);

  // Override Author + enable Sign-off
  await page.getByLabel("提交说明标题").fill("feat: amended v2 with signoff");
  await page.getByTestId("git-commit-signoff-checkbox").click();
  await expect(page.getByTestId("git-commit-signoff-preview")).toContainText(
    "Signed-off-by: Alice Author <alice@example.invalid>",
  );
  await page
    .getByTestId("git-commit-author-input")
    .fill("Bob Coauthor <bob@example.invalid>");

  await page.getByRole("button", { name: "提交预览…" }).click();
  await expect(page.getByTestId("git-commit-preview-message")).toContainText(
    "Signed-off-by: Alice Author <alice@example.invalid>",
  );
  await page.getByTestId("git-commit-preview-confirm-btn").click();
  await expect(page.getByTestId("git-commit-preview-confirm-btn")).toHaveCount(
    0,
  );

  // Verify Git recorded Bob as Author, Alice as Committer, and Signed-off-by trailer!
  await expect
    .poll(() => git(mainPath, ["log", "-1", "--pretty=%an <%ae>|%cn <%ce>|%B"]))
    .toContain(
      "Bob Coauthor <bob@example.invalid>|Alice Author <alice@example.invalid>|feat: amended v2 with signoff",
    );
  expect(git(mainPath, ["log", "-1", "--pretty=%B"])).toContain(
    "Signed-off-by: Alice Author <alice@example.invalid>",
  );

  // Push commit to origin so HEAD is now pushed/reachable from upstream
  git(mainPath, ["push", "origin", "main"]);
  await page.reload();
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();

  // Toggle Amend on pushed HEAD -> warning banner + Commit & Push disabled + preview ack gate!
  await page
    .getByTestId("git-commit-advanced-options")
    .getByRole("button", { name: /高级提交选项/ })
    .click();
  await page.getByTestId("git-commit-amend-checkbox").click();
  await expect(page.getByTestId("git-amend-pushed-warning")).toBeVisible();
  await expect(page.getByTestId("git-commit-and-push-btn")).toBeDisabled();
  await assertNoHorizontalOverflow(page);

  await page.getByLabel("提交说明标题").fill("feat: local rewrite after push");
  await page.getByRole("button", { name: "提交预览…" }).click();
  await expect(
    page.getByTestId("git-preview-pushed-amend-guard"),
  ).toBeVisible();
  await expect(
    page.getByTestId("git-commit-preview-confirm-btn"),
  ).toBeDisabled();

  await page.getByTestId("git-preview-ack-pushed-amend-checkbox").click();
  await expect(
    page.getByTestId("git-commit-preview-confirm-btn"),
  ).toBeEnabled();
  await page.getByTestId("git-commit-preview-confirm-btn").click();

  // Branch diverged locally (ahead 1, behind 1) and never force-pushed!
  await expect
    .poll(() =>
      git(mainPath, [
        "rev-list",
        "--left-right",
        "--count",
        "HEAD...origin/main",
      ]).trim(),
    )
    .toBe("1\t1");
});

test("renames local branch, configures & unsets upstream, creates & deletes annotated tag with recovery command, and redacts remote URL credentials (#550)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "v2-refs-remotes",
  );
  const headOid = git(mainPath, ["rev-parse", "HEAD"]).trim();

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=branches`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  // 1. Rename checked-out local branch `main` -> `feat/renamed-main`
  await page.getByTestId("git-branch-detail-rename-btn").click();
  await expect(page.getByTestId("git-branch-rename-dialog")).toBeVisible();
  await page.getByTestId("git-branch-rename-input").fill("feat/renamed-main");
  await expect(page.getByTestId("git-branch-rename-confirm-btn")).toBeEnabled();
  await page.getByTestId("git-branch-rename-confirm-btn").click();
  await expect(page.getByTestId("git-branch-rename-dialog")).toHaveCount(0);
  expect(git(mainPath, ["branch", "--show-current"]).trim()).toBe(
    "feat/renamed-main",
  );

  // 2. Unset upstream, then set upstream back to `origin/main`
  await page.getByTestId("git-branch-detail-upstream-btn").click();
  await expect(page.getByTestId("git-branch-upstream-dialog")).toBeVisible();
  await page.getByTestId("git-branch-upstream-unset-btn").click();
  await expect(page.getByTestId("git-branch-upstream-dialog")).toHaveCount(0);
  expect(() =>
    git(mainPath, [
      "rev-parse",
      "--abbrev-ref",
      "feat/renamed-main@{upstream}",
    ]),
  ).toThrow();

  await page.getByTestId("git-branch-detail-upstream-btn").click();
  await page.getByTestId("git-branch-upstream-input").fill("origin/main");
  await page.getByTestId("git-branch-upstream-confirm-btn").click();
  await expect(page.getByTestId("git-branch-upstream-dialog")).toHaveCount(0);
  expect(
    git(mainPath, [
      "rev-parse",
      "--abbrev-ref",
      "feat/renamed-main@{upstream}",
    ]).trim(),
  ).toBe("origin/main");

  // 3. Create Annotated Tag v1.0.0 -> inspect metadata -> Delete Tag with recovery command
  await page.getByTestId("git-branches-create-tag-head-btn").click();
  await expect(page.getByTestId("git-tag-create-dialog")).toBeVisible();
  await page.getByTestId("git-tag-name-input").fill("v1.0.0");
  await page
    .getByTestId("git-tag-message-input")
    .fill("Release v1.0.0 milestone");
  await page.getByTestId("git-tag-create-confirm-btn").click();
  await expect(page.getByTestId("git-tag-create-dialog")).toHaveCount(0);
  expect(git(mainPath, ["cat-file", "-t", "refs/tags/v1.0.0"]).trim()).toBe(
    "tag",
  );

  await page
    .locator('[data-slot="git-branches-pane"] [data-branch-item-btn]')
    .filter({ hasText: "v1.0.0" })
    .click();
  await expect(page.getByTestId("git-branch-detail-card")).toContainText(
    "附注标签",
  );
  await expect(page.getByTestId("git-branch-detail-card")).toContainText(
    "Release v1.0.0 milestone",
  );

  await page.getByTestId("git-branch-detail-delete-tag-btn").click();
  await expect(page.getByTestId("git-tag-delete-dialog")).toBeVisible();
  await expect(page.getByTestId("git-tag-delete-recovery-cmd")).toContainText(
    `git tag v1.0.0 ${headOid}`,
  );
  await page.getByTestId("git-tag-delete-confirm-btn").click();
  await expect(page.getByTestId("git-tag-delete-dialog")).toHaveCount(0);
  expect(git(mainPath, ["tag", "-l", "v1.0.0"]).trim()).toBe("");

  // 4. Add Remote with embedded token -> verify redacted `***` display + literal `***` write guard + Remove
  await page.getByTestId("git-remote-add-btn").click();
  await expect(page.getByTestId("git-remote-edit-dialog")).toBeVisible();
  await page.getByTestId("git-remote-name-input").fill("mirror");
  await page
    .getByTestId("git-remote-fetch-input")
    .fill(
      "https://deploy:ghp_super_secret_token@example.invalid/team/repo.git",
    );
  await page.getByTestId("git-remote-save-btn").click();
  await expect(page.getByTestId("git-remote-edit-dialog")).toHaveCount(0);

  const mirrorRow = page.locator('[data-remote-row="mirror"]');
  await expect(mirrorRow).toContainText(
    "https://deploy:***@example.invalid/team/repo.git",
  );
  await expect(mirrorRow).not.toContainText("ghp_super_secret_token");

  await page.getByRole("button", { name: "编辑远程 mirror" }).click();
  await expect(page.getByTestId("git-remote-edit-dialog")).toBeVisible();
  await expect(page.getByTestId("git-remote-save-btn")).toBeDisabled();
  await page.getByRole("button", { name: "取消" }).click();

  await page.getByRole("button", { name: "移除远程 mirror" }).click();
  await expect(page.getByTestId("git-remote-remove-dialog")).toBeVisible();
  await page.getByTestId("git-remote-remove-confirm-btn").click();
  await expect(mirrorRow).toHaveCount(0);
});

test("inspects --follow file history across rename, line-level Blame with parent re-blame & Log jump, revision compare, and Session Code panel entry (#550)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "v2-history-blame",
  );
  const seedOid = git(mainPath, ["rev-parse", "HEAD"]).trim();

  // Rename `service.ts` -> `core-service.ts` and modify a line so `--follow` + parent re-blame are exercised
  git(mainPath, ["mv", "service.ts", "core-service.ts"]);
  await writeFile(
    join(mainPath, "core-service.ts"),
    [
      "// Header section",
      "export const host = 'localhost';",
      "",
      "// Port section",
      "export const port = 7682;",
      "",
      "// Footer section",
      "export const mode = 'Renamed v2';",
      "",
    ].join("\n"),
  );
  git(mainPath, ["add", "-A"]);
  git(mainPath, ["commit", "-m", "refactor: rename service to core-service"]);
  const renameOid = git(mainPath, ["rev-parse", "HEAD"]).trim();

  // Make an uncommitted worktree edit so it also appears in Commit working changes
  await writeFile(
    join(mainPath, "core-service.ts"),
    [
      "// Header section",
      "export const host = 'localhost';",
      "",
      "// Port section",
      "export const port = 7682;",
      "",
      "// Footer section",
      "export const mode = 'Working draft';",
      "",
    ].join("\n"),
  );

  // 1. Open from Git Workbench Commit view -> verify --follow History, Blame parent chain, Compare, and Jump to Log
  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await page
    .getByRole("button", { name: "查看文件历史与 Blame core-service.ts" })
    .click();

  const inspector = page.getByTestId("git-file-inspector-dialog");
  await expect(inspector).toBeVisible();
  await expect(page.getByTestId("git-file-history-rename-badge")).toContainText(
    "service.ts → core-service.ts",
  );
  await expect(
    page.locator(`[data-file-history-row="${seedOid.slice(0, 8)}"]`),
  ).toBeVisible();
  await assertNoHorizontalOverflow(page);

  // Switch to Blame tab -> re-blame parent across rename (`service.ts`) -> breadcrumb step-back
  await page.getByTestId("git-inspector-tab-blame").click();
  await expect(page.getByTestId("git-blame-view")).toBeVisible();
  await expect(page.locator('[data-blame-line="5"]')).toContainText(
    "export const port = 7682;",
  );
  await page
    .locator(`[data-blame-gutter="${renameOid.slice(0, 8)}"]`)
    .first()
    .click();
  await page.getByTestId("git-blame-parent-btn").click();
  await expect(page.getByTestId("git-blame-breadcrumb")).toBeVisible();
  await expect(page.getByTestId("git-blame-breadcrumb")).toContainText(
    "service.ts",
  );

  // Switch to Revision Compare tab -> verify base/target controls render without overflow
  await page.getByTestId("git-inspector-tab-compare").click();
  await expect(page.getByTestId("git-revision-compare-view")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  // Switch back to Blame and click "在历史图中定位" -> closes dialog and jumps to Log commit!
  await page.getByTestId("git-inspector-tab-blame").click();
  await page.getByTestId("git-blame-jump-log-btn").click();
  await expect(inspector).toHaveCount(0);
  await expect(page).toHaveURL(/view=log/);

  // 2. Verify Session Code workspace header entry (`code-git-file-inspector-trigger`)
  await createShell(page, "Blame shell", mainPath);
  await runCommand(page, "printf 'blame-shell-ready\\n'");
  await expectTerminalLine(page, "blame-shell-ready");
  await viewSwitch(page.locator(".terminal-header"), "代码").click();
  await expect(page.getByRole("region", { name: "代码工作区" })).toBeVisible();
  await page.getByRole("button", { name: /^core-service\.ts(?: |$)/ }).click();
  await expect(
    page.getByTestId("code-git-file-inspector-trigger"),
  ).toBeVisible();
  await page.getByTestId("code-git-file-inspector-trigger").click();
  await page.getByRole("menuitem", { name: "逐行溯源 (Blame)" }).click();
  await expect(page.getByTestId("git-file-inspector-dialog")).toBeVisible();
  await expect(page.getByTestId("git-blame-view")).toContainText(
    "export const port = 7682;",
  );
  await assertNoHorizontalOverflow(page);
});
