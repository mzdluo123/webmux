import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
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
    "export const step = 'seed';\nexport const keep = true;\n",
  );
  await writeFile(join(mainPath, "readme.md"), "# Seed\n");
  git(mainPath, ["add", "."]);
  git(mainPath, ["commit", "-m", "feat: seed repository"]);
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

async function assertNoHorizontalOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    ),
  ).toBe(false);
}

test("auto-captures task baseline on worktree create, aggregates single net diff without double-counting, reviews clean committed task (A09), filters Log scope=task, and pushes without deleting worktree (G12)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainPath, remotePath } = await createRepoWithBareRemote(
    request,
    workDir,
    "task-baseline-flow",
  );
  const baseSeedOid = git(mainPath, ["rev-parse", "HEAD"]).trim();
  const taskWtPath = join(workDir, "task-baseline-flow-wt");

  // 1. Create linked worktree via WebMux API -> automatically captures immutable TaskBaseline + GC ref
  const createRes = await request.post(
    `/api/repositories/${repo.id}/worktrees`,
    {
      data: {
        path: taskWtPath,
        mode: "new",
        branch: "feat/task-547",
        base: "main",
      },
    },
  );
  expect(createRes.status(), await createRes.text()).toBe(201);
  const taskWt = await createRes.json();
  expect(taskWt.branch).toBe("feat/task-547");

  const baselineRes = await request.get(
    `/api/repositories/${repo.id}/worktrees/${taskWt.id}/git/task-baseline`,
  );
  expect(baselineRes.status()).toBe(200);
  const { baseline } = await baselineRes.json();
  expect(baseline.source).toBe("worktree_create");
  expect(baseline.baseOid).toBe(baseSeedOid);
  expect(baseline.cleanAtStart).toBe(true);
  expect(
    git(mainPath, ["rev-parse", `refs/webmux/baselines/${taskWt.id}`]).trim(),
  ).toBe(baseSeedOid);

  // 2. Make 2 Git commits touching service.ts + task-only.ts, PLUS an uncommitted edit on service.ts
  await writeFile(
    join(taskWtPath, "service.ts"),
    "export const step = 'commit-1';\nexport const keep = true;\n",
  );
  git(taskWtPath, ["add", "service.ts"]);
  git(taskWtPath, ["commit", "-m", "feat(task): first agent commit"]);

  await writeFile(
    join(taskWtPath, "task-only.ts"),
    "export const addedByAgent = 42;\n",
  );
  git(taskWtPath, ["add", "task-only.ts"]);
  git(taskWtPath, ["commit", "-m", "feat(task): add task-only module"]);

  // Uncommitted working-tree edit to already-committed service.ts -> MUST appear ONCE in task-changes!
  await writeFile(
    join(taskWtPath, "service.ts"),
    "export const step = 'working-net';\nexport const keep = true;\n",
  );

  await page.goto(
    `/git/${repo.id}/worktrees/${taskWt.id}?view=commit&scope=task`,
  );
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  const reviewPane = page.getByTestId("git-task-review-pane");
  await expect(reviewPane).toBeVisible();
  await expect(reviewPane.getByText("新建工作树自动记录")).toBeVisible();
  await expect(reviewPane).toContainText(
    "任务提交 2 笔 · 当前未提交 1 项 · 单份净变更 2 个文件",
  );
  // service.ts appears exactly once (net diff, never double-counted across commits + dirty worktree)
  await expect(reviewPane.locator('[data-task-file="service.ts"]')).toHaveCount(
    1,
  );
  await expect(
    reviewPane.locator('[data-task-file="task-only.ts"]'),
  ).toHaveCount(1);

  // Advance origin/main and run fetch -> TaskBaseline MUST stay pinned to baseSeedOid!
  const pusherPath = join(workDir, "task-baseline-pusher");
  git(workDir, ["clone", remotePath, pusherPath]);
  await writeFile(join(pusherPath, "upstream-drift.md"), "upstream moved\n");
  git(pusherPath, ["add", "upstream-drift.md"]);
  git(pusherPath, ["commit", "-m", "docs: advance upstream main"]);
  git(pusherPath, ["push", "origin", "main"]);
  const fetchRes = await request.post(`/api/repositories/${repo.id}/fetch`, {
    data: {},
  });
  expect(fetchRes.ok()).toBe(true);

  // 3. Commit the remaining working edit so working tree is 100% clean (A09)
  git(taskWtPath, ["add", "service.ts"]);
  git(taskWtPath, ["commit", "-m", "feat(task): finalize net service change"]);
  const finalTaskHead = git(taskWtPath, ["rev-parse", "HEAD"]).trim();

  await reviewPane
    .getByRole("button", { name: "刷新相对任务起点的变化" })
    .click();
  await expect(reviewPane).toContainText(
    "任务提交 3 笔 · 当前未提交 0 项 · 单份净变更 2 个文件",
  );
  await expect(reviewPane.getByText(baseSeedOid.slice(0, 7))).toBeVisible();

  // Click service.ts in clean worktree -> Pierre CodeDiffReader renders net diff ('seed' -> 'working-net')
  await reviewPane
    .locator('[data-task-file="service.ts"]')
    .getByRole("option")
    .click();
  await expect(
    page
      .getByRole("tabpanel", { name: /提交/ })
      .getByTestId("git-log-diff-panel"),
  ).toContainText("working-net");
  await assertNoHorizontalOverflow(page);

  const backToTaskList = page.getByRole("button", {
    name: "返回任务变更列表",
  });
  if (await backToTaskList.isVisible()) {
    await backToTaskList.click();
  }

  // 4. Switch to Log view and verify scope=task ("当前任务") lists ONLY baseOid..HEAD (3 commits, not seed or upstream)
  await page
    .getByRole("tablist", { name: "Git 工作台视图" })
    .getByRole("tab", { name: /历史/ })
    .click();
  await page
    .getByRole("radiogroup", { name: "提交范围" })
    .getByRole("radio", { name: "当前任务" })
    .click();
  const commitList = page.getByRole("listbox", { name: "提交记录" });
  await expect(commitList.getByText("first agent commit")).toBeVisible();
  await expect(commitList.getByText("add task-only module")).toBeVisible();
  await expect(
    commitList.getByText("finalize net service change"),
  ).toBeVisible();
  await expect(commitList.getByText("seed repository")).toHaveCount(0);
  await assertNoHorizontalOverflow(page);

  // 5. Return to Commit view -> Delivery Summary opens wide Push dialog -> Push succeeds & worktree STILL exists (G12)!
  await page
    .getByRole("tablist", { name: "Git 工作台视图" })
    .getByRole("tab", { name: /提交/ })
    .click();
  const delivery = page.getByTestId("git-task-delivery-summary");
  await expect(delivery).toBeVisible();
  await expect(delivery.getByText("工作区干净")).toBeVisible();
  await delivery.getByRole("button", { name: "推送提交…" }).click();

  const pushDialog = page.getByRole("dialog");
  await expect(pushDialog).toBeVisible();
  await expect(pushDialog.getByText("首次发布上游 (Publish)")).toBeVisible();
  await pushDialog.getByRole("button", { name: "发布并推送" }).click();
  await expect(pushDialog).toHaveCount(0);

  expect(
    git(remotePath, ["rev-parse", "refs/heads/feat/task-547"]).trim(),
  ).toBe(finalTaskHead);
  expect(existsSync(taskWtPath)).toBe(true);
});

test("supports manual baseline selection on existing dirty worktree with honest warning and cleans up GC ref on worktree removal", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "task-manual-dirty",
  );
  const seedOid = git(mainPath, ["rev-parse", "HEAD"]).trim();

  // Add a second commit on main + leave an uncommitted dirty modification before setting baseline
  await writeFile(join(mainPath, "later.ts"), "export const v = 2;\n");
  git(mainPath, ["add", "later.ts"]);
  git(mainPath, ["commit", "-m", "feat: commit after seed"]);
  await writeFile(join(mainPath, "readme.md"), "# Seed with dirty pre-edit\n");

  // Main worktree was registered before any task -> starts with no_task_baseline
  await page.goto(
    `/git/${repo.id}/worktrees/${mainWt.id}?view=commit&scope=task`,
  );
  const reviewPane = page.getByTestId("git-task-review-pane");
  await expect(
    reviewPane.getByRole("button", { name: "将当前 HEAD 设为任务起点" }),
  ).toBeVisible();

  // Navigate to Log and manually select seedOid from commit row menu
  await reviewPane
    .getByRole("button", { name: "前往日志选择历史提交起点" })
    .click();
  await page
    .getByRole("button", {
      name: new RegExp(`提交操作 ${seedOid.slice(0, 7)}`),
    })
    .click();
  await page
    .getByRole("menuitem", {
      name: new RegExp(`设为当前工作树任务基线 \\(${seedOid.slice(0, 7)}`),
    })
    .click();

  // Switch back to Commit -> Task review shows manual_select + honest pre-existing dirty warning
  await page
    .getByRole("tablist", { name: "Git 工作台视图" })
    .getByRole("tab", { name: /提交/ })
    .click();
  await expect(reviewPane.getByText("手动指定起点")).toBeVisible();
  await expect(
    reviewPane.getByText("相对所选提交的全部变化（含设定前已存在修改）"),
  ).toBeVisible();
  await assertNoHorizontalOverflow(page);

  // Clearing baseline removes refs/webmux/baselines/<worktreeId>
  expect(
    git(mainPath, ["rev-parse", `refs/webmux/baselines/${mainWt.id}`]).trim(),
  ).toBe(seedOid);
  await reviewPane.getByRole("button", { name: "清除基线" }).click();
  await expect(
    reviewPane.getByRole("button", { name: "将当前 HEAD 设为任务起点" }),
  ).toBeVisible();
  const afterClear = await request.get(
    `/api/repositories/${repo.id}/worktrees/${mainWt.id}/git/task-baseline`,
  );
  expect((await afterClear.json()).baseline).toBeNull();

  // Creating & removing a linked worktree cleans up its TaskBaseline and GC ref
  const tempWtPath = join(workDir, "task-manual-dirty-linked");
  const createdWtRes = await request.post(
    `/api/repositories/${repo.id}/worktrees`,
    {
      data: {
        path: tempWtPath,
        mode: "new",
        branch: "feat/temp-remove",
        base: "main",
      },
    },
  );
  expect(createdWtRes.status()).toBe(201);
  const tempWt = await createdWtRes.json();
  expect(
    git(mainPath, ["show-ref", `refs/webmux/baselines/${tempWt.id}`]).trim(),
  ).not.toBe("");

  const previewRes = await request.post(
    `/api/repositories/${repo.id}/worktrees/${tempWt.id}/removal-preview`,
    { data: {} },
  );
  expect(previewRes.status()).toBe(200);
  const preview = await previewRes.json();
  const rmRes = await request.post(
    `/api/repositories/${repo.id}/worktrees/${tempWt.id}/remove`,
    { data: { confirmationId: preview.confirmationId, mode: "safe" } },
  );
  expect(rmRes.status()).toBe(204);
  expect(() =>
    git(mainPath, [
      "show-ref",
      "--verify",
      `refs/webmux/baselines/${tempWt.id}`,
    ]),
  ).toThrow();
});
