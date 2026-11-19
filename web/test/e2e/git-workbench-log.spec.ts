import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, readFile, stat, utimes, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext } from "@playwright/test";
import { expect, test } from "./fixtures";
import { pdfFixture } from "../fixtures/pdf";

function git(
  root: string,
  args: string[],
  envOverrides: Record<string, string> = {},
): string {
  return execFileSync("git", ["-C", root, ...args], {
    env: {
      ...process.env,
      GIT_CONFIG_GLOBAL: "/dev/null",
      GIT_CONFIG_NOSYSTEM: "1",
      GIT_AUTHOR_NAME: "Alice Author",
      GIT_AUTHOR_EMAIL: "alice@example.invalid",
      GIT_COMMITTER_NAME: "Alice Author",
      GIT_COMMITTER_EMAIL: "alice@example.invalid",
      ...envOverrides,
    },
  }).toString();
}

async function readIndexFingerprint(repoPath: string) {
  const indexPath = join(repoPath, ".git", "index");
  const [buf, st] = await Promise.all([readFile(indexPath), stat(indexPath)]);
  return {
    sha256: createHash("sha256").update(buf).digest("hex"),
    mtimeMs: st.mtimeMs,
  };
}

async function setupRichGitLogRepository(
  request: APIRequestContext,
  workDir: string,
) {
  const remotePath = join(workDir, "origin.git");
  const mainPath = join(workDir, "webmux-log");
  const linkedPath = join(workDir, "webmux-log-wt-occupied");

  await mkdir(remotePath, { recursive: true });
  git(remotePath, ["init", "--bare", "-b", "main"]);

  await mkdir(mainPath, { recursive: true });
  git(mainPath, ["init", "-b", "main"]);
  git(mainPath, ["remote", "add", "origin", remotePath]);

  // Seed initial commit + annotated tag v0.9.0
  await writeFile(
    join(mainPath, "src.ts"),
    "export const version = '0.9.0';\nexport function greet() {\n  return 'hello';\n}\n",
  );
  await writeFile(join(mainPath, "legacy.txt"), "legacy content line 1\n");
  await writeFile(
    join(mainPath, "logo.bin"),
    Buffer.from([0x00, 0x89, 0x50, 0x4e, 0x47, 0x00, 0xff]),
  );
  git(mainPath, ["add", "."]);
  git(mainPath, ["commit", "-m", "feat: initial seed for log workbench"]);
  const baseOid = git(mainPath, ["rev-parse", "HEAD"]).trim();
  git(mainPath, ["tag", "-a", "v0.9.0", "-m", "Release 0.9.0"]);

  // Add 105 linear history commits on history-bulk.txt so defaultHistoryLimit (100) yields nextCursor!
  for (let i = 1; i <= 105; i++) {
    await writeFile(join(mainPath, "history-bulk.txt"), `commit step ${i}\n`);
    git(mainPath, ["add", "history-bulk.txt"]);
    git(mainPath, [
      "commit",
      "-m",
      `chore(history): bulk step ${String(i).padStart(3, "0")}`,
    ]);
  }
  git(mainPath, ["push", "-u", "origin", "main"]);

  // Create unoccupied local branch feat/ready-switch for SwitchBranch checkout preview verification
  git(mainPath, ["checkout", "-b", "feat/ready-switch"]);
  await writeFile(
    join(mainPath, "switchable.ts"),
    "export const readyToCheckout = true;\n",
  );
  git(mainPath, ["add", "switchable.ts"]);
  git(mainPath, ["commit", "-m", "feat(switch): branch ready for checkout"]);
  const switchBranchOid = git(mainPath, ["rev-parse", "HEAD"]).trim();

  // Create feature branch feat/graph-lane that modifies feature-lane.ts & renames legacy.txt -> docs/guide.txt
  git(mainPath, ["checkout", "main"]);
  git(mainPath, ["checkout", "-b", "feat/graph-lane"]);
  await mkdir(join(mainPath, "docs"), { recursive: true });
  git(mainPath, ["mv", "legacy.txt", "docs/guide.txt"]);
  await writeFile(
    join(mainPath, "feature-lane.ts"),
    "export const laneFeature = 'from-second-parent';\n",
  );
  git(mainPath, ["add", "."]);
  git(
    mainPath,
    ["commit", "-m", "feat(graph): add feature lane and rename guide"],
    {
      GIT_AUTHOR_NAME: "Carol Contributor",
      GIT_AUTHOR_EMAIL: "carol@example.invalid",
    },
  );

  // Advance main on a different file (src.ts), then merge feat/graph-lane with distinct committer!
  git(mainPath, ["checkout", "main"]);
  await writeFile(
    join(mainPath, "src.ts"),
    "export const version = '1.0.0';\nexport function greet() {\n  return 'hello from main';\n}\n",
  );
  git(mainPath, ["add", "src.ts"]);
  git(mainPath, [
    "commit",
    "-m",
    "feat(core): update src on main before merge",
  ]);

  git(
    mainPath,
    [
      "merge",
      "--no-ff",
      "feat/graph-lane",
      "-m",
      "Merge branch 'feat/graph-lane' into main\n\nDetailed merge body line 1.\nReviewed-by: Release Bot",
    ],
    {
      GIT_COMMITTER_NAME: "Bob Merger",
      GIT_COMMITTER_EMAIL: "bob@example.invalid",
    },
  );
  git(mainPath, ["tag", "v1.0.0"]);
  const headOid = git(mainPath, ["rev-parse", "HEAD"]).trim();

  // Add an occupied linked worktree on feat/occupied
  git(mainPath, [
    "worktree",
    "add",
    "-b",
    "feat/occupied",
    linkedPath,
    "HEAD~1",
  ]);

  // Advance bare remote by cloning a temporary pusher so origin/main has 1 incoming commit AND main has ahead commits!
  const pusherPath = join(workDir, "pusher");
  git(workDir, ["clone", remotePath, pusherPath]);
  await writeFile(join(pusherPath, "remote-note.md"), "incoming from origin\n");
  git(pusherPath, ["add", "remote-note.md"]);
  git(pusherPath, ["commit", "-m", "docs(remote): incoming upstream note"]);
  git(pusherPath, ["push", "origin", "main"]);
  git(mainPath, ["fetch", "origin"]);

  // Settle Git's 1-second racy-git stat cache before registering the repository
  // so .git/index mtime + sha256 remain strictly constant across the entire test.
  const past = new Date(Date.now() - 10_000);
  for (const rel of [
    "src.ts",
    "logo.bin",
    "history-bulk.txt",
    "feature-lane.ts",
    "docs/guide.txt",
  ]) {
    await utimes(join(mainPath, rel), past, past);
  }
  git(mainPath, ["status"]);

  const res = await request.post("/api/repositories", {
    data: { path: mainPath },
  });
  expect(res.status(), await res.text()).toBe(201);
  const repo = await res.json();
  const mainWt = repo.worktrees.find(
    (w: { kind: string }) => w.kind === "main",
  );
  expect(mainWt).toBeTruthy();

  return {
    repo,
    mainWt,
    mainPath,
    baseOid,
    switchBranchOid,
    headOid,
  };
}

test("Log view filters refs/tags without checkout (U02), paginates %P topology & merge parents (U03), compares two commits with Pierre Diff, and never mutates .git/index (A14)", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, mainPath, baseOid, switchBranchOid, headOid } =
    await setupRichGitLogRepository(request, workDir);

  const isPhone = (page.viewportSize()?.width ?? 1440) < 640;

  const indexBefore = await readIndexFingerprint(mainPath);
  const stagedBefore = git(mainPath, ["diff", "--cached", "--name-status"]);
  const headSymBefore = git(mainPath, ["symbolic-ref", "HEAD"]).trim();
  const headRevBefore = git(mainPath, ["rev-parse", "HEAD"]).trim();
  expect(headSymBefore).toBe("refs/heads/main");
  expect(headRevBefore).toBe(headOid);

  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=log`);
  const topbar = page.getByTestId("git-workbench-topbar");
  await expect(topbar).toBeVisible();
  await expect(topbar).toContainText("main");

  if (isPhone) {
    // UI-12 Phone (320×568): verify no horizontal overflow, ref drawer filtering, and 3-step list -> detail -> Unified Pierre Diff -> back.
    const checkNoHorizontalOverflow = async () => {
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > window.innerWidth,
      );
      expect(overflow).toBe(false);
    };
    await checkNoHorizontalOverflow();

    // Open ref drawer and filter by tag v0.9.0 without changing checked-out HEAD!
    await page.getByRole("button", { name: "打开引用筛选抽屉" }).click();
    const drawerTree = page.getByRole("complementary", {
      name: "历史引用筛选树",
    });
    await expect(drawerTree).toBeVisible();
    await drawerTree
      .locator('[data-ref-section="tags"]')
      .getByRole("button", { name: /^v0\.9\.0(?:\s|$)/ })
      .click();

    await expect(
      page.getByRole("option", { name: /initial seed for log workbench/ }),
    ).toBeVisible();
    expect(git(mainPath, ["symbolic-ref", "HEAD"]).trim()).toBe(
      "refs/heads/main",
    );
    await page.getByRole("button", { name: "重置全部筛选" }).click();

    // Step 1 -> Step 2: tap merge commit row -> Commit Detail & Changed Files
    await page
      .getByRole("option", {
        name: /Merge branch 'feat\/graph-lane' into main/,
      })
      .click();
    await expect(
      page.getByRole("button", { name: "返回提交列表" }),
    ).toBeVisible();
    await checkNoHorizontalOverflow();

    // Step 2 -> Step 3: tap changed file -> Unified Pierre Diff
    await page
      .getByRole("listbox", { name: "提交变更文件" })
      .getByRole("option", { name: /feature-lane\.ts/ })
      .click();
    const diffPanel = page.getByTestId("git-log-diff-panel");
    await expect(diffPanel).toHaveAttribute("data-diff-presentation", "mobile");
    await expect(diffPanel.locator(".code-diff-reader")).toContainText(
      "from-second-parent",
    );
    await checkNoHorizontalOverflow();

    // Step back to detail -> list
    await diffPanel.getByRole("button", { name: "返回提交详情" }).click();
    await page.getByRole("button", { name: "返回提交列表" }).click();
    await expect(page.getByRole("listbox", { name: "提交记录" })).toBeVisible();
  } else {
    // Desktop (1440×900): full multi-column Log verification!
    const tree = page.getByRole("complementary", { name: "历史引用筛选树" });
    await expect(tree).toBeVisible();

    // 1. Merge commit details: distinct committer (Bob Merger), body, merge parent switcher (#1 vs #2), and Pierre Diff!
    const detailPane = page.getByRole("region", { name: "提交详情元数据" });
    const filesListbox = page.getByRole("listbox", { name: "提交变更文件" });
    const diffPanel = page.getByTestId("git-log-diff-panel");

    await expect(detailPane).toContainText("Alice Author");
    await expect(detailPane).toContainText("Bob Merger");
    await expect(detailPane).toContainText("Detailed merge body line 1.");

    // Against Parent #1 (main before merge), feature-lane.ts & renamed legacy.txt -> docs/guide.txt changed:
    await expect(filesListbox.getByText("feature-lane.ts")).toBeVisible();
    await expect(
      filesListbox.getByText("legacy.txt → docs/guide.txt"),
    ).toBeVisible();

    await filesListbox
      .getByRole("option", { name: /feature-lane\.ts/ })
      .click();
    await expect(diffPanel.locator(".code-diff-reader")).toContainText(
      "from-second-parent",
    );

    // Switch merge parent to Parent #2 (feat/graph-lane): changed file becomes src.ts!
    await detailPane.getByRole("radio", { name: /父节点 #2/ }).click();
    await expect(filesListbox.getByText("src.ts")).toBeVisible();
    await expect(diffPanel.locator(".code-diff-reader")).toContainText(
      "hello from main",
    );

    // Switch back to Parent #1
    await detailPane.getByRole("radio", { name: /父节点 #1/ }).click();
    await expect(filesListbox.getByText("feature-lane.ts")).toBeVisible();

    // 2. U02 Hard Red Line: Single-clicking branch & tag in left tree ONLY filters Log; disk HEAD & topbar never change!
    await tree
      .locator('[data-ref-section="local"]')
      .getByRole("button", { name: /^feat\/ready-switch/ })
      .click();
    await expect(
      page.getByRole("option", { name: /branch ready for checkout/ }),
    ).toBeVisible();
    // Previous selection (merge commit) is not on feat/ready-switch -> selectionNotice with "定位当前 HEAD" appears!
    await expect(
      page.getByRole("button", { name: "定位当前 HEAD" }),
    ).toBeVisible();
    await expect(topbar).toContainText("main");
    expect(git(mainPath, ["symbolic-ref", "HEAD"]).trim()).toBe(
      "refs/heads/main",
    );
    expect(git(mainPath, ["rev-parse", "HEAD"]).trim()).toBe(headOid);

    // Filter by annotated tag v0.9.0
    await tree
      .locator('[data-ref-section="tags"]')
      .getByRole("button", { name: /^v0\.9\.0(?:\s|$)/ })
      .click();
    await expect(
      page.getByRole("option", { name: /initial seed for log workbench/ }),
    ).toBeVisible();
    // Verify binary file badge + non-crashing binary notice in Diff panel on initial commit
    await filesListbox.getByRole("option", { name: /logo\.bin/ }).click();
    await expect(filesListbox.getByText("二进制")).toBeVisible();
    await expect(diffPanel).toContainText(
      /二进制|非文本或受限内容不能显示行差异/,
    );
    await expect(diffPanel).toContainText("下载变更后文件");

    // Click "定位当前 HEAD" -> resets filter & returns to HEAD merge commit!
    await page.getByRole("button", { name: "定位当前 HEAD" }).click();
    await expect(
      page.getByRole("option", {
        name: /Merge branch 'feat\/graph-lane' into main/,
      }),
    ).toHaveAttribute("aria-selected", "true");

    // Verify explicit "检出分支…" from row "…" menu opens SwitchBranch preview dialog pre-selecting feat/ready-switch (cancel without switching)
    await tree
      .locator('[data-ref-section="local"]')
      .getByRole("button", { name: "引用操作 feat/ready-switch" })
      .click();
    await page.getByRole("menuitem", { name: "检出分支…" }).click();
    const switchDialog = page.locator('[role="dialog"][data-state="open"]');
    await expect(switchDialog).toContainText("feat/ready-switch");
    await page.keyboard.press("Escape");
    await expect(switchDialog).toHaveCount(0);

    // 3. U03 Path filter indirect dashed edge + cursor pagination ("加载更多历史")
    const pathInput = page.getByRole("textbox", { name: "按路径筛选提交" });
    await pathInput.fill("src.ts");
    await expect(page).toHaveURL(/path=src\.ts/);
    await expect(page.getByText("筛选省略中间节点").first()).toBeVisible();
    await page.getByRole("button", { name: "重置全部筛选" }).click();
    await expect(pathInput).toHaveValue("");

    // Cursor pagination: > 100 commits yields "加载更多历史"; clicking appends initial seed commit!
    const loadMoreBtn = page.getByRole("button", { name: "加载更多历史" });
    await expect(loadMoreBtn).toBeVisible();
    await loadMoreBtn.click();
    await expect(
      page.getByRole("option", { name: /initial seed for log workbench/ }),
    ).toBeVisible();

    // Scroll commit list down, preview src.ts via Enter in docked split diff -> list scrollTop is preserved and removed toolbar buttons are absent!
    const commitListbox = page.getByRole("listbox", { name: "提交记录" });
    await commitListbox.evaluate((el) => {
      el.scrollTop = 360;
    });
    const scrollBeforePreview = await commitListbox.evaluate(
      (el) => el.scrollTop,
    );
    expect(scrollBeforePreview).toBeGreaterThan(200);

    await filesListbox.getByRole("option", { name: /src\.ts/ }).click();
    await page.keyboard.press("Enter");
    await expect(diffPanel).toHaveAttribute("data-diff-presentation", "docked");
    await expect(
      diffPanel.getByRole("button", { name: "变更前全文" }),
    ).toHaveCount(0);
    await expect(
      diffPanel.getByRole("button", { name: "变更后全文" }),
    ).toHaveCount(0);
    await expect(
      diffPanel.getByRole("radiogroup", { name: "历史差异布局模式" }),
    ).toHaveCount(0);
    await expect(
      diffPanel.getByRole("button", { name: "放大差异到主工作区" }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "放大查看当前文件差异" }),
    ).toHaveCount(0);
    await expect(diffPanel.getByText("修改前")).toBeVisible();
    await expect(diffPanel.getByText("修改后")).toBeVisible();
    await expect
      .poll(() => commitListbox.evaluate((el) => el.scrollTop))
      .toBeGreaterThan(200);

    // 4. Two-commit arbitrary compare: baseOid -> headOid, swap direction, and exit compare
    await commitListbox.evaluate((el) => {
      el.scrollTop = 0;
    });
    const switchShort7 = switchBranchOid.slice(0, 7);
    const headShort7 = headOid.slice(0, 7);
    await page
      .getByRole("button", { name: new RegExp(`^提交操作 ${switchShort7}`) })
      .click();
    await page.getByRole("menuitem", { name: /设为比较起点/ }).click();
    await expect(
      page.getByText(new RegExp(`已设比较起点.*${switchShort7}`)),
    ).toBeVisible();

    await page
      .getByRole("button", { name: new RegExp(`^提交操作 ${headShort7}`) })
      .click();
    await page
      .getByRole("menuitem", {
        name: new RegExp(`与起点 \\(${switchShort7}\\) 比较`),
      })
      .click();

    const filesRegion = page.getByRole("region", { name: "变更文件列表" });
    await expect(filesRegion.getByText("两版本比较")).toBeVisible();
    await expect(
      filesRegion.getByText(`${switchShort7} → ${headShort7}`),
    ).toBeVisible();
    await expect(filesListbox.getByText("switchable.ts")).toBeVisible();

    // Swap compare direction: headShort7 -> switchShort7
    await page.getByRole("button", { name: "交换比较方向" }).click();
    await expect(
      filesRegion.getByText(`${headShort7} → ${switchShort7}`),
    ).toBeVisible();
    await filesListbox.getByRole("option", { name: /switchable\.ts/ }).click();
    await expect(diffPanel.locator(".code-diff-reader")).toContainText(
      "readyToCheckout",
    );

    await page.getByRole("button", { name: "退出两版本比较" }).click();
    await expect(filesRegion.getByText("两版本比较")).toHaveCount(0);

    // Quick scope checks: 待推送 & 远端新增 render real commits, and repo-only /git/{repoId} surfaces no_worktree notice!
    await page
      .getByRole("radiogroup", { name: "提交范围" })
      .getByRole("radio", { name: "远端新增" })
      .click();
    await expect(
      page.getByRole("option", { name: /incoming upstream note/ }),
    ).toBeVisible();

    await page.goto(`/git/${repo.id}?view=log&scope=to-push`);
    await expect(page.getByRole("listbox", { name: "提交记录" })).toContainText(
      /未指定目标分支或可用工作树|未绑定可用工作树/,
    );
  }

  // A14 Hard Red Line: .git/index sha256 + mtime, staged entries, and checked-out HEAD are 100% untouched!
  const indexAfter = await readIndexFingerprint(mainPath);
  expect(indexAfter.sha256).toBe(indexBefore.sha256);
  expect(indexAfter.mtimeMs).toBe(indexBefore.mtimeMs);
  expect(git(mainPath, ["diff", "--cached", "--name-status"])).toBe(
    stagedBefore,
  );
  expect(git(mainPath, ["symbolic-ref", "HEAD"]).trim()).toBe(
    "refs/heads/main",
  );
  expect(git(mainPath, ["rev-parse", "HEAD"]).trim()).toBe(headOid);
  void baseOid;
});

test("Docked log diff minimize and phone back preserve the reader and selected file", async ({
  page,
  request,
  workDir,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const { repo, mainWt, headOid } = await setupRichGitLogRepository(
    request,
    workDir,
  );
  const route = `/git/${repo.id}/worktrees/${mainWt.id}?view=log&commit=${headOid}`;
  await page.goto(route);
  const files = page.getByRole("listbox", { name: "提交变更文件" });
  const selectedFile = files.getByRole("option", { name: /feature-lane\.ts/ });
  await selectedFile.click();
  const diff = page.getByTestId("git-log-diff-panel");
  const diffDock = page.locator('[data-dock-panel="diff"]');

  for (const codeReader of ["文件内查找", "缩放"]) {
    await expect(diff).toHaveAttribute("data-diff-presentation", "docked");
    const panelToggle = diff.getByRole("button", {
      name: codeReader,
      exact: true,
    });
    await panelToggle.click();
    await expect(panelToggle).toHaveAttribute("aria-expanded", "true");

    await diffDock
      .getByRole("button", { name: "收起「文件差异预览」" })
      .click();
    await expect(diff).toBeHidden();
    await page
      .getByRole("button", { name: "文件差异预览", exact: true })
      .click();
    await expect(panelToggle).toHaveAttribute("aria-expanded", "true");
    await expect(diff).toHaveAttribute("data-diff-presentation", "docked");
    await expect(page.getByRole("listbox", { name: "提交记录" })).toBeVisible();
    await expect(selectedFile).toHaveAttribute("aria-selected", "true");
    await expect(diff.locator(".code-diff-reader")).toContainText(
      "from-second-parent",
    );
    await expect(page).toHaveURL(route);
  }

  await page.setViewportSize({ width: 390, height: 844 });
  const selectedCommit = page.getByRole("option", {
    name: /Merge branch 'feat\/graph-lane' into main/,
  });
  await selectedCommit.click();
  for (const codeReader of ["文件内查找", "缩放"]) {
    await selectedFile.click();
    await expect(diff).toHaveAttribute("data-diff-presentation", "mobile");
    const panelToggle = diff.getByRole("button", {
      name: codeReader,
      exact: true,
    });
    await panelToggle.click();
    await expect(panelToggle).toHaveAttribute("aria-expanded", "true");

    await diff.getByRole("button", { name: "返回提交详情" }).click();
    await expect(
      page.getByRole("button", { name: "返回提交列表" }),
    ).toBeVisible();
    await expect(diff).not.toBeVisible();
    await expect(selectedFile).toHaveAttribute("aria-selected", "true");
    await expect(page).toHaveURL(route);
  }
  await page.getByRole("button", { name: "返回提交列表" }).click();
  await expect(page.getByRole("listbox", { name: "提交记录" })).toBeVisible();
  await expect(selectedCommit).toHaveAttribute("aria-selected", "true");
  await expect(page).toHaveURL(route);
});

test("Git mode docks restore independently and the phone selector leaves the wide Log layout untouched", async ({
  page,
  request,
  workDir,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const { repo, mainWt } = await setupRichGitLogRepository(request, workDir);
  await page.goto(`/git/${repo.id}/worktrees/${mainWt.id}?view=log`);

  const modes = [
    { mode: "历史", id: "refs", title: "引用与分支筛选" },
    { mode: "提交", id: "composer", title: "提交信息" },
    { mode: "分支", id: "detail", title: "分支详情" },
    { mode: "贮藏", id: "detail", title: "贮藏详情" },
  ];
  const workspace = page.locator("[data-dock-workspace]:visible");
  for (const { mode, id, title } of modes) {
    await page.getByRole("tab", { name: mode, exact: true }).click();
    const panel = workspace.locator(`[data-dock-panel="${id}"]`);
    await expect(panel).toBeVisible();
    await panel.getByRole("button", { name: `收起「${title}」` }).click();
    await expect(panel).toBeHidden();
  }

  await page.reload();
  for (const { mode, id, title } of modes) {
    await page.getByRole("tab", { name: mode, exact: true }).click();
    await expect(workspace.locator(`[data-dock-panel="${id}"]`)).toBeHidden();
    await expect(
      workspace.getByRole("button", { name: title, exact: true }),
    ).toBeVisible();
  }

  await page.getByRole("tab", { name: "历史", exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  const refs = workspace.locator('[data-dock-panel="refs"]');
  await expect(refs).toBeHidden();
  const pickerTrigger = workspace.getByRole("button", {
    name: "打开面板",
    exact: true,
  });
  await expect(pickerTrigger).toHaveCount(1);
  await pickerTrigger.click();
  const picker = page.getByRole("dialog", { name: "打开面板" });
  await expect(picker.getByRole("button", { name: "提交信息" })).toHaveCount(0);
  await picker.getByRole("button", { name: "引用与分支筛选" }).click();
  await expect(refs).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 720 });
  await expect(refs).toBeHidden();
  await expect(
    workspace.getByRole("button", { name: "引用与分支筛选", exact: true }),
  ).toBeVisible();
});

test("Log previews the selected Git snapshot sides and retains binary downloads", async ({
  page,
  request,
  workDir,
}, testInfo) => {
  git(workDir, ["init", "-b", "main"]);
  const image = join(workDir, "asset.bmp");
  // Uncompressed 24-bit BMP with a distinct color and width in each version.
  function bitmap(width: number, color: number[]) {
    const height = 80;
    const data = Buffer.alloc(54 + width * height * 3);
    data.write("BM");
    data.writeUInt32LE(data.length, 2);
    data.writeUInt32LE(54, 10);
    data.writeUInt32LE(40, 14);
    data.writeUInt32LE(width, 18);
    data.writeUInt32LE(height, 22);
    data.writeUInt16LE(1, 26);
    data.writeUInt16LE(24, 28);
    data.fill(Buffer.from(color), 54);
    return data;
  }
  await writeFile(image, bitmap(160, [175, 120, 42]));
  git(workDir, ["add", "."]);
  git(workDir, ["commit", "-m", "original asset"]);
  await writeFile(image, bitmap(240, [90, 142, 28]));
  await writeFile(join(workDir, "manual.pdf"), pdfFixture());
  await writeFile(join(workDir, "opaque.bin"), Buffer.from([0, 255]));
  git(workDir, ["add", "."]);
  git(workDir, ["commit", "-m", "update assets"]);
  // The after preview must use the committed blob, not the live worktree.
  await writeFile(image, bitmap(320, [20, 32, 180]));

  const registration = await request.post("/api/repositories", {
    data: { path: workDir },
  });
  expect(registration.status(), await registration.text()).toBe(201);
  const repo = await registration.json();
  const worktree = repo.worktrees.find(
    (wt: { kind: string }) => wt.kind === "main",
  );
  const phone = (page.viewportSize()?.width ?? 1280) < 640;
  if (!phone) await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto(`/git/${repo.id}/worktrees/${worktree.id}?view=log`);
  await page.getByRole("option", { name: /update assets/ }).click();
  const files = page.getByRole("listbox", { name: "提交变更文件" });
  const diff = page.getByTestId("git-log-diff-panel");
  const open = (name: RegExp) => files.getByRole("option", { name }).click();
  const back = async () => {
    if (phone) await diff.getByRole("button", { name: "返回提交详情" }).click();
  };

  await open(/asset\.bmp/);
  const images = diff.getByRole("img", { name: "asset.bmp" });
  await expect(images).toHaveCount(2);
  await expect
    .poll(() =>
      images.evaluateAll((nodes) =>
        nodes.map((node) => (node as HTMLImageElement).naturalWidth),
      ),
    )
    .toEqual([160, 240]);
  const downloadBefore = diff.getByRole("button", { name: "下载变更前文件" });
  await downloadBefore.focus();
  await expect(downloadBefore).toBeFocused();
  await expect(
    diff.getByRole("button", { name: "下载变更后文件" }),
  ).toBeVisible();
  for (const viewport of phone
    ? [
        { width: 320, height: 568 },
        { width: 375, height: 667 },
        { width: 390, height: 844 },
      ]
    : [
        { width: 1280, height: 720 },
        { width: 768, height: 1024 },
      ]) {
    await page.setViewportSize(viewport);
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
      .toBeLessThanOrEqual(viewport.width);
    for (const colorScheme of ["light", "dark"] as const) {
      await page.emulateMedia({ colorScheme });
      await testInfo.attach(`binary-preview-${viewport.width}-${colorScheme}`, {
        body: await page.screenshot(),
        contentType: "image/png",
      });
    }
  }
  await back();

  await open(/manual\.pdf/);
  await expect(diff.getByRole("group", { name: "PDF 预览" })).toHaveCount(1);
  await expect(diff.locator(".textLayer")).toContainText("Page 1");
  await expect(
    diff.getByRole("button", { name: "下载变更前文件" }),
  ).toHaveCount(0);
  await back();

  await open(/opaque\.bin/);
  await expect(diff).toContainText(/二进制|非文本或受限内容不能显示行差异/);
  await expect(
    diff.getByRole("button", { name: "下载变更后文件" }),
  ).toBeVisible();
});
