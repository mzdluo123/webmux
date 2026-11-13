import { execFileSync } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext } from "@playwright/test";
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

  await writeFile(join(mainPath, "seed.ts"), "export const seed = 1;\n");
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

test("records persistent operationId in bottom status bar, survives reload, and reconciles dropped POST /git/commit without duplicate commits (A07, UI-02)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "ops-ledger",
  );

  // 1. Stage ledger.ts and commit normally -> verify 24-28px bottom status bar + Popover + reload persistence
  await writeFile(join(mainPath, "ledger.ts"), "export const ledger = 1;\n");
  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();

  const statusBar = page.getByTestId("git-workbench-status-bar");
  await expect(statusBar).toBeVisible();
  const barBox = await statusBar.boundingBox();
  expect(barBox!.height).toBeGreaterThanOrEqual(24);
  expect(barBox!.height).toBeLessThanOrEqual(28);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    ),
  ).toBe(false);

  const untrackedSec = page.locator('[data-change-section="untracked"]');
  await untrackedSec
    .getByRole("button", { name: "暂存文件 ledger.ts" })
    .click();

  await page
    .getByRole("textbox", { name: "提交说明标题" })
    .fill("feat(git): record operation in status bar");
  await page.getByRole("button", { name: "提交预览…" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "确认本地提交" })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  // Status bar reflects completed commit operation
  const trigger = page.getByTestId("git-operations-popover-trigger");
  await expect(trigger).toContainText("提交 · 已完成");
  await trigger.click();

  const popover = page.getByTestId("git-operations-popover");
  await expect(popover).toBeVisible();
  await expect(popover.getByTestId("git-operation-detail")).toContainText(
    "op:",
  );
  await expect(popover.getByTestId("git-operation-output")).toContainText(
    "record operation in status bar",
  );

  // Reload page -> operation history persists from server statefile git-operations.json
  await page.reload();
  await expect(
    page.getByTestId("git-operations-popover-trigger"),
  ).toContainText("提交 · 已完成");

  // 2. Simulate network connection drop AFTER backend executes POST /git/commit:
  // Frontend MUST recover via GET /git/operations using clientRequestId and NEVER commit twice!
  const commitsBefore = Number(
    git(mainPath, ["rev-list", "--count", "HEAD"]).trim(),
  );
  await writeFile(
    join(mainPath, "reconnect.ts"),
    "export const reconnectSafe = true;\n",
  );
  await page.getByRole("button", { name: "刷新暂存区快照" }).click();
  await untrackedSec
    .getByRole("button", { name: "暂存文件 reconnect.ts" })
    .click();
  await page
    .getByRole("textbox", { name: "提交说明标题" })
    .fill("feat(git): idempotent recovery after dropped response");
  await page.getByRole("button", { name: "提交预览…" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();

  let commitPostCount = 0;
  await page.route("**/git/commit", async (route) => {
    commitPostCount += 1;
    // Forward the real request to the backend so the commit is actually recorded on disk + ledger,
    // then drop the browser connection before returning the response!
    const realRes = await route.fetch();
    expect(realRes.status()).toBe(200);
    await route.abort("connectionreset");
  });

  await page
    .getByRole("dialog")
    .getByRole("button", { name: "确认本地提交" })
    .click();

  // Dialog closes automatically once GET /git/operations reconciles the clientRequestId!
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(commitPostCount).toBe(1);

  const commitsAfter = Number(
    git(mainPath, ["rev-list", "--count", "HEAD"]).trim(),
  );
  expect(commitsAfter).toBe(commitsBefore + 1);
  const headShort = git(mainPath, ["rev-parse", "--short=8", "HEAD"]).trim();
  expect(git(mainPath, ["log", "-1", "--pretty=%s"]).trim()).toBe(
    "feat(git): idempotent recovery after dropped response",
  );
  await expect(page.getByTestId("git-status-summary")).toContainText(
    `已创建提交 ${headShort}`,
  );
  await page.getByTestId("git-operations-popover-trigger").click();
  await expect(
    page
      .getByTestId("git-operations-popover")
      .getByTestId("git-operation-output"),
  ).toContainText("idempotent recovery after dropped response");
});
