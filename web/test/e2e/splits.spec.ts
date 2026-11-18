import {
  test,
  expect,
  createShell,
  chooseDockPanel,
  chooseDockPanelById,
  expectSessionInputSide,
  openSplitPanel,
  selectSessionInputSide,
  toggleCode,
} from "./fixtures";
import type { Locator, Page } from "@playwright/test";

async function split(pane: Locator, direction: "左右" | "上下") {
  await openSplitPanel(pane);
  await pane
    .page()
    .getByRole("button", { name: `${direction}分屏 · 新建 Shell`, exact: true })
    .click();
}
async function command(page: Page, pane: Locator, value: string) {
  const input = pane.locator(".xterm-helper-textarea");
  await expect(input).toHaveAttribute("aria-disabled", "false");
  await input.focus();
  await page.keyboard.type(value);
  await page.keyboard.press("Enter");
}
async function ready(pane: Locator) {
  await expect(pane.locator(".xterm")).toBeAttached();
}

test("docking and minimizing a running terminal retain the emulator, layout and output", async ({
  page,
  workDir,
}, testInfo) => {
  test.skip(!!testInfo.project.use.isMobile, "Wide docking surface");
  const id = await createShell(page, "Dock anchor", workDir);
  const original = page.locator(`[data-session-id="${id}"]`);
  await split(original, "左右");
  const second = page.locator(".split-terminal").nth(1);
  const secondId = await second.getAttribute("data-session-id");
  const root = page.locator(`[data-dock-panel="${secondId}"]`);
  const anchor = page.locator(`[data-dock-panel="${id}"]`);
  await ready(second);
  const emulator = await second.locator(".xterm").elementHandle();
  await command(page, second, "sleep 1; printf 'DOCK_%s\\n' COMPLETE");

  const header = root.locator("[draggable=true]");
  const box = (await anchor.boundingBox())!;
  await header.dragTo(anchor, {
    targetPosition: { x: box.width / 2, y: box.height - 10 },
  });
  await expect
    .poll(async () => (await root.boundingBox())!.y)
    .toBeGreaterThan((await anchor.boundingBox())!.y + 100);
  const placed = await root.boundingBox();
  await header.dblclick();
  await expect(anchor).toBeHidden();
  await expect(root).toHaveAccessibleName(/^终端 · .+$/);
  const name = (await root.getAttribute("aria-label"))!;
  await root
    .getByRole("button", { name: `还原「${name}」布局`, exact: true })
    .click();
  expect(await root.boundingBox()).toEqual(placed);
  await root
    .getByRole("button", { name: `收起「${name}」`, exact: true })
    .click();
  await expect(root).toBeHidden();
  await page.getByRole("button", { name, exact: true }).click();
  await expect(second.locator(".xterm-rows")).toContainText("DOCK_COMPLETE");
  expect(await emulator!.evaluate((element) => element.isConnected)).toBe(true);
  await page.setViewportSize({ width: 390, height: 844 });
  await chooseDockPanel(page, name);
  await expect(root).toBeVisible();
  await expect(anchor).toBeHidden();
  await expect(second.locator(".xterm-helper-textarea")).not.toBeFocused();
  await page.setViewportSize({ width: 1440, height: 900 });
  expect(await root.boundingBox()).toEqual(placed);
});

test("nested splits preserve terminal state, isolate input and restore layout", async ({
  page,
  request,
  workDir,
}, testInfo) => {
  await page.addInitScript(() => {
    // Trusted HTTP deployments expose getRandomValues, but not randomUUID.
    Object.defineProperty(crypto, "randomUUID", { value: undefined });
  });
  const id = await createShell(page, "Primary shell", workDir);
  const original = page.locator(`[data-session-id="${id}"]`);
  await original
    .locator(".xterm")
    .evaluate((element) => element.setAttribute("data-retained", "yes"));
  await command(
    page,
    original,
    "export SPLIT_ORIGINAL=original; printf 'original-pane\\n'",
  );
  const draftButton = testInfo.project.use.isMobile ? original : page;
  await draftButton.getByRole("button", { name: "草稿模式" }).click();
  const draftInput = original.locator(".terminal-composer textarea");
  await draftInput.fill("keep this draft");
  await split(original, "左右");
  await expect(page.locator(".split-terminal")).toHaveCount(2);
  const second = page.locator(".split-terminal").nth(1);
  await ready(second);
  await expect(original.locator(".xterm")).toHaveAttribute(
    "data-retained",
    "yes",
  );
  await expect(draftInput).toHaveValue("keep this draft");
  await command(
    page,
    second,
    "printf 'second-pane:%s\\n' \"${SPLIT_ORIGINAL:-independent}\"",
  );
  await expect(second.locator(".xterm-rows")).toContainText(
    "second-pane:independent",
  );
  await expect(original.locator(".xterm-rows")).not.toContainText(
    "second-pane",
  );
  await toggleCode(second);
  await expect(
    page.getByRole("region", { name: "文件树", exact: true }),
  ).toBeVisible();
  if (testInfo.project.use.isMobile)
    await expect(second.locator(".terminal-host")).toBeHidden();
  await toggleCode(second);
  await expect(second.locator(".terminal-host")).toBeVisible();
  await expect(original.locator(".xterm")).toHaveAttribute(
    "data-retained",
    "yes",
  );
  await split(second, "上下");
  await expect(page.locator(".split-terminal")).toHaveCount(3);
  await ready(page.locator(".split-terminal").nth(2));
  if (testInfo.project.name === "chromium-desktop") {
    const divider = page
      .getByRole("separator", { name: "调整窗格宽度" })
      .first();
    await divider.focus();
    await page.keyboard.press("ArrowLeft");
    await expect(divider).toHaveAttribute("aria-valuenow", "45");
    // Grab the vertical divider above the crossing with the nested divider.
    await divider.hover({ position: { x: 2, y: 20 } });
    const bounds = (await divider.boundingBox())!;
    await page.mouse.down();
    await page.mouse.move(bounds.x + 100, bounds.y + 20);
    await page.mouse.up();
    await expect
      .poll(async () => Number(await divider.getAttribute("aria-valuenow")))
      .toBeGreaterThan(45);
  } else {
    await expect(page.locator(".split-terminal:visible")).toHaveCount(1);
    await chooseDockPanel(page, "终端 · Primary shell");
    await expect(original).toBeVisible();
    await expect(draftInput).toHaveValue("keep this draft");
  }
  const saved = await page.evaluate(
    (key) => localStorage.getItem(`webmux.layout.${key}`),
    id,
  );
  await page.reload();
  await expect(page.locator(".split-terminal")).toHaveCount(3);
  expect(
    await page.evaluate(
      (key) => localStorage.getItem(`webmux.layout.${key}`),
      id,
    ),
  ).toBe(saved);
  for (const pane of await page.locator(".split-terminal").all())
    await ready(pane);
  const sessions = await (await request.get("/api/sessions")).json();
  expect(sessions).toHaveLength(3);
  expect(
    sessions.every((session: { cwd: string }) => session.cwd === workDir),
  ).toBe(true);
  await chooseDockPanel(page, "终端 · Primary shell");
  await expect(original.locator(".terminal-composer textarea")).toHaveValue(
    "keep this draft",
  );
  for (const theme of ["light", "dark"] as const) {
    await page.emulateMedia({ colorScheme: theme });
    await expect(page.locator("html")).toHaveClass(
      theme === "dark" ? /dark/ : /^(?!.*dark)/,
    );
    const screenshot = await page.screenshot({
      path: `/tmp/webmux-split-pr-evidence/after-${testInfo.project.use.isMobile ? "mobile" : "desktop"}-${theme}.jpg`,
      type: "jpeg",
      quality: 75,
      scale: "css",
      animations: "disabled",
    });
    await testInfo.attach(`splits-${theme}`, {
      body: screenshot,
      contentType: "image/jpeg",
    });
  }
});

test("the keyboard size hold preference applies to every mounted pane", async ({
  page,
  workDir,
}) => {
  const id = await createShell(page, "Hold preference", workDir);
  const first = page.locator(`[data-session-id="${id}"]`);
  await split(first, "左右");
  await expect(page.locator(".split-terminal")).toHaveCount(2);
  const second = page.locator(".split-terminal").nth(1);
  await ready(second);
  const menu = page.getByRole("button", { name: "终端菜单", exact: true });
  // The dropdown renders in a portal outside the pane.
  const option = page.getByRole("menuitemcheckbox", {
    name: "键盘弹出时保持尺寸",
    exact: true,
  });
  const secondId = (await second.getAttribute("data-session-id"))!;

  await chooseDockPanelById(page, secondId);
  await expect(second).toBeVisible();
  await menu.click();
  await expect(option).toBeChecked();
  await option.click();
  await expect(option).not.toBeChecked();
  await page.keyboard.press("Escape");

  await chooseDockPanel(page, "终端 · Hold preference");
  await expect(first).toBeVisible();
  await menu.click();
  await expect(option).not.toBeChecked();
});

test("tmux prefix splits, focuses, zooms and closes only the pane", async ({
  page,
  request,
  workDir,
}, testInfo) => {
  test.skip(
    testInfo.project.name !== "chromium-desktop",
    "Keyboard shortcuts use a physical keyboard",
  );
  const id = await createShell(page, "Keyboard shell", workDir);
  await page.locator(".xterm-helper-textarea").focus();
  await page.keyboard.press("Control+b");
  await page.keyboard.press("Shift+5");
  await expect(page.locator(".split-terminal")).toHaveCount(2);
  const second = page.locator(".split-terminal").nth(1);
  await ready(second);
  const secondId = await second.getAttribute("data-session-id");
  await second.locator(".xterm-helper-textarea").focus();
  await page.keyboard.press("Control+b");
  await page.keyboard.press("o");
  await expect(
    page.locator(`[data-session-id="${id}"] .xterm-helper-textarea`),
  ).toBeFocused();
  await page.keyboard.press("Control+b");
  await page.keyboard.press("z");
  await expect(second).toBeHidden();
  await page.keyboard.press("Control+b");
  await page.keyboard.press("z");
  await expect(second).toBeVisible();
  await second.locator(".xterm-helper-textarea").focus();
  await page.keyboard.press("Control+b");
  await page.keyboard.press("x");
  await expect(page.locator(".split-terminal")).toHaveCount(1);
  const retained = await (
    await request.get(`/api/sessions/${secondId}`)
  ).json();
  expect(retained.status).not.toBe("exited");
  await command(
    page,
    page.locator(".split-terminal"),
    "printf 'shortcuts-do-not-leak\\n'",
  );
  await expect(page.locator(".xterm-rows")).toContainText(
    "shortcuts-do-not-leak",
  );
});

test("existing sessions can join a split and missing panes can be dismissed", async ({
  page,
  request,
  workDir,
}) => {
  const id = await createShell(page, "Anchor", workDir);
  const response = await request.post("/api/sessions", {
    data: {
      cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
      cwd: workDir,
      name: "Existing agent",
    },
  });
  expect(response.ok()).toBe(true);
  const existing = (await response.json()).session;
  await openSplitPanel(page);
  await page
    .getByRole("button", { name: "加入已有会话…", exact: true })
    .click();
  await page.getByRole("radio", { name: "上下分屏", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /Existing agent/ })
    .click();
  await expect(page.locator(".split-terminal")).toHaveCount(2);
  const joined = page.locator(`[data-session-id="${existing.id}"]`);
  await ready(joined);
  expect(await (await request.get("/api/sessions")).json()).toHaveLength(2);
  await request.delete(`/api/sessions/${existing.id}`);
  await expect
    .poll(
      async () =>
        (await (await request.get(`/api/sessions/${existing.id}`)).json())
          .status,
    )
    .toBe("exited");
  await request.delete(`/api/sessions/${existing.id}?remove=1`);
  await page.reload();
  await chooseDockPanelById(page, existing.id);
  await expect(joined).toBeVisible();
  await expect(
    page.getByRole("button", { name: "返回列表", exact: true }),
  ).toBeVisible();
  await openSplitPanel(joined);
  await page
    .getByRole("button", { name: "关闭窗格（保留会话）", exact: true })
    .click();
  await expect(page.locator(".split-terminal")).toHaveCount(1);
  await expect(page.locator(`[data-session-id="${id}"]`)).toBeVisible();
});

test("返回列表 in a split leaves only its own pane", async ({
  page,
  request,
  workDir,
}) => {
  const id = await createShell(page, "Anchor", workDir);
  const created = await request.post("/api/sessions", {
    data: {
      cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
      cwd: workDir,
      name: "Gone soon",
    },
  });
  const gone = (await created.json()).session;
  await openSplitPanel(page);
  await page
    .getByRole("button", { name: "加入已有会话…", exact: true })
    .click();
  await page.getByRole("radio", { name: "左右分屏", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /Gone soon/ })
    .click();
  await expect(page.locator(".split-terminal")).toHaveCount(2);
  const pane = page.locator(`[data-session-id="${gone.id}"]`);
  await ready(pane);
  await request.delete(`/api/sessions/${gone.id}`);
  await expect
    .poll(
      async () =>
        (await (await request.get(`/api/sessions/${gone.id}`)).json()).status,
    )
    .toBe("exited");
  await request.delete(`/api/sessions/${gone.id}?remove=1`);
  await page.reload();
  await chooseDockPanelById(page, gone.id);
  await expect(pane).toBeVisible();
  await page.getByRole("button", { name: "返回列表", exact: true }).click();
  await expect(page.locator(".split-terminal")).toHaveCount(1);
  await expect(page).toHaveURL(new RegExp(`/s/${id}`));
  await expect(page.locator(`[data-session-id="${id}"]`)).toBeVisible();
});

test("failed split creation leaves the current terminal intact", async ({
  page,
  workDir,
}) => {
  await createShell(page, "Failure recovery", workDir);
  await page.route("**/api/sessions", (route) =>
    route.request().method() === "POST"
      ? route.fulfill({
          status: 400,
          json: { error: "configured shell unavailable" },
        })
      : route.continue(),
  );
  await split(page.locator(".split-terminal"), "左右");
  await expect(page.getByRole("alert")).toContainText(
    "configured shell unavailable",
  );
  await expect(page.locator(".split-terminal")).toHaveCount(1);
  await command(
    page,
    page.locator(".split-terminal"),
    "printf 'still-usable\\n'",
  );
  await expect(page.locator(".xterm-rows")).toContainText("still-usable");
});

test("a split preserves hybrid chat and terminal switching beside a shell", async ({
  page,
  request,
  workDir,
}) => {
  const id = await createShell(page, "Shell beside chat", workDir);
  const anchor = page.locator(`[data-session-id="${id}"]`);
  await anchor
    .locator(".xterm")
    .evaluate((element) => element.setAttribute("data-retained", "yes"));
  const config = await request.post("/api/launch-configs", {
    data: { name: "Chat agent", cmd: ["omp"], env: {}, chat: true },
  });
  expect(config.status()).toBe(201);
  const created = await request.post("/api/sessions", {
    data: {
      cwd: workDir,
      configId: (await config.json()).id,
      view: "chat",
      name: "Chat agent",
    },
  });
  expect(created.status()).toBe(201);
  const chatId = (await created.json()).session.id;
  await openSplitPanel(anchor);
  await page
    .getByRole("button", { name: "加入已有会话…", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /Chat agent/ })
    .click();
  const chat = page.locator(`[data-dock-panel="chat:${chatId}"]`);
  await expect(
    chat.getByRole("textbox", { name: "消息", exact: true }),
  ).toBeEditable();
  await chat
    .getByRole("textbox", { name: "消息", exact: true })
    .fill("split chat stays independent");
  await chat.getByRole("button", { name: /^发送$/ }).click();
  await expect(
    chat.getByRole("list", { name: "对话", exact: true }),
  ).toContainText("echo: split chat stays independent");
  // The echo arrives before the turn ends. Switching while it still runs
  // asks to interrupt it first, so wait for the turn to end.
  await expect(
    chat.getByRole("status").filter({ hasText: "正在工作…" }),
  ).toHaveCount(0);
  // Completion toasts can cover pane controls and pause expiry on hover.
  // Dismiss them if present, without requiring one to appear.
  const completionToast = page
    .getByRole("region", { name: "会话通知", exact: true })
    .locator(`[data-toast-session-id="${chatId}"]`);
  await page.addLocatorHandler(completionToast, async () => {
    await completionToast
      .getByRole("button", { name: "关闭 Chat agent 的通知", exact: true })
      .click();
  });
  await selectSessionInputSide(page, "terminal");
  const terminal = page.locator(`[data-session-id="${chatId}"]`);
  await expect(terminal.locator(".xterm")).toBeVisible();
  await expect(terminal.locator(".xterm-rows")).toContainText("prompt>");
  await expectSessionInputSide(page, "terminal");
  await expect(anchor.locator(".xterm")).toHaveAttribute(
    "data-retained",
    "yes",
  );
  await chooseDockPanel(page, "终端 · Shell beside chat");
  await expect(page).toHaveTitle(/Shell beside chat · WebMux$/);
  await expect(page.locator(".split-terminal")).toHaveCount(2);

  // Archiving the hybrid pane removes only that pane; the page stays.
  await chooseDockPanel(page, "终端 · Chat agent");
  await page.getByRole("button", { name: "终端菜单", exact: true }).click();
  await page
    .getByRole("menuitem", { name: "停止会话并归档", exact: true })
    .click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "停止并归档", exact: true })
    .click();
  await expect(page.locator(".split-terminal")).toHaveCount(1);
  await expect(page).toHaveURL(new RegExp(`/s/${id}$`));
  await expect(page).toHaveTitle(/Shell beside chat · WebMux$/);
  await expect(anchor).toBeVisible();
  await expect(anchor.locator(".xterm")).toHaveAttribute(
    "data-retained",
    "yes",
  );
});
