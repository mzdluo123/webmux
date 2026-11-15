import { execFileSync } from "node:child_process";
import { mkdir, readFile, writeFile } from "node:fs/promises";
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

async function assertNoHorizontalOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    ),
  ).toBe(false);
}

async function openPushFromTopBar(page: Page) {
  const topbar = page.getByTestId("git-workbench-topbar");
  const viewportWidth = page.viewportSize()?.width ?? 1440;
  if (viewportWidth >= 768) {
    await topbar.getByRole("button", { name: /^推送/ }).click();
  } else {
    await topbar.getByRole("button", { name: "更多同步操作" }).click();
    await page.getByRole("menuitem", { name: /^推送/ }).click();
  }
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
    join(mainPath, "config.ts"),
    [
      "// Host section",
      "export const host = 'localhost';",
      "",
      "// Port section",
      "export const port = 7681;",
      "",
      "// Debug section",
      "export const debug = false;",
      "",
    ].join("\n"),
  );
  await writeFile(join(mainPath, "notes.md"), "# Seed Notes\n");
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

test("recovers push-rejected diverged branch via Rebase preview -> 3-way GitConflictView (Draft -> Save Result -> Mark as Resolved -> Continue), re-entry banner (A13), and subsequent clean Push (UI-05, UI-06, UI-07, UI-12)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath, remotePath } = await createRepoWithBareRemote(
    request,
    workDir,
    "v2-rebase-conflict",
  );

  // 1. Local commit on main modifying host (non-conflicting) + port (conflicting)
  await writeFile(
    join(mainPath, "config.ts"),
    [
      "// Host section",
      "export const host = '127.0.0.1';",
      "",
      "// Port section",
      "export const port = 8080;",
      "",
      "// Debug section",
      "export const debug = false;",
      "",
    ].join("\n"),
  );
  git(mainPath, ["add", "config.ts"]);
  git(mainPath, ["commit", "-m", "feat(local): bind loopback 8080"]);

  // 2. Advance origin/main from another clone modifying port (conflicting) + debug (non-conflicting)
  const peerPath = join(workDir, "v2-rebase-peer");
  git(workDir, ["clone", remotePath, peerPath]);
  git(peerPath, ["config", "user.name", "Bob Peer"]);
  git(peerPath, ["config", "user.email", "bob@example.invalid"]);
  await writeFile(
    join(peerPath, "config.ts"),
    [
      "// Host section",
      "export const host = 'localhost';",
      "",
      "// Port section",
      "export const port = 9000;",
      "",
      "// Debug section",
      "export const debug = true;",
      "",
    ].join("\n"),
  );
  git(peerPath, ["add", "config.ts"]);
  git(peerPath, ["commit", "-m", "feat(remote): switch production port 9000"]);
  git(peerPath, ["push", "origin", "main"]);

  const viewportWidth = page.viewportSize()?.width ?? 1440;
  const isDesktop = viewportWidth >= 1280;

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=commit`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  // 3. Open Push dialog -> Push fails non-fast-forward -> Fetch -> Rebase onto origin/main!
  await openPushFromTopBar(page);
  const pushDialog = page.getByRole("dialog");
  await expect(pushDialog).toBeVisible();
  await pushDialog.getByRole("button", { name: "确认推送" }).click();
  await expect(pushDialog).toContainText("推送未完成");

  // Fetch remote inside Push Rejected banner so origin/main tracking ref is fresh, then click Rebase!
  await pushDialog.getByRole("button", { name: "获取远程 (Fetch)" }).click();
  await expect(page.getByTestId("git-push-rejected-rebase-btn")).toBeVisible();
  await page.getByTestId("git-push-rejected-rebase-btn").click();

  // 4. Explicit Direction & Blast Radius Preview Dialog (UI-05)
  const previewDialog = page.getByTestId("git-divergence-preview-dialog");
  await expect(previewDialog).toBeVisible();
  await expect(previewDialog).toContainText("origin/main");
  await assertNoHorizontalOverflow(page);
  await page.getByTestId("git-divergence-confirm-btn").click();

  // 5. Rebase pauses on conflict -> GitConflictView opens automatically + In-Progress Banner mounted
  await expect(page.getByTestId("git-in-progress-banner")).toBeVisible();
  await expect(page.getByTestId("git-conflict-view")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  // 6. Reload browser page to verify stateless coordinator re-entry recovery (A13)!
  await page.reload();
  const banner = page.getByTestId("git-in-progress-banner");
  await expect(banner).toBeVisible();
  await expect(banner).toContainText("变基 (Rebase)");
  await expect(page.getByTestId("git-continue-operation-btn")).toBeDisabled();
  await page.getByTestId("git-open-conflict-resolver-btn").click();
  await expect(page.getByTestId("git-conflict-view")).toBeVisible();

  const saveBtn = page.getByTestId("git-conflict-save-btn");
  const resolveBtn = page.getByTestId("git-conflict-resolve-btn");
  const continueBtn = page.getByTestId("git-conflict-continue-btn");

  await expect(resolveBtn).toBeDisabled();
  await expect(continueBtn).toBeDisabled();

  // 7. Stage 1 Draft: Apply all non-conflicting changes + Accept Left on conflict chunk
  await page.getByTestId("git-conflict-apply-nonconflicting-btn").click();
  if (isDesktop) {
    await expect(page.getByTestId("git-conflict-left-column")).toContainText(
      "当前分支重放提交",
    );
    await expect(page.getByTestId("git-conflict-right-column")).toContainText(
      "变基目标基底",
    );
    // Toggle Base comparison and confirm Center Result stays mounted
    await page.getByRole("radio", { name: "左侧对照 Base" }).click();
    await expect(
      page.getByTestId("git-conflict-result-textarea"),
    ).toBeVisible();
    await page
      .getByTestId("git-conflict-left-column")
      .getByRole("button", { name: "接受左侧" })
      .click();
  } else {
    // Mobile 320px: switch source viewer to Common Ancestor Base while Result editor stays visible
    await page.getByRole("radio", { name: "共同祖先 Base" }).click();
    await expect(
      page.getByTestId("git-conflict-mobile-textarea"),
    ).toBeVisible();
    await page
      .getByTestId("git-conflict-mobile-pane")
      .getByRole("button", { name: "接受左侧" })
      .click();
    await assertNoHorizontalOverflow(page);
  }

  // Mark as Resolved MUST still be disabled before Save Result!
  await expect(resolveBtn).toBeDisabled();
  await expect(continueBtn).toBeDisabled();

  // 8. Stage 2 Save Result: writes merged file to worktree WITHOUT staging (stages 1..3 remain!)
  await saveBtn.click();
  await expect(resolveBtn).toBeEnabled();
  await expect(continueBtn).toBeDisabled();
  expect(git(mainPath, ["ls-files", "-u"]).trim()).not.toBe("");
  expect(await readFile(join(mainPath, "config.ts"), "utf8")).toBe(
    [
      "// Host section",
      "export const host = '127.0.0.1';",
      "",
      "// Port section",
      "export const port = 8080;",
      "",
      "// Debug section",
      "export const debug = true;",
      "",
    ].join("\n"),
  );

  // 9. Stage 3 Mark as Resolved: clears unmerged stages 1..3 -> Continue becomes enabled!
  await resolveBtn.click();
  await expect(page.getByTestId("git-conflict-all-resolved")).toBeVisible();
  expect(git(mainPath, ["ls-files", "-u"]).trim()).toBe("");
  await expect(continueBtn).toBeEnabled();

  // 10. Stage 4 Continue: completes rebase cleanly!
  await continueBtn.click();
  await expect(page.getByTestId("git-in-progress-banner")).toHaveCount(0);
  expect(git(mainPath, ["status", "--porcelain"]).trim()).toBe("");
  expect(git(mainPath, ["log", "-1", "--pretty=%s"]).trim()).toBe(
    "feat(local): bind loopback 8080",
  );

  // 11. Push rebased branch cleanly to bare origin without --force!
  await openPushFromTopBar(page);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "确认推送" })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(git(remotePath, ["rev-parse", "refs/heads/main"]).trim()).toBe(
    git(mainPath, ["rev-parse", "HEAD"]).trim(),
  );
});

test("cherry-picks commit from Log detail into binary conflict, inspects honest Abort preview with ORIG_HEAD, and aborts cleanly back to pre-operation HEAD (G10, A13)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath } = await createRepoWithBareRemote(
    request,
    workDir,
    "v2-cherrypick-abort",
  );

  // Seed binary logo.bin on main
  await writeFile(
    join(mainPath, "logo.bin"),
    Buffer.from([0x00, 0x89, 0x50, 0x4e, 0x47, 0x01]),
  );
  git(mainPath, ["add", "logo.bin"]);
  git(mainPath, ["commit", "-m", "feat: add binary logo"]);

  // Create side branch modifying logo.bin
  git(mainPath, ["checkout", "-b", "feat/new-logo"]);
  await writeFile(
    join(mainPath, "logo.bin"),
    Buffer.from([0x00, 0x89, 0x50, 0x4e, 0x47, 0x99, 0xfe]),
  );
  git(mainPath, ["add", "logo.bin"]);
  git(mainPath, ["commit", "-m", "feat(logo): dark mode binary"]);
  const cherryOid = git(mainPath, ["rev-parse", "HEAD"]).trim();

  // Diverge logo.bin on main so cherry-pick triggers a binary conflict!
  git(mainPath, ["checkout", "main"]);
  await writeFile(
    join(mainPath, "logo.bin"),
    Buffer.from([0x00, 0x89, 0x50, 0x4e, 0x47, 0x42, 0xaa]),
  );
  git(mainPath, ["add", "logo.bin"]);
  git(mainPath, ["commit", "-m", "feat(main): compress binary logo"]);
  const preOpHeadOid = git(mainPath, ["rev-parse", "HEAD"]).trim();

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=log`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();

  // Switch scope to All History so feat/new-logo commit appears, select it, and click Cherry-pick!
  await page.getByRole("radio", { name: "所有历史" }).click();
  await page
    .getByRole("option", { name: /feat\(logo\): dark mode binary/ })
    .click();
  await page.getByTestId("git-log-detail-cherry-pick-btn").click();

  const previewDialog = page.getByTestId("git-divergence-preview-dialog");
  await expect(previewDialog).toBeVisible();
  await expect(previewDialog).toContainText(cherryOid.slice(0, 7));
  await page.getByTestId("git-divergence-confirm-btn").click();

  // Binary conflict variant card opens!
  await expect(page.getByTestId("git-conflict-variant-card")).toBeVisible();
  await expect(page.getByTestId("git-conflict-choose-left-btn")).toBeVisible();
  await assertNoHorizontalOverflow(page);

  // Click Honest Abort in the In-Progress Banner -> verify pre-operation restoreOid & affected path!
  await page.getByTestId("git-abort-operation-btn").click();
  const abortDialog = page.getByTestId("git-abort-operation-dialog");
  await expect(abortDialog).toBeVisible();
  await expect(abortDialog).toContainText(preOpHeadOid.slice(0, 7));
  await expect(abortDialog).toContainText("logo.bin");
  await assertNoHorizontalOverflow(page);

  await page.getByTestId("git-abort-confirm-btn").click();
  await expect(page.getByTestId("git-in-progress-banner")).toHaveCount(0);
  expect(git(mainPath, ["rev-parse", "HEAD"]).trim()).toBe(preOpHeadOid);
  expect(git(mainPath, ["status", "--porcelain"]).trim()).toBe("");
});
