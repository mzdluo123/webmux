import { execFileSync } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import { expect, test } from "./fixtures";

function git(root: string, args: string[]): string {
  return execFileSync("git", ["-C", root, ...args], {
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "Alice Author",
      GIT_AUTHOR_EMAIL: "alice@example.invalid",
      GIT_COMMITTER_NAME: "Alice Author",
      GIT_COMMITTER_EMAIL: "alice@example.invalid",
    },
  }).toString();
}

async function createRepoWithTaskWorktree(
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
    join(mainPath, "alpha.ts"),
    "export const alpha = 'seed';\nexport const keepAlpha = true;\n",
  );
  await writeFile(
    join(mainPath, "beta.ts"),
    "export const beta = 'seed';\nexport const keepBeta = true;\n",
  );
  git(mainPath, ["add", "."]);
  git(mainPath, ["commit", "-m", "feat: seed repository"]);
  git(mainPath, ["push", "-u", "origin", "main"]);

  const repoRes = await request.post("/api/repositories", {
    data: { path: mainPath },
  });
  expect(repoRes.status(), await repoRes.text()).toBe(201);
  const repo = await repoRes.json();

  const taskWtPath = join(workDir, `${slug}-wt`);
  const wtRes = await request.post(`/api/repositories/${repo.id}/worktrees`, {
    data: {
      path: taskWtPath,
      mode: "new",
      branch: `feat/${slug}`,
      base: "main",
    },
  });
  expect(wtRes.status(), await wtRes.text()).toBe(201);
  const taskWt = await wtRes.json();

  return { repo, taskWt, taskWtPath, mainPath, remotePath };
}

async function assertNoHorizontalOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    ),
  ).toBe(false);
}

test("syncs review progress across devices, rejects stale marks, auto-invalidates on external edit, and jumps to next unreviewed file (G07, A03, A10)", async ({
  browser,
  page,
  request,
  workDir,
}) => {
  const { repo, taskWt, taskWtPath } = await createRepoWithTaskWorktree(
    request,
    workDir,
    "v1-review-sync",
  );

  // Create 1 task commit + 2 uncommitted file edits
  await writeFile(
    join(taskWtPath, "committed.ts"),
    "export const taskCommit = 1;\n",
  );
  git(taskWtPath, ["add", "committed.ts"]);
  git(taskWtPath, ["commit", "-m", "feat(task): add committed module"]);

  await writeFile(
    join(taskWtPath, "alpha.ts"),
    "export const alpha = 'edited-v1';\nexport const keepAlpha = true;\n",
  );
  await writeFile(
    join(taskWtPath, "beta.ts"),
    "export const beta = 'edited-v1';\nexport const keepBeta = true;\n",
  );

  const commitUrl = `/git/${repo.id}/worktrees/${taskWt.id}?view=commit`;
  await page.goto(commitUrl);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  await expect(page.getByTestId("git-review-progress-badge")).toHaveText(
    "已审阅 0/2",
  );

  const unstagedSec = page.locator('[data-change-section="unstaged"]');
  await unstagedSec
    .getByRole("button", { name: "标记已审阅 alpha.ts" })
    .click();
  await expect(page.getByTestId("git-review-progress-badge")).toHaveText(
    "已审阅 1/2",
  );

  // Capture alpha.ts v1 review key from server, then open Device B context to verify cross-device sync
  const reviewsV1Res = await request.get(
    `/api/repositories/${repo.id}/worktrees/${taskWt.id}/git/reviews?scope=uncommitted`,
  );
  expect(reviewsV1Res.status()).toBe(200);
  const reviewsV1 = await reviewsV1Res.json();
  expect({
    scope: reviewsV1.scope,
    totalFiles: reviewsV1.totalFiles,
    reviewedFiles: reviewsV1.reviewedFiles,
  }).toEqual({
    scope: "uncommitted",
    totalFiles: 2,
    reviewedFiles: 1,
  });
  const alphaV1File = reviewsV1.files.find(
    (it: { path: string; reviewed: boolean }) =>
      it.path === "alpha.ts" && it.reviewed,
  );
  expect(alphaV1File).toBeTruthy();

  const deviceB = await browser.newContext({
    baseURL: new URL(page.url()).origin,
    viewport: page.viewportSize() ?? { width: 1440, height: 900 },
  });
  try {
    const pageB = await deviceB.newPage();
    await pageB.goto(commitUrl);
    await expect(pageB.getByTestId("git-review-progress-badge")).toHaveText(
      "已审阅 1/2",
    );
    await expect(
      pageB
        .locator('[data-change-section="unstaged"]')
        .locator('[data-reviewed="true"]'),
    ).toBeVisible();

    // Click "下一处未审阅" on Device B -> jumps directly to beta.ts (and enters Diff step on narrow viewports)
    await pageB.getByTestId("git-next-unreviewed").click();
    await expect(
      pageB.getByRole("region", { name: "工作树文件差异与代码块暂存" }),
    ).toContainText("beta.ts");
    await assertNoHorizontalOverflow(pageB);
  } finally {
    await deviceB.close();
  }

  // Mutate alpha.ts externally -> server auto-invalidates v1 mark AND rejects stale v1 PUT!
  await writeFile(
    join(taskWtPath, "alpha.ts"),
    "export const alpha = 'edited-v2-external';\nexport const keepAlpha = true;\n",
  );

  const stalePutRes = await request.put(
    `/api/repositories/${repo.id}/worktrees/${taskWt.id}/git/reviews`,
    {
      data: {
        scope: "uncommitted",
        action: "mark",
        item: {
          scope: alphaV1File.scope,
          area: alphaV1File.area,
          path: alphaV1File.path,
          oldPath: alphaV1File.oldPath,
          baseMode: alphaV1File.baseMode,
          baseIdentity: alphaV1File.baseIdentity,
          targetMode: alphaV1File.targetMode,
          targetIdentity: alphaV1File.targetIdentity,
        },
      },
    },
  );
  expect(stalePutRes.status()).toBe(200);
  const stalePutJson = await stalePutRes.json();
  expect(stalePutJson.applied).toBe(false);
  expect(stalePutJson.staleRejected).toBe(true);
  expect(stalePutJson.reviewedFiles).toBe(0);

  await page.getByRole("button", { name: "刷新暂存区快照" }).click();
  await expect(page.getByTestId("git-review-progress-badge")).toHaveText(
    "已审阅 0/2",
  );

  // Switch to scope=task ("相对任务起点的变化") -> verify server review sync + next unreviewed jump in task scope
  await page
    .getByRole("radiogroup", { name: "审阅范围" })
    .getByRole("radio", { name: /相对任务起点的变化/ })
    .click();
  await expect(page.getByTestId("git-review-progress-badge")).toHaveText(
    "已审阅 0/3",
  );
  await page
    .getByTestId("git-task-review-pane")
    .getByRole("button", { name: "标记已审阅 alpha.ts" })
    .click();
  await expect(page.getByTestId("git-review-progress-badge")).toHaveText(
    "已审阅 1/3",
  );
  await page.getByTestId("git-next-unreviewed").click();
  await expect(
    page
      .getByRole("tabpanel", { name: /提交/ })
      .getByTestId("git-log-diff-panel"),
  ).toContainText("beta.ts");
  await assertNoHorizontalOverflow(page);
});

test("enforces optimistic revision check on cross-device commit drafts with side-by-side 409 conflict resolution and clears draft only on real commit (G03)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, taskWt, taskWtPath } = await createRepoWithTaskWorktree(
    request,
    workDir,
    "v1-commit-draft",
  );

  await writeFile(
    join(taskWtPath, "alpha.ts"),
    "export const alpha = 'ready-to-commit';\nexport const keepAlpha = true;\n",
  );
  git(taskWtPath, ["add", "alpha.ts"]);

  const commitUrl = `/git/${repo.id}/worktrees/${taskWt.id}?view=commit`;
  await page.goto(commitUrl);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();

  const subjectInput = page.getByRole("textbox", { name: "提交说明标题" });
  const bodyInput = page.getByRole("textbox", { name: "补充正文描述" });

  // Device A saves initial draft -> revision #1
  await subjectInput.fill("feat(git): local draft v1");
  await bodyInput.fill("local detail from page A");
  await expect(page.getByTestId("git-commit-draft-synced")).toContainText(
    "草稿已同步 #1",
  );

  // Another device updates the draft on the server -> revision #2
  const remotePut = await request.put(
    `/api/repositories/${repo.id}/worktrees/${taskWt.id}/git/commit-draft`,
    {
      data: {
        subject: "feat(git): remote draft v2",
        body: "remote detail from tablet B",
        baseRevision: 1,
      },
    },
  );
  expect(remotePut.status()).toBe(200);
  expect((await remotePut.json()).revision).toBe(2);

  // Device A (still at baseRevision=1) edits its local draft -> receives 409 draft_conflict without silent overwrite!
  await bodyInput.fill("local detail edited offline on page A");
  const conflictCard = page.getByTestId("git-commit-draft-conflict");
  await expect(conflictCard).toBeVisible();
  await expect(page.getByTestId("git-draft-conflict-local")).toContainText(
    "local detail edited offline on page A",
  );
  await expect(page.getByTestId("git-draft-conflict-remote")).toContainText(
    "remote detail from tablet B",
  );
  await expect(page.getByTestId("git-draft-conflict-remote")).toContainText(
    "rev #2",
  );
  await assertNoHorizontalOverflow(page);

  // Choose "合并两份内容" -> merges both sides and saves revision #3
  await page.getByTestId("git-draft-merge-both").click();
  await expect(conflictCard).toHaveCount(0);
  await expect(page.getByTestId("git-commit-draft-synced")).toContainText(
    "草稿已同步 #3",
  );
  await expect(bodyInput).toHaveValue(/local detail edited offline on page A/);
  await expect(bodyInput).toHaveValue(/remote detail from tablet B/);

  // Execute real commit -> server clears commit draft!
  await page.getByRole("button", { name: "提交预览…" }).click();
  const previewDialog = page.getByRole("dialog");
  await expect(previewDialog).toBeVisible();
  await previewDialog.getByRole("button", { name: "确认本地提交" }).click();
  await expect(previewDialog).toHaveCount(0);

  await expect
    .poll(async () => {
      const res = await request.get(
        `/api/repositories/${repo.id}/worktrees/${taskWt.id}/git/commit-draft`,
      );
      const json = await res.json();
      return `${json.subject}|${json.body}`;
    })
    .toBe("|");
});
