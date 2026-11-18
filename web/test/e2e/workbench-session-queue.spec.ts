import { test, expect } from "./fixtures";

test("the default queue prioritizes explicit attention and keeps a focused session in place", async ({
  page,
  request,
  workDir,
}, info) => {
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

  await main.getByRole("searchbox").fill("stale filter");
  if (info.project.use.isMobile)
    await page.getByRole("button", { name: "打开侧栏" }).click();
  const tree = page.getByRole("tree", { name: "仓库与会话" });
  await expect(
    tree.getByRole("treeitem", { name: /Watch backend/ }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: /^待处理会话/ }).click();
  await expect(main.getByRole("searchbox")).toHaveValue("");
  await expect(main.getByText("当前没有需要你处理的会话。")).toBeVisible();
  await main.getByRole("radio", { name: "按工作区" }).click();
  await page.reload();
  await expect(main.getByRole("radio", { name: "按工作区" })).toBeChecked();
});
