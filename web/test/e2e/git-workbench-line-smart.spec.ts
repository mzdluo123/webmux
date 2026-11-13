import { execFileSync } from "node:child_process";
import { access, mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import { createShell, expect, test } from "./fixtures";

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

async function ensureChangesListVisible(page: Page) {
  const backBtn = page.getByRole("button", { name: "返回变更列表" });
  if (await backBtn.isVisible()) {
    await backBtn.click();
  }
}

function buildServiceContent(mode: "base" | "edited"): string {
  const lines: string[] = ["export const header = 0;"];
  if (mode === "base") {
    lines.push("export const a1 = 'old-1';");
    lines.push("export const a2 = 'old-2';");
    lines.push("export const a3 = 'old-3';");
  } else {
    lines.push("export const a1 = 'new-1';");
    lines.push("export const a2 = 'new-2';");
    lines.push("export const a3 = 'new-3';");
  }
  for (let i = 5; i <= 16; i++) {
    lines.push(`export const keep${i} = ${i};`);
  }
  lines.push(
    mode === "base"
      ? "export const tail = 'old-tail';"
      : "export const tail = 'new-tail';",
  );
  return `${lines.join("\n")}\n`;
}

async function createSeededRepo(
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

  await writeFile(join(mainPath, "service.ts"), buildServiceContent("base"));
  await writeFile(join(mainPath, "crlf-tail.txt"), "alpha\r\nbeta");
  git(mainPath, ["add", "."]);
  git(mainPath, ["commit", "-m", "feat: seed v3 repo"]);
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
  return { repo, mainWt, mainPath };
}

test("stages and unstages arbitrary sub-hunk lines with CRLF/no-newline fidelity while leaving working tree untouched (#551)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createSeededRepo(
    request,
    workDir,
    "v3-subhunk-lines",
  );

  const editedService = buildServiceContent("edited");
  await writeFile(join(mainPath, "service.ts"), editedService);
  await writeFile(join(mainPath, "crlf-tail.txt"), "alpha\r\nbeta-next");
  const expectedServiceBytes = await readFile(join(mainPath, "service.ts"));

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  const stagedSec = page.locator('[data-change-section="staged"]');
  const unstagedSec = page.locator('[data-change-section="unstaged"]');

  // 1. Stage CRLF + no-trailing-newline file via sub-hunk line selector
  await unstagedSec
    .locator('[data-slot="list-button"]', { hasText: "crlf-tail.txt" })
    .click();
  const toggleBtn = page.getByTestId("git-toggle-line-selector");
  const selector = page.getByTestId("git-staging-line-selector");
  await expect(toggleBtn).toBeVisible();
  if ((await toggleBtn.getAttribute("aria-pressed")) !== "true") {
    await toggleBtn.click();
  }
  await expect(selector).toBeVisible();
  await expect(selector.getByText("CRLF")).toBeVisible();
  await expect(selector.getByText("无末尾换行")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  await selector.getByRole("button", { name: "全选此块" }).click();
  await page.getByTestId("git-line-stage-btn").click();
  await expect
    .poll(() => git(mainPath, ["show", ":crlf-tail.txt"]))
    .toBe("alpha\r\nbeta-next");

  // 2. Stage ONLY lines 2 & 4 of adjacent 3-line modification in service.ts (leaving line 3 & hunk #2 unstaged)
  await ensureChangesListVisible(page);
  await unstagedSec
    .locator('[data-slot="list-button"]', { hasText: "service.ts" })
    .click();
  await expect(toggleBtn).toBeVisible();
  if ((await toggleBtn.getAttribute("aria-pressed")) !== "true") {
    await toggleBtn.click();
  }
  await expect(selector).toBeVisible();

  await selector
    .getByRole("checkbox", { name: /选择变更块 #1 第 2 行删除/ })
    .click();
  await selector
    .getByRole("checkbox", { name: /选择变更块 #1 第 2 行新增/ })
    .click();
  await selector
    .getByRole("checkbox", { name: /选择变更块 #1 第 4 行删除/ })
    .click();
  await selector
    .getByRole("checkbox", { name: /选择变更块 #1 第 4 行新增/ })
    .click();
  await page.getByTestId("git-line-stage-btn").click();

  await expect
    .poll(() => git(mainPath, ["show", ":service.ts"]))
    .toContain("a1 = 'new-1'");
  expect(git(mainPath, ["show", ":service.ts"])).toContain("a2 = 'old-2'");
  expect(git(mainPath, ["show", ":service.ts"])).toContain("a3 = 'new-3'");
  expect(git(mainPath, ["show", ":service.ts"])).toContain("tail = 'old-tail'");
  expect(
    (await readFile(join(mainPath, "service.ts"))).equals(expectedServiceBytes),
  ).toBe(true);

  // 3. Unstage ONLY line 4 (`a3`) from staged `.git/index` while keeping `a1` staged and worktree untouched
  await ensureChangesListVisible(page);
  await expect(stagedSec.getByText("service.ts")).toBeVisible();
  await expect(unstagedSec.getByText("service.ts")).toBeVisible();
  await stagedSec
    .locator('[data-slot="list-button"]', { hasText: "service.ts" })
    .click();
  await expect(toggleBtn).toBeVisible();
  if ((await toggleBtn.getAttribute("aria-pressed")) !== "true") {
    await toggleBtn.click();
  }
  await expect(selector).toBeVisible();
  await selector.getByRole("checkbox", { name: /第 4 行删除/ }).click();
  await selector.getByRole("checkbox", { name: /第 4 行新增/ }).click();
  await page.getByTestId("git-line-unstage-btn").click();

  await expect
    .poll(() => git(mainPath, ["show", ":service.ts"]))
    .toContain("a3 = 'old-3'");
  expect(git(mainPath, ["show", ":service.ts"])).toContain("a1 = 'new-1'");
  expect(
    (await readFile(join(mainPath, "service.ts"))).equals(expectedServiceBytes),
  ).toBe(true);

  // 4. Commit staged index and verify committed tree vs untouched working tree
  await ensureChangesListVisible(page);
  const subjectInput = page.getByRole("textbox", { name: "提交说明标题" });
  await subjectInput.fill("feat(git): sub-hunk partial line commit");
  await subjectInput.press("Control+Enter");
  const commitDialog = page.getByRole("dialog");
  await expect(commitDialog).toBeVisible();
  await commitDialog.getByRole("button", { name: "确认本地提交" }).click();
  await expect(commitDialog).toHaveCount(0);

  expect(git(mainPath, ["show", "HEAD:service.ts"])).toContain("a1 = 'new-1'");
  expect(git(mainPath, ["show", "HEAD:service.ts"])).toContain("a2 = 'old-2'");
  expect(git(mainPath, ["show", "HEAD:service.ts"])).toContain("a3 = 'old-3'");
  expect(git(mainPath, ["show", "HEAD:crlf-tail.txt"])).toBe(
    "alpha\r\nbeta-next",
  );
  expect(
    (await readFile(join(mainPath, "service.ts"))).equals(expectedServiceBytes),
  ).toBe(true);
});

test("discards tracked & untracked changes with 409 stale guard, protects staged index, and records recovery snapshot OID (#551)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createSeededRepo(
    request,
    workDir,
    "v3-discard-guard",
  );

  // Stage a file in .git/index — Discard MUST NEVER touch it!
  await writeFile(join(mainPath, "staged-keep.ts"), "export const keep = 1;\n");
  git(mainPath, ["add", "staged-keep.ts"]);

  // Create unstaged tracked modification + untracked file
  await writeFile(join(mainPath, "service.ts"), buildServiceContent("edited"));
  await writeFile(join(mainPath, "scratch.txt"), "temporary scratch note\n");

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();

  const stagedRow = page.getByTestId("git-change-row-staged:staged-keep.ts");
  await expect(stagedRow).toBeVisible();
  await expect(stagedRow.getByRole("button", { name: /丢弃更改/ })).toHaveCount(
    0,
  );

  // Multi-select unstaged `service.ts` + untracked `scratch.txt`
  await page
    .getByRole("checkbox", { name: "选择未暂存条目 service.ts" })
    .click();
  await page
    .getByRole("checkbox", { name: "选择未暂存条目 scratch.txt" })
    .click();
  await expect(page.getByTestId("git-discard-batch-bar")).toBeVisible();
  await page.getByTestId("git-batch-discard-btn").click();

  const discardDialog = page.getByRole("dialog", {
    name: /确认丢弃未暂存更改/,
  });
  await expect(discardDialog).toBeVisible();
  await expect(page.getByTestId("git-discard-tracked-list")).toContainText(
    "service.ts",
  );
  await expect(page.getByTestId("git-discard-untracked-list")).toContainText(
    "scratch.txt",
  );

  // Simulate a CLI agent writing new edits AFTER preview opened -> 409 stale rejection!
  const lateAgentText = `${buildServiceContent("edited")}// agent late line\n`;
  await writeFile(join(mainPath, "service.ts"), lateAgentText);

  await discardDialog.getByRole("button", { name: "确认丢弃更改" }).click();
  await expect(page.getByTestId("git-discard-refresh-stale-btn")).toBeVisible();
  expect(await readFile(join(mainPath, "service.ts"), "utf8")).toBe(
    lateAgentText,
  );

  // Refresh preview with the latest file digest and confirm discard
  await page.getByTestId("git-discard-refresh-stale-btn").click();
  await expect(page.getByTestId("git-discard-refresh-stale-btn")).toHaveCount(
    0,
  );
  await discardDialog.getByRole("button", { name: "确认丢弃更改" }).click();
  await expect(discardDialog).toHaveCount(0);

  // Staged index stays intact; tracked file restored; untracked file removed
  expect(git(mainPath, ["diff", "--cached", "--name-only"]).trim()).toBe(
    "staged-keep.ts",
  );
  expect(await readFile(join(mainPath, "service.ts"), "utf8")).toBe(
    buildServiceContent("base"),
  );
  await expect(access(join(mainPath, "scratch.txt"))).rejects.toThrow();

  // Recovery snapshot ref exists in git and recovery command is surfaced in status bar
  const recoveryRefs = git(mainPath, [
    "for-each-ref",
    "--format=%(objectname)",
    "refs/webmux/discard-recovery",
  ]).trim();
  expect(recoveryRefs).toMatch(/^[0-9a-f]{40}$/);
  expect(git(mainPath, ["show", `${recoveryRefs}:service.ts`])).toBe(
    lateAgentText,
  );
  expect(git(mainPath, ["show", `${recoveryRefs}:scratch.txt`])).toBe(
    "temporary scratch note\n",
  );

  await page.getByTestId("git-operations-popover-trigger").click();
  await expect(page.getByTestId("git-operation-recovery-cmd")).toContainText(
    `git restore --source=${recoveryRefs} --worktree`,
  );
});

test("executes 3-phase tracked-stash Smart Checkout with active session guard and guides conflict stash recovery into 3-way GitConflictView (#551)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createSeededRepo(
    request,
    workDir,
    "v3-smart-checkout",
  );

  // Seed `feat/clean` (disjoint file) and `feat/conflict` (conflicting edit on `service.ts`)
  git(mainPath, ["switch", "-c", "feat/clean"]);
  await writeFile(join(mainPath, "clean-feature.ts"), "export const ok = 1;\n");
  git(mainPath, ["add", "."]);
  git(mainPath, ["commit", "-m", "feat: clean branch"]);

  git(mainPath, ["switch", "-c", "feat/conflict", "main"]);
  await writeFile(
    join(mainPath, "service.ts"),
    buildServiceContent("base").replace("old-1", "branch-conflict-1"),
  );
  git(mainPath, ["add", "."]);
  git(mainPath, ["commit", "-m", "feat: conflicting branch"]);
  git(mainPath, ["switch", "main"]);

  // Create a pre-existing user stash to verify Smart Checkout never drops user stashes
  await writeFile(join(mainPath, "user-stash.txt"), "keep user stash\n");
  git(mainPath, ["stash", "push", "-u", "-m", "user-important-stash"]);
  const userStashOid = git(mainPath, ["rev-parse", "refs/stash@{0}"]).trim();

  // Start an active terminal session in `mainPath` to trigger G11 / US-J16 session guard
  await createShell(page, "Agent Worker", mainPath);

  // Dirty the worktree with both tracked and untracked changes
  const localDirtyService = buildServiceContent("edited");
  await writeFile(join(mainPath, "service.ts"), localDirtyService);
  await writeFile(join(mainPath, "wip-untracked.md"), "# Carry me across\n");

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=branches`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  const branchesPane = page.locator('[data-slot="git-branches-pane"]');

  // 1. Clean Smart Checkout: `main` -> `feat/clean` with active session disclosure + acknowledgement
  await branchesPane
    .locator("[data-branch-item-btn]", { hasText: "feat/clean" })
    .click();
  await branchesPane.getByRole("button", { name: "检出到当前工作树…" }).click();

  const cleanDialog = page.getByRole("dialog", { name: /^切换分支：/ });
  await expect(cleanDialog).toBeVisible();
  await expect(
    cleanDialog.getByRole("button", { name: "改为在新工作树打开" }),
  ).toBeVisible();
  await expect(cleanDialog.getByTestId("smart-checkout-body")).toHaveCount(0);

  await cleanDialog.getByTestId("smart-checkout-session-disclosure").click();
  await expect(cleanDialog.getByTestId("smart-checkout-body")).toBeVisible();
  const cleanSubmit = cleanDialog.getByTestId("smart-checkout-submit-btn");
  await expect(cleanSubmit).toBeDisabled();
  await cleanDialog.getByTestId("smart-checkout-ack-sessions").click();
  await expect(cleanSubmit).toBeEnabled();
  await cleanSubmit.click();
  await expect(cleanDialog).toHaveCount(0);

  expect(git(mainPath, ["branch", "--show-current"]).trim()).toBe("feat/clean");
  expect(await readFile(join(mainPath, "service.ts"), "utf8")).toBe(
    localDirtyService,
  );
  expect(await readFile(join(mainPath, "wip-untracked.md"), "utf8")).toBe(
    "# Carry me across\n",
  );
  // Controlled temporary stash was cleanly dropped; user's own stash remains at stash@{0}!
  expect(git(mainPath, ["rev-parse", "refs/stash@{0}"]).trim()).toBe(
    userStashOid,
  );

  // 2. Conflicting Smart Checkout: `feat/clean` -> `feat/conflict` preserves controlled stash & opens 3-way GitConflictView
  await branchesPane
    .locator("[data-branch-item-btn]", { hasText: "feat/conflict" })
    .click();
  await branchesPane.getByRole("button", { name: "检出到当前工作树…" }).click();

  const conflictDialog = page.getByRole("dialog", { name: /^切换分支：/ });
  await expect(conflictDialog).toBeVisible();
  await conflictDialog.getByTestId("smart-checkout-session-disclosure").click();
  await conflictDialog.getByTestId("smart-checkout-ack-sessions").click();
  await conflictDialog.getByTestId("smart-checkout-submit-btn").click();

  await expect(
    conflictDialog.getByTestId("smart-checkout-conflict-card"),
  ).toBeVisible();
  await expect(
    conflictDialog.getByTestId("smart-checkout-recovery-cmd"),
  ).toContainText("git stash apply");
  expect(git(mainPath, ["branch", "--show-current"]).trim()).toBe(
    "feat/conflict",
  );

  await conflictDialog.getByTestId("smart-checkout-open-conflict-btn").click();
  await expect(conflictDialog).toHaveCount(0);
  await expect(page.getByTestId("git-conflict-view")).toBeVisible();
  await expect(
    page
      .getByTestId("git-conflict-view")
      .getByText(/Smart Checkout 贮藏/)
      .filter({ visible: true })
      .first(),
  ).toBeVisible();
  await assertNoHorizontalOverflow(page);
});
