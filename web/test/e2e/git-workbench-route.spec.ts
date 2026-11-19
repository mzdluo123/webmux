import { execFileSync } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { APIRequestContext, Page } from "@playwright/test";
import {
  expect,
  holdTerminalFont,
  revealSidebar,
  test,
  toggleCode,
  useWorkbenchBranches,
} from "./fixtures";
import {
  repositoryMenu,
  unfoldRepository,
  worktreeMenu,
} from "./repository-menu";

function git(root: string, ...args: string[]): string {
  return execFileSync("git", ["-C", root, ...args], {
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "WebMux QA",
      GIT_AUTHOR_EMAIL: "qa@example.invalid",
      GIT_COMMITTER_NAME: "WebMux QA",
      GIT_COMMITTER_EMAIL: "qa@example.invalid",
    },
  }).toString();
}

async function setupRepoWithLinkedWorktree(
  request: APIRequestContext,
  workDir: string,
  repoName = "webmux-git",
) {
  const mainPath = join(workDir, repoName);
  const linkedPath = join(workDir, `${repoName}-wt-feat`);
  await mkdir(mainPath, { recursive: true });

  git(mainPath, "init", "-b", "main");
  const longContent = Array.from(
    { length: 90 },
    (_, i) => `// line ${i + 1}: git workbench route test`,
  ).join("\n");
  await writeFile(
    join(mainPath, "README.md"),
    `# ${repoName}\n${longContent}\n`,
  );
  await writeFile(join(mainPath, "guide.txt"), "initial guide\n");
  git(mainPath, "add", ".");
  git(mainPath, "commit", "-m", "feat: initial repository seed");

  await writeFile(join(mainPath, "guide.txt"), "updated guide on main\n");
  git(mainPath, "add", "guide.txt");
  git(mainPath, "commit", "-m", "docs: update guide on main");

  git(mainPath, "worktree", "add", "-b", "feat/route", linkedPath, "HEAD");
  await writeFile(
    join(linkedPath, "feature.ts"),
    "export const routeReady = true;\n",
  );

  // Pin both worktrees on the home page / sidebar tree once registered.
  const response = await request.post("/api/repositories", {
    data: { path: mainPath },
  });
  expect(response.status(), await response.text()).toBe(201);
  const repo = await response.json();

  const mainWt = repo.worktrees.find(
    (w: { kind: string }) => w.kind === "main",
  );
  const featWt = repo.worktrees.find(
    (w: { branch: string | null }) => w.branch === "feat/route",
  );
  expect(mainWt).toBeTruthy();
  expect(featWt).toBeTruthy();

  const prefs = await (await request.get("/api/settings")).json();
  const patchRes = await request.patch("/api/settings", {
    data: {
      baseRevision: prefs.revision,
      changes: {
        homeWorktrees: { [repo.id]: [mainWt.id, featWt.id] },
      },
    },
  });
  expect(patchRes.ok()).toBeTruthy();

  return { repo, mainWt, featWt, mainPath, linkedPath };
}

async function selectGitMode(
  page: Page,
  mode: "历史" | "提交" | "分支",
): Promise<void> {
  await page
    .getByRole("tablist", { name: "Git 工作台视图" })
    .getByRole("tab", { name: new RegExp(`^${mode}`) })
    .click();
}

test("the Git and session views switch in both directions without losing the selected worktree", async ({
  page,
  request,
  workDir,
  isMobile,
}) => {
  const { repo, featWt, mainWt } = await setupRepoWithLinkedWorktree(
    request,
    workDir,
  );
  const longTitle =
    "Review branch output and compare the detailed terminal logs from the previous deployment";
  for (const [name, worktreeId] of [
    [longTitle, featWt.id],
    ["Watch the main branch", mainWt.id],
  ]) {
    expect(
      (
        await request.post("/api/sessions", {
          data: {
            name,
            workspace: { repositoryId: repo.id, worktreeId },
            cmd: ["/bin/sh", "-c", "sleep 120"],
          },
        })
      ).status(),
    ).toBe(201);
  }
  expect(
    (
      await request.post("/api/sessions", {
        data: {
          name: "Direct shell",
          cwd: workDir,
          cmd: ["/bin/sh", "-c", "sleep 120"],
        },
      })
    ).status(),
  ).toBe(201);
  await page.goto("/");
  await revealSidebar(page);
  const sidebar = page.getByRole("navigation", { name: "导航", exact: true });
  const project = sidebar.getByRole("list", { name: repo.name, exact: true });
  for (const [name, branch] of [
    [/^Review branch output/, "feat/route"],
    [/^Watch the main branch/, "main"],
  ] as const) {
    const session = project.getByRole("button", { name });
    const location = session.locator('[data-slot="sidebar-session-location"]');
    await expect(location).toHaveText(branch);
    await expect(location.locator('svg[aria-hidden="true"]')).toBeVisible();
    await expect(session).toHaveAccessibleDescription(branch);
    expect(await session.getAttribute("title")).toBeNull();
    await expect(location.locator("[title]")).toHaveCount(0);
  }
  if (!isMobile) {
    const short = project.getByRole("button", {
      name: /^Watch the main branch/,
    });
    await short.hover();
    expect(await short.getAttribute("title")).toBeNull();
    const long = project.getByRole("button", { name: /^Review branch output/ });
    await long.hover();
    await expect(long).toHaveAttribute("title", `${longTitle}\nfeat/route`);
  }
  const direct = sidebar
    .getByRole("list", { name: "直接会话", exact: true })
    .getByRole("button", { name: /^Direct shell/ });
  await expect(direct).toHaveAccessibleDescription(workDir);
  await expect(
    direct.locator('[data-slot="sidebar-session-location"] svg'),
  ).toHaveCount(0);
  await expect(
    sidebar.getByRole("button", { name: "仓库与工作树" }),
  ).toHaveCount(0);
  await useWorkbenchBranches(page);
  await unfoldRepository(page, repo.name);
  await page.getByRole("treeitem", { name: /^feat\/route/ }).click();
  const heading = page.getByRole("heading", { level: 1, name: /feat\/route/ });
  await expect(heading).toBeVisible();
  await page.getByRole("button", { name: "Git 工作台", exact: true }).click();
  await expect(page).toHaveURL(
    new RegExp(`/git/${repo.id}/worktrees/${featWt.id}`),
  );
  await page.getByRole("button", { name: "切换到会话列表" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(heading).toBeVisible();
  await page.getByRole("button", { name: "Git 工作台", exact: true }).click();
  await expect(page).toHaveURL(
    new RegExp(`/git/${repo.id}/worktrees/${featWt.id}`),
  );
});

test("standalone /git works with zero sessions, never spawns PTY, and survives last-session deletion", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, featWt } = await setupRepoWithLinkedWorktree(request, workDir);

  let sessionPostCount = 0;
  let terminalWsCount = 0;
  page.on("request", (req) => {
    if (
      req.method() === "POST" &&
      new URL(req.url()).pathname === "/api/sessions"
    ) {
      sessionPostCount++;
    }
  });
  page.on("websocket", (ws) => {
    if (new URL(ws.url()).pathname.startsWith("/ws/s/")) {
      terminalWsCount++;
    }
  });

  await page.goto("/");
  await expect(page.getByLabel("正在加载会话")).toHaveCount(0);

  await useWorkbenchBranches(page);
  // Open standalone /git/{repoId} from sidebar repository "⋯" menu with 0 sessions.
  await repositoryMenu(page, "Git 工作台", repo.name);
  await expect(page).toHaveURL(new RegExp(`/git/${repo.id}(\\?|$)`));
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  await expect(
    page.getByText("docs: update guide on main").first(),
  ).toBeVisible();

  // Browse Branches view in repo-only mode without any session.
  await selectGitMode(page, "分支");
  await expect(page).toHaveURL(new RegExp(`view=branches`));
  await expect(
    page
      .getByRole("region", { name: "分支与上游列表" })
      .getByRole("option", { name: /^feat\/route/ }),
  ).toBeVisible();

  // Repo-only Commit view MUST show explicit worktree guard and never silently pick a worktree.
  await selectGitMode(page, "提交");
  await expect(page.getByTestId("git-guard-pick-worktree")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "请选择要写入的工作树" }),
  ).toBeVisible();

  // Explicitly choose the dirty linked worktree -> navigates to /git/{repoId}/worktrees/{featWt.id}?view=commit.
  await page
    .getByTestId("git-guard-pick-worktree")
    .getByRole("button", { name: /feat\/route/ })
    .click();
  await expect(page).toHaveURL(
    new RegExp(`/git/${repo.id}/worktrees/${featWt.id}\\?.*view=commit`),
  );
  await expect(page.getByText("feature.ts").first()).toBeVisible();

  expect(sessionPostCount).toBe(0);
  expect(terminalWsCount).toBe(0);
  expect(await (await request.get("/api/sessions")).json()).toEqual([]);

  // Create one session and delete the last session while staying on /git: page must stay alive.
  const created = await request.post("/api/sessions", {
    data: {
      cmd: ["/bin/sh", "-c", "sleep 600"],
      name: "Transient session",
      workspace: { repositoryId: repo.id, worktreeId: featWt.id },
    },
  });
  expect(created.status()).toBe(201);
  const transientId = (await created.json()).session.id;

  expect((await request.delete(`/api/sessions/${transientId}`)).status()).toBe(
    204,
  );
  await expect
    .poll(
      async () =>
        (await request.get(`/api/sessions/${transientId}`)).status() === 404 ||
        (await (await request.get(`/api/sessions/${transientId}`)).json())
          .status === "exited",
    )
    .toBe(true);
  await request.delete(`/api/sessions/${transientId}?remove=1`);

  await selectGitMode(page, "历史");
  await expect(page).toHaveURL(
    new RegExp(`/git/${repo.id}/worktrees/${featWt.id}`),
  );
  await expect(
    page.getByText("docs: update guide on main").first(),
  ).toBeVisible();
});

test("worktree menu '提交改动…' deep-link and session Code Workspace round-trip preserve session state", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, featWt } = await setupRepoWithLinkedWorktree(
    request,
    workDir,
  );

  await page.goto("/");
  await expect(page.getByLabel("正在加载会话")).toHaveCount(0);

  await useWorkbenchBranches(page);
  // Dirty worktree row in sidebar offers "提交改动…" directly opening ?view=commit.
  await unfoldRepository(page, repo.name);
  await worktreeMenu(page, "提交改动…", "feat/route");
  await expect(page).toHaveURL(
    new RegExp(`/git/${repo.id}/worktrees/${featWt.id}\\?.*view=commit`),
  );
  await expect(page.getByText("feature.ts").first()).toBeVisible();

  // Start a hybrid chat session in mainWt so we can verify chat mode + unsent draft + CodeWorkspace tabs & scroll.
  const cfgRes = await request.post("/api/launch-configs", {
    data: { name: "E2E Hybrid", cmd: ["omp"], env: {}, chat: true },
  });
  expect(cfgRes.status()).toBe(201);
  const configId = (await cfgRes.json()).id;

  const sessRes = await request.post("/api/sessions", {
    data: {
      configId,
      view: "chat",
      name: "Agent coding session",
      workspace: { repositoryId: repo.id, worktreeId: mainWt.id },
    },
  });
  expect(sessRes.status()).toBe(201);
  const sessionId = (await sessRes.json()).session.id;

  await page.goto(`/s/${sessionId}`);
  const composer = page.getByRole("textbox", { name: "消息", exact: true });
  await expect(composer).toBeEditable();
  await composer.fill("unsent-draft-before-git-jump");

  // Open Code panel, open README.md tab, and scroll the reader.
  await toggleCode(page);
  const codeRegion = page.getByRole("region", { name: "代码工作区" });
  await page.getByRole("button", { name: "README.md", exact: true }).click();
  await expect(codeRegion).toBeVisible();
  await expect(
    page.locator("[data-code-reader]:visible [data-content]").first(),
  ).toContainText("line 1: git workbench route test");

  await page.locator("[data-code-reader]:visible").evaluate((el) => {
    el.scrollTop = 220;
  });
  const savedScroll = await page
    .locator("[data-code-reader]:visible")
    .evaluate((el) => el.scrollTop);
  expect(savedScroll).toBeGreaterThan(80);

  // Jump from CodeWorkspace header "在 Git 中查看" -> /git/{repoId}/worktrees/{mainWt.id}?returnSession={sessionId}.
  await page.getByRole("button", { name: "在 Git 中查看" }).click();
  await expect(page).toHaveURL(
    new RegExp(
      `/git/${repo.id}/worktrees/${mainWt.id}\\?.*returnSession=${sessionId}`,
    ),
  );
  await expect(page.getByRole("button", { name: "返回会话" })).toBeVisible();

  // Return to session: chat view, unsent draft, open Code panel, active README.md tab, and scroll are intact!
  await page.getByRole("button", { name: "返回会话" }).click();
  await expect(page).toHaveURL(`/s/${sessionId}`);
  await expect(codeRegion).toBeVisible();
  await expect(page.getByRole("tab", { name: /README\.md/ })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect
    .poll(() =>
      page.locator("[data-code-reader]:visible").evaluate((el) => el.scrollTop),
    )
    .toBeGreaterThan(80);

  // Close the code sidebar to verify the chat composer draft is still untouched.
  await toggleCode(page);
  await expect(composer).toHaveValue("unsent-draft-before-git-jump");

  // Close (archive) the hybrid session, then open /git with returnSession -> honest notice, Git workbench stays usable.
  expect((await request.delete(`/api/sessions/${sessionId}`)).status()).toBe(
    204,
  );
  await page.goto(
    `/git/${repo.id}/worktrees/${mainWt.id}?returnSession=${sessionId}`,
  );
  await expect(page.getByTestId("git-return-session-notice")).toContainText(
    /已归档|已关闭/,
  );
  await expect(
    page.getByText("docs: update guide on main").first(),
  ).toBeVisible();
});

test("retains cross-view state, isolates worktree drafts, handles query popstate, and keeps single /ws/sessions", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt, featWt } = await setupRepoWithLinkedWorktree(
    request,
    workDir,
  );

  let sessionsWsCount = 0;
  page.on("websocket", (ws) => {
    if (new URL(ws.url()).pathname === "/ws/sessions") {
      sessionsWsCount++;
    }
  });

  await page.goto(`/git/${repo.id}/worktrees/${featWt.id}`);
  await expect(page.getByTestId("git-workbench-topbar")).toBeVisible();
  expect(sessionsWsCount).toBe(1);

  // Filter Log search query -> switch to Commit & write draft -> switch to Branches -> back to Log.
  const searchBox = page.getByRole("textbox", { name: "搜索提交关键字" });
  await searchBox.fill("initial repository seed");
  await expect(page).toHaveURL(
    /q=initial\+repository\+seed|q=initial%20repository%20seed/,
  );

  await selectGitMode(page, "提交");
  await expect(page).toHaveURL(/view=commit/);
  const commitInput = page.getByRole("textbox", { name: "提交说明" });
  await commitInput.fill("feat(wt): isolated draft for featWt");

  await selectGitMode(page, "分支");
  await expect(page).toHaveURL(/view=branches/);

  await selectGitMode(page, "历史");
  await expect(page).toHaveURL(/view=log/);
  await expect(searchBox).toHaveValue("initial repository seed");

  // Browser Back / Forward across query-only URL changes updates view reactively without remounting shell.
  await page.goBack();
  await expect(page).toHaveURL(/view=branches/);
  await expect(
    page
      .getByRole("tablist", { name: "Git 工作台视图" })
      .getByRole("tab", { name: /^分支/ }),
  ).toHaveAttribute("aria-selected", "true");

  await page.goBack();
  await expect(page).toHaveURL(/view=commit/);
  await expect(commitInput).toHaveValue("feat(wt): isolated draft for featWt");

  await page.goForward();
  await expect(page).toHaveURL(/view=branches/);

  // Switch worktree to mainWt: commit draft must NOT leak across worktrees!
  await page.getByRole("button", { name: "切换操作目标工作树" }).click();
  await page
    .getByTestId("git-worktree-switcher-popover")
    .getByRole("menuitemradio", { name: /^main/ })
    .click();
  await expect(page).toHaveURL(
    new RegExp(`/git/${repo.id}/worktrees/${mainWt.id}`),
  );
  await selectGitMode(page, "提交");
  await expect(commitInput).toHaveValue("");

  // Switch back to featWt: featWt's draft is still preserved!
  await page.getByRole("button", { name: "切换操作目标工作树" }).click();
  await page
    .getByTestId("git-worktree-switcher-popover")
    .getByRole("menuitemradio", { name: /feat\/route/ })
    .click();
  await expect(commitInput).toHaveValue("feat(wt): isolated draft for featWt");

  // Sidebar grouping keeps the Git route and the single session-list socket.
  const openedDrawer = await revealSidebar(page);
  await useWorkbenchBranches(page);
  await expect(page).toHaveURL(
    new RegExp(`/git/${repo.id}/worktrees/${featWt.id}`),
  );
  await useWorkbenchBranches(page, false);
  await expect(page).toHaveURL(
    new RegExp(`/git/${repo.id}/worktrees/${featWt.id}`),
  );
  if (openedDrawer) await page.keyboard.press("Escape");

  expect(sessionsWsCount).toBe(1);
});

test("both sidebar layouts show GitHub project avatars and keep a fallback when they fail", async ({
  page,
  request,
  workDir,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  const { repo, mainWt, mainPath } = await setupRepoWithLinkedWorktree(
    request,
    workDir,
  );
  git(
    mainPath,
    "remote",
    "add",
    "origin",
    "git@github.com:octocat/example.git",
  );
  expect(
    (
      await request.post("/api/sessions", {
        data: {
          name: "Read project output",
          workspace: { repositoryId: repo.id, worktreeId: mainWt.id },
          cmd: ["/bin/sh", "-c", "sleep 120"],
        },
      })
    ).status(),
  ).toBe(201);

  const url = "https://avatars.githubusercontent.com/octocat?s=48";
  await page.route(url, async (route) => {
    expect(route.request().headers()).not.toHaveProperty("referer");
    await route.fulfill({
      contentType: "image/gif",
      headers: { "access-control-allow-origin": "*" },
      body: Buffer.from(
        "R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
        "base64",
      ),
    });
  });
  await page.goto("/");
  await revealSidebar(page);
  const heading = page
    .getByRole("region", { name: repo.name, exact: true })
    .getByRole("heading");
  const image = heading.locator("img");
  await expect(image).toBeVisible();
  await expect(image).toHaveAttribute("alt", "");
  await expect(heading).toHaveAccessibleName(`${repo.name} 1`);
  const initialBounds = await heading.boundingBox();

  await useWorkbenchBranches(page);
  const project = page.getByRole("treeitem", {
    name: `${repo.name}，1 个会话`,
    exact: true,
  });
  const branchImage = project.locator("img");
  await expect(branchImage).toBeVisible();
  await expect(branchImage).toHaveAttribute("src", url);
  const branchBounds = await project.boundingBox();

  await page.unroute(url);
  await page.route(url, (route) => route.abort());
  const requestOnReload = page.waitForRequest(url);
  await page.reload();
  await revealSidebar(page);
  await requestOnReload;
  await expect(branchImage).toHaveCount(0);
  await expect(project).toContainText("WE");
  expect(await project.boundingBox()).toEqual(branchBounds);

  await useWorkbenchBranches(page, false);
  await expect(image).toHaveCount(0);
  await expect(heading).toContainText("WE");
  expect(await heading.boundingBox()).toEqual(initialBounds);
});

test("project groups collapse independently and remember the choice", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt } = await setupRepoWithLinkedWorktree(request, workDir);
  for (const input of [
    {
      name: "Project build session",
      workspace: { repositoryId: repo.id, worktreeId: mainWt.id },
    },
    { name: "Direct shell session", cwd: workDir },
  ]) {
    expect(
      (
        await request.post("/api/sessions", {
          data: { ...input, cmd: ["/bin/sh", "-c", "sleep 120"] },
        })
      ).status(),
    ).toBe(201);
  }

  await page.goto("/");
  await revealSidebar(page);
  const sidebar = page.getByRole("navigation", { name: "导航", exact: true });
  const project = sidebar.getByRole("region", { name: repo.name, exact: true });
  const toggle = project.getByRole("button", {
    name: `${repo.name} 1`,
    exact: true,
  });
  const session = project.getByRole("button", {
    name: /^Project build session/,
  });
  const directSession = sidebar.getByRole("button", {
    name: /^Direct shell session/,
  });

  await expect(session).toBeVisible();
  await expect(directSession).toBeVisible();
  await toggle.press("Space");
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(toggle).toBeFocused();
  await expect(session).toBeHidden();
  await expect(directSession).toBeVisible();

  await page.reload();
  await revealSidebar(page);
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(session).toBeHidden();
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await expect(session).toBeVisible();

  await page.setViewportSize({ width: 390, height: 844 });
  await revealSidebar(page);
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await page.getByRole("button", { name: "关闭侧栏", exact: true }).click();
  await page.setViewportSize({ width: 1280, height: 900 });
  await revealSidebar(page);
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(session).toBeHidden();
});

test("the layout switch moves shared session rows and respects reduced motion", async ({
  page,
  request,
  workDir,
}) => {
  const { repo, mainWt } = await setupRepoWithLinkedWorktree(request, workDir);
  const created = await request.post("/api/sessions", {
    data: {
      name: "Keep this session open",
      workspace: { repositoryId: repo.id, worktreeId: mainWt.id },
      cmd: ["/bin/sh", "-c", "sleep 120"],
    },
  });
  expect(created.status()).toBe(201);
  const { session } = await created.json();
  const releaseFont = await holdTerminalFont(page);
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await page.goto(`/s/${session.id}`);
  await revealSidebar(page);

  const sidebar = page.getByRole("navigation", { name: "导航", exact: true });
  const toggle = sidebar.getByRole("radio", { name: "按分支", exact: true });
  const selected = sidebar.locator('[aria-current="page"]', {
    hasText: "Keep this session open",
  });
  await expect(selected).toBeVisible();
  const animatedRows = await page.evaluateHandle(() => {
    const rows: string[] = [];
    const animate = Element.prototype.animate;
    Element.prototype.animate = function (keyframes, options) {
      if (this instanceof HTMLElement && this.dataset.sidebarSessionId)
        rows.push(this.dataset.sidebarSessionId);
      return animate.call(this, keyframes, options);
    };
    return rows;
  });
  await toggle.press("Space");
  await expect(toggle).toBeChecked();
  await releaseFont();
  await expect(page.locator(".xterm-helper-textarea")).toHaveCount(1);
  await expect(toggle).toBeFocused();
  await expect(
    sidebar.getByRole("treeitem", { name: /^Keep this session open/ }),
  ).toHaveAttribute("aria-current", "page");
  await expect.poll(() => animatedRows.jsonValue()).toContain(session.id);
  await expect(page).toHaveURL(new RegExp(`/s/${session.id}$`));

  await page.emulateMedia({ reducedMotion: "reduce" });
  await animatedRows.evaluate((rows) => (rows.length = 0));
  await toggle.press("ArrowLeft");
  await expect(toggle).not.toBeChecked();
  await expect(selected).toBeVisible();
  expect(await animatedRows.jsonValue()).toEqual([]);
});
