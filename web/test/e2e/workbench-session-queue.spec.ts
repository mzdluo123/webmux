import { execFileSync } from "node:child_process";
import { join } from "node:path";
import {
  test,
  expect,
  openSessionQueue,
  revealSidebar,
  useWorkbenchBranches,
} from "./fixtures";

test("the sidebar waits for project and archive data before showing groups", async ({
  page,
  request,
  workDir,
}) => {
  const path = join(workDir, "slow-project");
  execFileSync("git", ["init", "-b", "main", path]);
  const registered = await request.post("/api/repositories", {
    data: { path },
  });
  expect(registered.ok()).toBe(true);
  for (const [name, cwd] of [
    ["Project shell", path],
    ["Direct shell", workDir],
  ]) {
    const created = await request.post("/api/sessions", {
      data: { name, cwd, cmd: ["/bin/sh"] },
    });
    expect(created.status()).toBe(201);
  }

  const repositories = Promise.withResolvers<void>();
  const archive = Promise.withResolvers<void>();
  const archiveRequested = Promise.withResolvers<void>();
  await page.route("**/api/repositories", async (route) => {
    await repositories.promise;
    await route.continue();
  });
  await page.route("**/api/sessions?state=archived", async (route) => {
    archiveRequested.resolve();
    await archive.promise;
    await route.continue();
  });

  try {
    await page.goto("/");
    await revealSidebar(page);
    const sidebar = page.getByRole("navigation", { name: "导航", exact: true });
    const skeleton = sidebar.getByRole("status", { name: "正在加载会话" });
    const direct = sidebar.getByRole("list", { name: "直接会话", exact: true });
    const project = sidebar.getByRole("list", { name: "slow-project" });
    await archiveRequested.promise;
    await expect(skeleton).toBeVisible();
    await expect(direct).toHaveCount(0);

    const metadata = page.waitForResponse(
      (response) => new URL(response.url()).pathname === "/api/repositories",
    );
    repositories.resolve();
    expect((await metadata).ok()).toBe(true);
    await expect(skeleton).toBeVisible();
    await expect(project).toHaveCount(0);
    await expect(direct).toHaveCount(0);

    archive.resolve();
    await expect(
      project.getByRole("button", { name: /^Project shell/ }),
    ).toBeVisible();
    await expect(
      direct.getByRole("button", { name: /^Direct shell/ }),
    ).toBeVisible();
    await expect(
      direct.getByRole("button", { name: /^Project shell/ }),
    ).toHaveCount(0);
    await expect(skeleton).toHaveCount(0);
  } finally {
    repositories.resolve();
    archive.resolve();
  }
});

test("the default queue prioritizes explicit attention and keeps a focused session in place", async ({
  page,
  request,
  workDir,
}) => {
  const create = async (name: string) => {
    const response = await request.post("/api/sessions", {
      data: {
        name,
        cwd: workDir,
        cmd: [
          "/bin/sh",
          "-c",
          "printf 'queue fixture\npreview output\n'; sleep 120",
        ],
      },
    });
    expect(response.status()).toBe(201);
    return (await response.json()).session.id as string;
  };
  const urgent = await create("Review build");
  const running = await create("Watch backend");
  const attention = await request.post(`/api/sessions/${urgent}/attention`, {
    data: { kind: "error", message: "Build needs a retry" },
  });
  expect(attention.status()).toBe(200);

  await page.goto("/");
  await openSessionQueue(page);
  const main = page.getByRole("main");
  const issue = main.locator(`[data-session-id="${urgent}"]`);
  const monitor = main.locator(`[data-session-id="${running}"]`);
  await expect(issue).toHaveAttribute("data-queue-group", "attention");
  await expect(issue).toContainText("Build needs a retry");
  await expect(monitor).toHaveAttribute("data-queue-group", "running");
  const link = issue.getByRole("link", { name: "Review build" });
  await link.focus();
  expect(
    (await request.delete(`/api/sessions/${urgent}/attention`)).status(),
  ).toBe(204);
  await expect(issue.getByText("Build needs a retry")).toHaveCount(0);
  await expect(link).toBeFocused();
  await expect(issue).toHaveAttribute("data-queue-group", "attention");
  await main.getByRole("searchbox").focus();
  await expect(issue).toHaveAttribute("data-queue-group", "running");

  const raise = () =>
    request.post(`/api/sessions/${urgent}/attention`, {
      data: { kind: "error", message: "Build needs a retry" },
    });
  const clear = () => request.delete(`/api/sessions/${urgent}/attention`);
  expect((await raise()).ok()).toBeTruthy();
  await expect(issue).toHaveAttribute("data-queue-group", "attention");
  const actions = issue.getByRole("button", { name: "会话操作：Review build" });
  await actions.click();
  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  expect((await clear()).status()).toBe(204);
  await expect(issue.getByText("Build needs a retry")).toHaveCount(0);
  await expect(menu).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(actions).toBeFocused();
  await main.getByRole("searchbox").focus();

  expect((await raise()).ok()).toBeTruthy();
  await expect(issue).toHaveAttribute("data-queue-group", "attention");
  const trigger = issue.getByRole("button", { name: "预览 Review build" });
  await trigger.click();
  const preview =
    (page.viewportSize()?.width ?? 0) >= 1440
      ? page.getByRole("complementary", {
          name: "预览 Review build",
          exact: true,
        })
      : page.getByRole("dialog", { name: "Review build", exact: true });
  await expect(preview.getByLabel("终端输出预览")).toContainText(
    "queue fixture",
  );
  const close = preview.getByRole("button", { name: "关闭预览" });
  await expect(close).toBeFocused();
  expect((await clear()).status()).toBe(204);
  await close.click();
  await expect(trigger).toBeFocused();

  const exitedSession = await request.post("/api/sessions", {
    data: {
      name: "Failed deploy",
      cwd: workDir,
      cmd: ["/bin/sh", "-c", "exit 7"],
    },
  });
  expect(exitedSession.status()).toBe(201);
  const search = main.getByRole("searchbox");
  await search.fill("Failed deploy");
  const exited = main.getByRole("button", { name: "已退出 1" });
  const deploy = main.getByRole("link", { name: "Failed deploy" });
  await expect(deploy).toBeVisible();
  await exited.click();
  await expect(deploy).toHaveCount(0);
  await exited.click();
  await expect(deploy).toBeVisible();
  await search.fill("");
  await expect(exited).toHaveAttribute("aria-expanded", "false");

  expect((await raise()).ok()).toBeTruthy();
  await search.fill("stale filter");
  const drawer = await revealSidebar(page);
  const tree = page.getByRole("tree", { name: "仓库与会话" });
  await expect(tree).toHaveCount(0);
  const sidebar = page.getByRole("navigation", { name: "导航", exact: true });
  const conversation = sidebar.getByRole("button", { name: /^Watch backend/ });
  await expect(conversation).toHaveCount(1);
  await expect(conversation).toHaveAccessibleDescription(workDir);
  const directRows = sidebar.getByRole("list", {
    name: "直接会话",
    exact: true,
  });
  await expect(
    directRows.getByRole("button", { name: /^Review build/ }),
  ).toHaveCount(1);
  const scope = sidebar.getByRole("radiogroup", { name: "会话范围" });
  const all = scope.getByRole("radio", { name: "全部", exact: true });
  const pending = scope.getByRole("radio", { name: "待处理 1", exact: true });
  const directGroup = sidebar.getByRole("region", {
    name: "直接会话",
    exact: true,
  });
  const toggle = directGroup.getByRole("heading").getByRole("button");
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  if (drawer) await pending.click();
  else {
    await all.focus();
    await all.press("ArrowRight");
    await expect(pending).toBeFocused();
  }
  await expect(pending).toHaveAttribute("aria-checked", "true");
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await expect(
    directRows.getByRole("button", { name: /^Review build/ }),
  ).toBeVisible();
  await expect(conversation).toHaveCount(0);
  if (drawer) await page.keyboard.press("Escape");
  await expect(search).toHaveValue("");
  await expect(issue).toHaveAttribute("data-queue-group", "attention");
  await expect(monitor).toHaveCount(0);
  await actions.click();
  await expect(menu).toBeVisible();
  expect((await clear()).status()).toBe(204);
  await expect(issue.getByText("Build needs a retry")).toHaveCount(0);
  await expect(menu).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(actions).toBeFocused();
  await search.focus();
  await expect(issue).toHaveCount(0);
  await expect(main.getByText("当前没有需要你处理的会话。")).toBeVisible();
  const reopened = await revealSidebar(page);
  const zero = scope.getByRole("radio", { name: "待处理", exact: true });
  await expect(zero).toHaveAttribute("aria-checked", "true");
  await expect(
    sidebar.getByText("当前没有需要你处理的会话。", { exact: true }),
  ).toContainText("当前没有需要你处理的会话。");
  expect((await raise()).ok()).toBeTruthy();
  const sidebarIssue = directRows.getByRole("button", {
    name: /^Review build/,
  });
  await sidebarIssue.focus();
  await expect(sidebarIssue).toBeFocused();
  expect((await clear()).status()).toBe(204);
  await expect(zero).toBeFocused();
  if (reopened) await page.keyboard.press("Escape");
  await search.fill("keep this filter");
  const layoutDrawer = await useWorkbenchBranches(page);
  await useWorkbenchBranches(page, false);
  if (layoutDrawer) await page.keyboard.press("Escape");
  await expect(search).toHaveValue("keep this filter");
  const restoredDrawer = await revealSidebar(page);
  await expect(zero).toBeChecked();
  await all.click();
  await expect(all).toHaveAttribute("aria-checked", "true");
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  if (restoredDrawer) await page.keyboard.press("Escape");
  await useWorkbenchBranches(page);
  await page.reload();
  await revealSidebar(page);
  await expect(
    page.getByRole("radio", { name: "按分支", exact: true }),
  ).toBeChecked();
});

test("closing the docked preview returns focus after a running session exits", async ({
  page,
  request,
  workDir,
}, info) => {
  test.skip(
    !!info.project.use.isMobile,
    "the dock is only used on wide screens",
  );
  await page.setViewportSize({ width: 1440, height: 900 });
  const response = await request.post("/api/sessions", {
    data: {
      name: "Wait for deployment",
      cwd: workDir,
      cmd: ["/bin/sh", "-c", "printf 'ready\\n'; sleep 120"],
    },
  });
  expect(response.status()).toBe(201);
  const { session } = await response.json();
  await page.goto("/");
  await openSessionQueue(page);
  const main = page.getByRole("main");
  const row = main.locator(`[data-session-id="${session.id}"]`);
  await expect(row).toHaveAttribute("data-queue-group", "running");
  const trigger = row.getByRole("button", { name: "预览 Wait for deployment" });
  await trigger.click();
  const close = main
    .getByRole("complementary", { name: "预览 Wait for deployment" })
    .getByRole("button", { name: "关闭预览" });
  await expect(close).toBeFocused();
  expect((await request.delete(`/api/sessions/${session.id}`)).status()).toBe(
    204,
  );
  await expect(row).toHaveAttribute("data-status", "exited");
  await expect(close).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(trigger).toBeFocused();
  await main.getByRole("searchbox").focus();
  await expect(main.getByRole("button", { name: "已退出 1" })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  await expect(row).toBeHidden();
});

test("both sidebar filters fit at double text size on a narrow screen", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 568 });
  await page.goto("/");
  await page.evaluate(() => localStorage.setItem("webmux.locale", "en"));
  await page.reload();
  await page.evaluate(() => {
    document.documentElement.style.fontSize = "200%";
  });
  await page.getByRole("button", { name: "Open sidebar", exact: true }).click();
  const filters = page.getByRole("radiogroup", { name: "Session scope" });
  const all = filters.getByRole("radio", { name: "All", exact: true });
  const pending = filters.getByRole("radio", {
    name: "Needs attention",
    exact: true,
  });
  await pending.scrollIntoViewIfNeeded();
  await expect(all).toBeInViewport({ ratio: 1 });
  await expect(pending).toBeInViewport({ ratio: 1 });
  await pending.click();
  await expect(pending).toHaveAttribute("aria-checked", "true");
});
