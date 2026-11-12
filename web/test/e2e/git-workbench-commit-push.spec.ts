import { execFileSync } from "node:child_process";
import { chmod, mkdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import { expect, test } from "./fixtures";

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

function buildTwoHunkContent(variant: "base" | "edited" | "external"): string {
  const lines: string[] = [];
  for (let i = 1; i <= 20; i++) {
    if (i === 2 && variant !== "base") {
      lines.push(`export const hunkOne = '${variant}';`);
    } else if (i === 18 && variant !== "base") {
      lines.push(`export const hunkTwo = '${variant}';`);
    } else {
      lines.push(`export const line${i} = ${i};`);
    }
  }
  return `${lines.join("\n")}\n`;
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

  await writeFile(join(mainPath, "app.ts"), buildTwoHunkContent("base"));
  await writeFile(join(mainPath, "cli-pre.ts"), "export const cli = 0;\n");
  await writeFile(
    join(mainPath, "asset.bin"),
    Buffer.from([0x00, 0x89, 0x50, 0x4e, 0x47, 0x00, 0x01]),
  );
  git(mainPath, ["add", "."]);
  git(mainPath, ["commit", "-m", "feat: initial seed"]);
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

async function ensureChangesListVisible(page: Page) {
  const backBtn = page.getByRole("button", { name: "返回变更列表" });
  if (await backBtn.isVisible()) {
    await backBtn.click();
  }
}

test("stages single hunk to real index (A02), invalidates reviewed mark on external edit (A03), refreshes 409 stale preview (A04), and commits full index including CLI pre-staged & degraded binary (A01, A12)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "commit-staging",
  );

  // Pre-stage cli-pre.ts from CLI before opening workbench (A01)
  await writeFile(join(mainPath, "cli-pre.ts"), "export const cli = 1;\n");
  git(mainPath, ["add", "cli-pre.ts"]);

  // Edit 2 separate hunks in app.ts and binary asset.bin
  await writeFile(join(mainPath, "app.ts"), buildTwoHunkContent("edited"));
  await writeFile(
    join(mainPath, "asset.bin"),
    Buffer.from([0x00, 0x89, 0x50, 0x4e, 0x47, 0x00, 0x02, 0xff]),
  );

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();

  // Verify no horizontal overflow even on 320px phone
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    ),
  ).toBe(false);

  const stagedSec = page.locator('[data-change-section="staged"]');
  const unstagedSec = page.locator('[data-change-section="unstaged"]');
  await expect(stagedSec.getByText("cli-pre.ts")).toBeVisible();
  await expect(stagedSec.getByText("CLI 预暂存")).toBeVisible();

  // Mark app.ts reviewed without staging, then mutate app.ts externally -> reviewed mark invalidates (A03)!
  await unstagedSec.getByRole("button", { name: "标记已审阅 app.ts" }).click();
  await expect(unstagedSec.getByText("已审阅", { exact: true })).toBeVisible();

  await writeFile(join(mainPath, "app.ts"), buildTwoHunkContent("external"));
  await page.getByRole("button", { name: "刷新暂存区快照" }).click();
  await expect(unstagedSec.getByText("已审阅", { exact: true })).toHaveCount(0);

  // Open app.ts diff and stage ONLY hunk #1 via the non-hover hunk action strip (A02)
  await unstagedSec
    .locator('[data-slot="list-button"]', { hasText: "app.ts" })
    .click();
  const hunkStrip = page.getByTestId("git-hunk-action-strip");
  await expect(hunkStrip).toBeVisible();
  await expect(hunkStrip).toContainText("共 2 块");
  await hunkStrip.getByRole("button", { name: "暂存此块" }).first().click();

  await ensureChangesListVisible(page);
  // Partially staged app.ts MUST appear in BOTH Staged and Unstaged!
  await expect(stagedSec.getByText("app.ts")).toBeVisible();
  await expect(unstagedSec.getByText("app.ts")).toBeVisible();
  expect(git(mainPath, ["diff", "--cached", "app.ts"])).toContain("hunkOne");
  expect(git(mainPath, ["diff", "--cached", "app.ts"])).not.toContain(
    "hunkTwo",
  );
  expect(git(mainPath, ["diff", "app.ts"])).toContain("hunkTwo");

  // Stage degraded binary asset.bin whole-file (A12)
  await unstagedSec.getByRole("button", { name: "暂存文件 asset.bin" }).click();
  await expect(stagedSec.getByText("asset.bin")).toBeVisible();

  // Compose Subject + Body and open full index Commit preview (A01)
  await page
    .getByRole("textbox", { name: "提交说明标题" })
    .fill("feat(git): partial hunk and cli prestaged");
  const bodyInput = page.getByRole("textbox", { name: "补充正文描述" });
  await bodyInput.fill("Reviewed full HEAD -> index preview.");
  await bodyInput.press("Control+Enter");

  const previewDialog = page.getByRole("dialog");
  await expect(previewDialog).toBeVisible();
  await expect(previewDialog.getByText("终端预暂存")).toBeVisible();
  await expect(previewDialog.getByText("cli-pre.ts")).toBeVisible();
  await expect(previewDialog.getByText("app.ts")).toBeVisible();
  await expect(previewDialog.getByText("asset.bin")).toBeVisible();

  // Expand cli-pre.ts to inspect real HEAD -> index Pierre Diff inside preview dialog
  await previewDialog.getByRole("button", { name: /cli-pre\.ts/ }).click();
  await expect(
    previewDialog.getByTestId("git-commit-preview-diff"),
  ).toContainText("cli = 1");

  // Mutate .git/index externally while preview dialog is open -> 409 stale preserves draft & offers 1-click refresh (A04)!
  await writeFile(join(mainPath, "late-cli.ts"), "export const late = true;\n");
  git(mainPath, ["add", "late-cli.ts"]);

  await previewDialog.getByRole("button", { name: "确认本地提交" }).click();
  await expect(
    previewDialog.getByRole("button", { name: "重新生成提交预览" }),
  ).toBeVisible();

  await previewDialog.getByRole("button", { name: "重新生成提交预览" }).click();
  await expect(previewDialog.getByText("late-cli.ts")).toBeVisible();

  await previewDialog.getByRole("button", { name: "确认本地提交" }).click();
  await expect(previewDialog).toHaveCount(0);

  // Verify real Git commit on disk contains staged hunk #1 + CLI files, and excludes unstaged hunk #2!
  const logSubject = git(mainPath, ["log", "-1", "--pretty=%s"]).trim();
  expect(logSubject).toBe("feat(git): partial hunk and cli prestaged");
  expect(git(mainPath, ["show", "HEAD:app.ts"])).toContain("hunkOne");
  expect(git(mainPath, ["show", "HEAD:app.ts"])).not.toContain("hunkTwo");
  expect(git(mainPath, ["diff", "app.ts"])).toContain("hunkTwo");
});

test("aborts Push when pre-commit hook rewrites tree (A05), reports two-step failure when remote advances (A08, U07), and pushes multiple commits via wide/fullscreen Push dialog (G05, UI-06)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath, remotePath } = await createRepoWithBareRemote(
    request,
    workDir,
    "commit-push",
  );
  const isDesktop = (page.viewportSize()?.width ?? 1440) >= 1280;

  // Create 1 already-ahead commit so Push dialog has multiple outgoing commits!
  await writeFile(join(mainPath, "ahead-1.ts"), "export const aheadOne = 1;\n");
  git(mainPath, ["add", "ahead-1.ts"]);
  git(mainPath, ["commit", "-m", "feat: first unpushed commit"]);

  // Install executable pre-commit hook that formats & re-stages hooked.ts -> triggers needs_review (A05)
  const hookPath = join(mainPath, ".git", "hooks", "pre-commit");
  await writeFile(
    hookPath,
    "#!/bin/sh\nif [ -f hooked.ts ]; then\n  printf '// formatted by hook\\n' >> hooked.ts\n  git add hooked.ts\nfi\n",
  );
  await chmod(hookPath, 0o755);

  await writeFile(join(mainPath, "hooked.ts"), "export const hooked = true;\n");
  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);

  const untrackedSec = page.locator('[data-change-section="untracked"]');
  await untrackedSec
    .getByRole("button", { name: "暂存文件 hooked.ts" })
    .click();
  await page
    .getByRole("textbox", { name: "提交说明标题" })
    .fill("feat: commit rewritten by pre-commit hook");
  await page.getByRole("button", { name: "提交并推送…" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "确认提交并继续预览推送" })
    .click();

  // Outcome MUST be needs_review, and Push dialog MUST NOT auto-open!
  await expect(page.getByTestId("git-commit-outcome-banner")).toContainText(
    "needs_review",
  );
  await expect(page.getByText(/推送确认 · main/)).toHaveCount(0);
  await rm(hookPath);

  // Now stage clean.ts and run Commit & Push -> matched -> auto-opens Push dialog with 3 outgoing commits!
  await writeFile(join(mainPath, "clean.ts"), "export const clean = true;\n");
  await page.getByRole("button", { name: "刷新暂存区快照" }).click();
  await untrackedSec.getByRole("button", { name: "暂存文件 clean.ts" }).click();
  await page
    .getByRole("textbox", { name: "提交说明标题" })
    .fill("feat: matched commit entering push preview");
  await page.getByRole("button", { name: "提交并推送…" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "确认提交并继续预览推送" })
    .click();

  const pushDialog = page.getByRole("dialog");
  await expect(pushDialog).toBeVisible();
  await expect(pushDialog.getByText("待推送提交 (3)")).toBeVisible();
  await expect(pushDialog).toContainText("第一步已完成：本地提交");

  if (isDesktop) {
    const box = await pushDialog.boundingBox();
    expect(box!.width).toBeGreaterThanOrEqual(840);
    expect(box!.width).toBeLessThanOrEqual(920);
  } else {
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth > window.innerWidth,
      ),
    ).toBe(false);
  }

  // Inspect an earlier outgoing commit's file diff inside the Push dialog
  await pushDialog
    .getByTestId("git-push-commits-list")
    .getByRole("button", { name: /first unpushed commit/ })
    .click();
  await expect(pushDialog.getByTestId("git-push-file-diff")).toContainText(
    "aheadOne",
  );

  // Advance bare remote externally so Push fails non-fast-forward -> independent two-step reporting (A08, U07)!
  const pusherPath = join(workDir, "pusher-advance");
  git(workDir, ["clone", remotePath, pusherPath]);
  await writeFile(join(pusherPath, "remote.md"), "remote ahead\n");
  git(pusherPath, ["add", "remote.md"]);
  git(pusherPath, ["commit", "-m", "docs: advance origin main"]);
  git(pusherPath, ["push", "origin", "main"]);

  const localHeadBeforePush = git(mainPath, ["rev-parse", "HEAD"]).trim();
  await pushDialog.getByRole("button", { name: "确认推送" }).click();
  await expect(pushDialog).toContainText(/本地已提交.*推送未完成/);
  await expect(
    pushDialog.getByRole("button", { name: "获取远程 (Fetch)" }),
  ).toBeVisible();
  expect(git(mainPath, ["rev-parse", "HEAD"]).trim()).toBe(localHeadBeforePush);
  await pushDialog.getByRole("button", { name: "取消" }).click();

  // Switch to a new un-published branch feat/publish-e2e and publish via Push dialog -> bare remote receives HEAD!
  git(mainPath, ["checkout", "-b", "feat/publish-e2e"]);
  const refreshRes = await request.get(
    `/api/repositories/${repo.id}/worktrees`,
  );
  expect(refreshRes.ok()).toBe(true);
  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=branches`);
  await page
    .getByRole("region", { name: "选中分支详情与工作树占用" })
    .getByRole("button", { name: /发布分支/ })
    .click();

  const pubDialog = page.getByRole("dialog");
  await expect(pubDialog).toBeVisible();
  await expect(pubDialog.getByText("首次发布上游 (Publish)")).toBeVisible();
  await pubDialog.getByRole("button", { name: "发布并推送" }).click();
  await expect(pubDialog).toHaveCount(0);

  const remotePublishedOid = git(remotePath, [
    "rev-parse",
    "refs/heads/feat/publish-e2e",
  ]).trim();
  expect(remotePublishedOid).toBe(localHeadBeforePush);
});

test("shares occupancy-safe branch actions, Shift+F10 menu, and dirty/active-session SwitchBranch warning (UI-05, A11, U08)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "branches-shared",
  );
  const occupiedPath = join(workDir, "branches-shared-occupied");

  // Unoccupied branch feat/ready
  git(mainPath, ["checkout", "-b", "feat/ready"]);
  await writeFile(join(mainPath, "ready.ts"), "export const ready = true;\n");
  git(mainPath, ["add", "ready.ts"]);
  git(mainPath, ["commit", "-m", "feat: ready branch"]);
  git(mainPath, ["checkout", "main"]);

  // Occupied linked worktree feat/occupied
  git(mainPath, [
    "worktree",
    "add",
    "-b",
    "feat/occupied",
    occupiedPath,
    "HEAD",
  ]);
  const treesRes = await request.get(`/api/repositories/${repo.id}/worktrees`);
  const trees = (await treesRes.json()).worktrees as {
    id: string;
    branch: string | null;
  }[];
  const occupiedWt = trees.find((w) => w.branch === "feat/occupied")!;
  expect(occupiedWt).toBeTruthy();

  // Start 1 active session in mainWt and 1 active session in occupiedWt!
  for (const wt of [mainWt, occupiedWt]) {
    const sRes = await request.post("/api/sessions", {
      data: {
        cmd: ["/bin/sh", "-c", "sleep 600"],
        name: `Session ${wt.id}`,
        workspace: { repositoryId: repo.id, worktreeId: wt.id },
      },
    });
    expect(sRes.status()).toBe(201);
  }

  // Dirty tracked file in mainWt so SwitchBranch warns about dirty files + active session (A11)
  await writeFile(join(mainPath, "app.ts"), "// uncommitted local edit\n");

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=branches`);
  const listRegion = page.getByRole("region", { name: "分支与上游列表" });

  // Occupied branch shows active session count and navigates to occupied worktree without offering Checkout
  await listRegion.getByRole("button", { name: /^feat\/occupied/ }).click();
  const detailRegion = page.getByRole("region", {
    name: "选中分支详情与工作树占用",
  });
  await expect(detailRegion).toContainText("当前活跃会话：1 个");
  await expect(
    detailRegion.getByRole("button", { name: "打开所在工作树" }),
  ).toBeVisible();
  await expect(detailRegion.getByRole("button", { name: /检出/ })).toHaveCount(
    0,
  );

  // Open menu on unoccupied feat/ready via Shift+F10 on desktop or action button on phone (U08)
  const readyBtn = listRegion.getByRole("button", { name: /^feat\/ready/ });
  if ((page.viewportSize()?.width ?? 1440) >= 640) {
    await readyBtn.focus();
    await page.keyboard.press("Shift+F10");
  } else {
    await listRegion
      .getByRole("button", { name: "分支操作 feat/ready" })
      .click();
  }
  await page.getByRole("menuitem", { name: "检出分支…" }).click();

  // SwitchBranchDialog opens pre-selecting feat/ready and displays dirty file & active session warnings (A11)!
  const switchDialog = page.locator('[role="dialog"][data-state="open"]');
  await expect(switchDialog).toBeVisible();
  await expect(switchDialog).toContainText("feat/ready");
  await expect(switchDialog).toContainText("有未提交的修改");
  await expect(switchDialog).toContainText("有 1 个运行中的会话");
  await page.keyboard.press("Escape");
  expect(git(mainPath, ["symbolic-ref", "HEAD"]).trim()).toBe(
    "refs/heads/main",
  );
});
