import { createShell, expect, sessionHeader, test } from "./fixtures";

const longTitle = "新建会话页面与动画方案".repeat(7);

test("the session title opens its full name and renames the current session", async ({
  page,
  workDir,
}, testInfo) => {
  const phone = !!testInfo.project.use.isMobile;
  if (phone) await page.setViewportSize({ width: 320, height: 568 });
  await createShell(page, longTitle, workDir);
  const header = sessionHeader(page);
  const title = header.getByRole("button", { name: longTitle, exact: true });
  await page.evaluate(() => document.fonts.ready);
  await expect
    .poll(() =>
      title
        .locator("span")
        .evaluate((node) => node.scrollWidth > node.clientWidth),
    )
    .toBe(true);
  await title.click();
  const details = page.getByRole("dialog", { name: "会话名称", exact: true });
  await expect(details.getByText(longTitle, { exact: true })).toBeVisible();
  await details.getByRole("button", { name: "修改标题", exact: true }).click();
  const rename = page.getByRole("dialog", { name: "修改标题", exact: true });
  const field = rename.getByRole("textbox");
  await expect(field).toHaveValue(longTitle);
  await field.fill("Dock layout");
  await rename.getByRole("button", { name: "保存", exact: true }).click();
  const renamed = header.getByRole("button", {
    name: "Dock layout",
    exact: true,
  });
  await expect(renamed).toBeFocused();
  expect(
    await renamed
      .locator("span")
      .evaluate((node) => node.scrollWidth <= node.clientWidth),
  ).toBe(true);
  await page.reload();
  await expect(renamed).toBeVisible();
});

test("desktop workspace header orders identity, view switch, and grouped tools cleanly", async ({
  page,
  request,
  workDir,
}, testInfo) => {
  test.skip(!!testInfo.project.use.isMobile, "Desktop three-zone header");
  const configs = await (await request.get("/api/launch-configs")).json();
  let configId = configs.find(
    (item: { chat?: boolean; name: string }) =>
      item.chat && /claude/i.test(item.name),
  )?.id;
  if (!configId) {
    const created = await request.post("/api/launch-configs", {
      data: { name: "Claude Layout", cmd: ["claude"], env: {}, chat: true },
    });
    expect(created.status(), await created.text()).toBe(201);
    configId = (await created.json()).id;
  }
  const sessionRes = await request.post("/api/sessions", {
    data: { cwd: workDir, configId, view: "chat", name: "Topbar order" },
  });
  expect(sessionRes.status(), await sessionRes.text()).toBe(201);
  const { id } = (await sessionRes.json()).session;

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`/s/${id}`);
  const toolbar = page.locator(".session-toolbar header");
  const back = toolbar.getByRole("button", {
    name: "返回会话列表",
    exact: true,
  });
  const status = toolbar.locator(".status-dot");
  const title = toolbar.getByRole("button", {
    name: "Topbar order",
    exact: true,
  });
  const agent = toolbar.locator('[data-slot="chat-agent"]');
  const tabs = toolbar.getByRole("tablist", { name: "会话视图", exact: true });
  const sessionMenu = toolbar.getByRole("button", {
    name: "会话菜单",
    exact: true,
  });
  const files = toolbar.getByRole("button", { name: /^文件树 · / });
  const panels = toolbar.getByRole("button", { name: "打开面板", exact: true });
  const paneMenu = toolbar.locator("[data-dock-command-menu]");
  const display = toolbar.getByRole("button", { name: "显示", exact: true });

  for (const locator of [
    back,
    status,
    title,
    agent,
    tabs,
    sessionMenu,
    files,
    panels,
    paneMenu,
    display,
  ]) {
    await expect(locator).toBeVisible();
  }

  const boxes = await Promise.all([
    back.boundingBox(),
    status.boundingBox(),
    title.boundingBox(),
    agent.boundingBox(),
    tabs.boundingBox(),
    sessionMenu.boundingBox(),
    files.boundingBox(),
    panels.boundingBox(),
    paneMenu.boundingBox(),
    display.boundingBox(),
  ]);
  for (let i = 0; i < boxes.length - 1; i++) {
    expect(boxes[i]!.x + boxes[i]!.width).toBeLessThanOrEqual(
      boxes[i + 1]!.x + 1,
    );
  }
  expect(
    await toolbar.evaluate((node) => node.scrollWidth <= node.clientWidth),
  ).toBe(true);

  // Tablet width (768px) keeps the session title readable beside the view tabs.
  await page.setViewportSize({ width: 768, height: 800 });
  await expect(title).toBeVisible();
  expect((await title.locator("span").boundingBox())!.width).toBeGreaterThan(
    40,
  );
  expect(
    await toolbar.evaluate((node) => node.scrollWidth <= node.clientWidth),
  ).toBe(true);
});
