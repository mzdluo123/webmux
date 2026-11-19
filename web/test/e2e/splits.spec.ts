import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import {
  test,
  expect,
  createShell,
  chooseDockPanel,
  chooseDockPanelById,
  expectConnected,
  expectDraftMode,
  expectSessionInputSide,
  openDockCommands,
  narrowSessionHeader,
  openSessionMenu,
  openSplitPanel,
  selectSessionInputSide,
  sessionHeader,
  toggleCode,
  toggleDraftMode,
  uploadAction,
} from "./fixtures";
import type { Locator, Page } from "@playwright/test";

async function split(pane: Locator, direction: "左右" | "上下") {
  await openSplitPanel(pane);
  await pane
    .page()
    .getByRole("button", {
      name: narrowSessionHeader(pane.page())
        ? "快速新建 Shell"
        : `${direction}分屏 · 新建 Shell`,
      exact: true,
    })
    .click();
}
async function command(page: Page, pane: Locator, value: string) {
  await expect(pane.locator(".restore-overlay")).toBeHidden();
  await expect(pane.locator(".xterm-rows")).toContainText("webmux-e2e$");
  const input = pane.locator(".xterm-helper-textarea");
  await expect(input).toHaveAttribute("aria-disabled", "false");
  await input.focus();
  await page.keyboard.type(value);
  await page.keyboard.press("Enter");
}
async function ready(pane: Locator) {
  await expect(pane.locator(".xterm")).toBeAttached();
}

test("edge tabs restore the original dock layout and keep it restored", async ({
  page,
  workDir,
}, testInfo) => {
  test.skip(!!testInfo.project.use.isMobile, "Wide docking surface");
  const id = await createShell(page, "Dock anchor", workDir);
  const original = page.locator(`[data-session-id="${id}"]`);
  await split(original, "左右");
  await expect(page.locator(".split-terminal")).toHaveCount(2);
  const second = page.locator(".split-terminal").nth(1);
  const secondId = await second.getAttribute("data-session-id");
  const root = page.locator(`[data-dock-panel="${secondId}"]`);
  const anchor = page.locator(`[data-dock-panel="${id}"]`);

  const header = root.locator("[draggable=true]");
  const box = (await anchor.boundingBox())!;
  await header.locator(".dock-title-space").dragTo(anchor, {
    targetPosition: { x: box.width / 2, y: box.height - 10 },
  });
  await expect
    .poll(async () => (await root.boundingBox())!.y)
    .toBeGreaterThan((await anchor.boundingBox())!.y + 100);
  const divider = page.getByRole("separator", { name: "调整窗格高度" });
  await divider.press("ArrowDown");
  await expect(divider).toHaveAttribute("aria-valuenow", "55");
  const layout = () => Promise.all([anchor.boundingBox(), root.boundingBox()]);
  const placed = await layout();
  await header.locator(".dock-title-space").dblclick();
  await expect(anchor).toBeHidden();
  await expect(root).toHaveAccessibleName(/.+/);
  const name = (await root.getAttribute("aria-label"))!;
  await (
    await openDockCommands(root)
  )
    .getByRole("menuitem", { name: /^还原/ })
    .click();
  await expect.poll(layout).toEqual(placed);

  const tab = page.locator(`[data-dock-tab="${secondId}"]`);
  const bottomTab = page.locator('[data-dock-rail="bottom"]').locator(tab);
  const rightTab = page.locator('[data-dock-rail="right"]').locator(tab);
  const expectRestored = async () => {
    await expect(tab).toHaveCount(0);
    await expect.poll(layout).toEqual(placed);
  };
  const rows = second.locator(".xterm-rows");
  const input = second.locator(".xterm-helper-textarea");
  const anchorInput = original.locator(".xterm-helper-textarea");
  const emulator = await second.locator(".xterm").elementHandle();
  await command(
    page,
    second,
    "printf 'DOCK_%s\\n' STARTED; while [ ! -e dock-release ]; do sleep 0.1; done; printf 'DOCK_%s\\n' COMPLETE; printf done > dock-result",
  );
  await expect(rows).toContainText("DOCK_STARTED");

  await (
    await openDockCommands(root)
  )
    .getByRole("menuitem", { name: /^收起(?:「.+」)?$/ })
    .click();
  await expect(root).toBeHidden();
  await expect(bottomTab).toBeVisible();
  await expect(bottomTab).toHaveAccessibleName(name);
  await expect
    .poll(async () => (await anchor.boundingBox())!.height)
    .toBeGreaterThan(placed[0]!.height);
  // Release the command only once minimized, and wait for it to finish there.
  await writeFile(join(workDir, "dock-release"), "");
  const result = join(workDir, "dock-result");
  await expect
    .poll(() => readFile(result, "utf8").catch(() => ""))
    .toBe("done");
  await expect(root).toBeHidden();

  await bottomTab.click();
  await expectRestored();
  await expect(rows).toContainText("DOCK_COMPLETE");
  expect(await emulator!.evaluate((element) => element.isConnected)).toBe(true);

  await expect(root).toHaveAttribute("data-dock-active", "true");
  await anchor.locator(".xterm-screen").click();
  await expect(anchorInput).toBeFocused();
  await expectRestored();

  await input.focus();
  await expect(input).toBeFocused();
  await page.keyboard.press("Escape");
  await expectRestored();
  await page.keyboard.press("Control+b");
  await page.keyboard.press("o");
  await expect(anchorInput).toBeFocused();
  await expectRestored();

  await openDockCommands(root);
  await page.getByRole("menuitem", { name: "收起到边栏", exact: true }).hover();
  await page.getByRole("menuitem", { name: "收起到右侧", exact: true }).click();
  await expect(root).toBeHidden();
  await expect(bottomTab).toHaveCount(0);
  await expect(rightTab).toBeVisible();
  await page.reload();
  await expect(rightTab).toBeVisible();
  await expect(root).toBeHidden();
  const desktopViewport = page.viewportSize()!;
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator("[data-dock-rail]")).toHaveCount(0);
  await chooseDockPanel(page, name);
  await expect(root).toBeVisible();
  await expect(anchor).toBeHidden();
  await expect(input).not.toBeFocused();
  await chooseDockPanelById(page, id);
  await openSplitPanel(page);
  await page
    .getByRole("dialog", { name: "打开面板", exact: true })
    .getByRole("button", { name: "收起「Dock anchor」", exact: true })
    .click();
  await expect(anchor).toBeHidden();
  await expect(root).toBeVisible();

  await page.setViewportSize(desktopViewport);
  await expect(rightTab).toBeVisible();
  await expect(anchor).toBeVisible();
  await expect(root).toBeHidden();
  await rightTab.click();
  await expectRestored();

  await page.reload();
  await expectRestored();
  await root.getByRole("button", { name: /^收起「/ }).click();
  const target = (await anchor.boundingBox())!;
  await tab.dragTo(anchor, { targetPosition: { x: 10, y: target.height / 2 } });
  await expect(tab).toHaveCount(0);
  await expect
    .poll(async () => (await root.boundingBox())!.x)
    .toBeLessThan((await anchor.boundingBox())!.x);
});

test("stacked panes reveal the selected tab and keep keyboard focus across switches", async ({
  page,
  workDir,
}, info) => {
  test.skip(info.project.use.isMobile, "Desktop docking surface");
  const id = await createShell(page, "Stack anchor", workDir);
  const first = page.locator(`[data-dock-panel="${id}"]`);
  await split(first.locator(".split-terminal"), "左右");
  const terminals = page.locator(".split-terminal");
  await expect(terminals).toHaveCount(2);
  const secondId = await terminals.nth(1).getAttribute("data-session-id");
  const second = page.locator(`[data-dock-panel="${secondId}"]`);
  await ready(terminals.nth(1));
  await second.locator("[draggable=true] .dock-title-space").dragTo(first);
  const list = page.locator("[data-dock-panel]:visible [data-dock-stack]");
  const chosen = list.getByRole("tab", { selected: true });
  const firstTitle = (await first.getAttribute("aria-label"))!;
  const secondTitle = (await second.getAttribute("aria-label"))!;
  await expect(chosen).toHaveText(secondTitle);
  await list.getByRole("tab", { name: firstTitle, exact: true }).click();
  await expect(first).toBeVisible();
  await expect(chosen).toHaveText(firstTitle);
  await expect(chosen).toBeFocused();
  await page.keyboard.press("ArrowRight");
  await expect(second).toBeVisible();
  await expect(chosen).toHaveText(secondTitle);
  await expect(chosen).toBeFocused();
  await expect(chosen).toBeInViewport();
  await page.reload();
  await expect(chosen).toHaveText(secondTitle);
});

test("desktop drag guides dock against a pane or the whole workspace", async ({
  page,
  workDir,
}, testInfo) => {
  test.skip(!!testInfo.project.use.isMobile, "Wide docking surface");
  const id = await createShell(page, "Dock target", workDir);
  const terminals = page.locator(".split-terminal");
  await split(page.locator(`[data-session-id="${id}"]`), "左右");
  await expect(terminals).toHaveCount(2);
  const neighborId = await terminals.nth(1).getAttribute("data-session-id");
  await split(terminals.nth(1), "上下");
  await expect(terminals).toHaveCount(3);
  const sourceId = await terminals.nth(2).getAttribute("data-session-id");

  const panel = (panelId: string | null) =>
    page.locator(`[data-dock-panel="${panelId}"]`);
  const target = panel(id);
  const neighbor = panel(neighborId);
  const source = panel(sourceId);
  const panes = [target, neighbor, source];
  await expect(page.locator("[data-dock-panel]:visible")).toHaveCount(3);

  const preview = page.locator("[data-dock-preview]");
  const paneGuides = page.locator("[data-guide-scope=pane]");
  const workspaceGuides = page.locator("[data-guide-scope=workspace]");
  const decorations = page.locator("[data-guide-scope], [data-dock-preview]");
  const innerTop = page.locator("[data-guide-scope=pane][data-guide-edge=top]");
  const outerLeft = page.locator(
    "[data-guide-scope=workspace][data-guide-edge=left]",
  );
  const outerRight = page.locator(
    "[data-guide-scope=workspace][data-guide-edge=right]",
  );
  async function dragOver(locator: Locator) {
    const { x, y, width, height } = (await locator.boundingBox())!;
    // The move that starts a native drag can send dragenter before dragover.
    await page.mouse.move(x + width / 2, y + height / 2);
    await page.mouse.move(x + width / 2, y + height / 2);
  }
  async function startDrag() {
    await source.locator("[draggable=true] .dock-title-space").hover();
    await page.mouse.down();
    await dragOver(source);
  }

  await startDrag();
  await expect(workspaceGuides).toHaveCount(4);
  await expect(paneGuides).toHaveCount(0);
  await expect(preview).toHaveCount(0);

  await dragOver(target);
  await expect(paneGuides).toHaveCount(5);
  const targetBox = (await target.boundingBox())!;
  const center = (await page
    .locator("[data-guide-scope=pane][data-guide-edge=center]")
    .boundingBox())!;
  expect(center.x + center.width / 2).toBeCloseTo(
    targetBox.x + targetBox.width / 2,
    0,
  );
  expect(center.y + center.height / 2).toBeCloseTo(
    targetBox.y + targetBox.height / 2,
    0,
  );
  await dragOver(innerTop);
  await expect(innerTop).toHaveClass(/\bactive\b/);
  await expect(preview).toBeVisible();
  const innerBox = (await preview.boundingBox())!;
  expect(innerBox.x).toBe(targetBox.x);
  expect(innerBox.y).toBe(targetBox.y);
  expect(innerBox.width).toBe(targetBox.width);
  expect(innerBox.height).toBeLessThan(targetBox.height);
  await page.mouse.up();
  await expect(decorations).toHaveCount(0);
  await expect.poll(() => source.boundingBox()).toEqual(innerBox);
  expect((await target.boundingBox())!.y).toBeCloseTo(
    innerBox.y + innerBox.height,
    0,
  );

  await startDrag();
  await dragOver(outerLeft);
  await expect(outerLeft).toHaveClass(/\bactive\b/);
  await expect(preview).toBeVisible();
  const outerBox = (await preview.boundingBox())!;
  const neighborBox = (await neighbor.boundingBox())!;
  expect(outerBox.x).toBe(targetBox.x);
  expect(outerBox.y).toBe(neighborBox.y);
  expect(outerBox.height).toBe(neighborBox.height);
  expect(outerBox.height).toBeGreaterThan(innerBox.height);
  await page.mouse.up();
  await expect(decorations).toHaveCount(0);
  await expect.poll(() => source.boundingBox()).toEqual(outerBox);
  const placed = await Promise.all(panes.map((pane) => pane.boundingBox()));
  expect(placed[0]!.x).toBeCloseTo(outerBox.x + outerBox.width, 0);
  expect(placed[1]!.x).toBeGreaterThan(placed[0]!.x);

  await startDrag();
  await dragOver(outerRight);
  await expect(outerRight).toHaveClass(/\bactive\b/);
  await expect(preview).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(decorations).toHaveCount(0);
  await page.mouse.up();
  await expect
    .poll(() => Promise.all(panes.map((pane) => pane.boundingBox())))
    .toEqual(placed);
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
  await toggleDraftMode(page);
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
    await chooseDockPanel(page, "Primary shell");
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
  await chooseDockPanel(page, "Primary shell");
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
  const menu = page
    .locator(".session-toolbar")
    .getByRole("button", { name: "显示", exact: true });
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

  await chooseDockPanel(page, "Hold preference");
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
  if (!narrowSessionHeader(page))
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

test("Escape from the split chooser returns keyboard focus to its pane", async ({
  page,
  workDir,
}, testInfo) => {
  test.skip(testInfo.project.use.isMobile, "Uses physical keyboard focus");
  const id = await createShell(page, "Split chooser anchor", workDir);
  const pane = page.locator(`[data-session-id="${id}"]`);
  await openSplitPanel(pane);
  await page
    .getByRole("button", { name: "加入已有会话…", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("textbox", { name: "搜索已有会话" }).focus();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(pane.locator(".xterm-helper-textarea")).toBeFocused();
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
  if (!narrowSessionHeader(page))
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
  const chat = page.locator(`[data-dock-panel="${chatId}"]`);
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
  await chooseDockPanel(page, "Shell beside chat");
  await expect(page).toHaveTitle(/Shell beside chat · WebMux$/);
  await expect(page.locator(".split-terminal")).toHaveCount(2);

  // Archiving the hybrid pane removes only that pane; the page stays.
  await chooseDockPanel(page, "Chat agent");
  await (
    await openSessionMenu(page)
  )
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

test("the workspace terminal menu and quick commands follow split pane focus", async ({
  page,
  workDir,
}, info) => {
  test.skip(!!info.project.use.isMobile, "Workspace terminal command bar");
  const id = await createShell(page, "Draft owner", workDir);
  const first = page.locator(`.split-terminal[data-session-id="${id}"]`);
  await split(first, "左右");
  const second = page.locator(".split-terminal").nth(1);
  // Let the split's automatic activation finish before moving focus ourselves.
  await expect(second).toHaveAttribute("data-active", "true");
  await expectConnected(page);
  await expect(second.locator(".restore-overlay")).toBeHidden();

  const firstDraft = first.getByRole("textbox", { name: "草稿内容" });
  const secondDraft = second.getByRole("textbox", { name: "草稿内容" });
  const title = sessionHeader(page).getByRole("button", {
    name: "Draft owner",
    exact: true,
  });

  await first.locator(".xterm-helper-textarea").focus();
  await expect(first).toHaveAttribute("data-active", "true");
  await expect(title).toBeVisible();
  await toggleDraftMode(page);
  await firstDraft.fill("keep this first draft");
  await expect(secondDraft).toBeHidden();

  await second.locator(".xterm-helper-textarea").focus();
  await expect(second).toHaveAttribute("data-active", "true");
  await expect(title).toHaveCount(0);
  await expectDraftMode(page, false);
  await toggleDraftMode(page);
  await sessionHeader(page)
    .getByRole("button", { name: "快捷指令", exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "新会话：/new", exact: true })
    .click();
  await expect(secondDraft).toHaveValue("/new");
  await expect(firstDraft).toHaveValue("keep this first draft");

  await firstDraft.focus();
  await expect(first).toHaveAttribute("data-active", "true");
  await expect(title).toBeVisible();
  await expectDraftMode(page, true);
});

test("same-name panes preserve the upload owner after focus moves", async ({
  page,
  request,
  workDir,
}, info) => {
  test.skip(!!info.project.use.isMobile, "Simultaneously visible split panes");
  const id = await createShell(page, "Review", workDir);
  const response = await request.post("/api/sessions", {
    data: {
      name: "Review",
      cwd: workDir,
      cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
    },
  });
  expect(response.status()).toBe(201);
  const other = (await response.json()).session.id;
  await openSplitPanel(page);
  await page.getByRole("button", { name: "加入已有会话…" }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /^Review \// })
    .click();
  const first = page.locator(`[data-dock-panel="${id}"]`),
    second = page.locator(`[data-dock-panel="${other}"]`);
  await expect(first).toHaveAccessibleName("Review · 1");
  await expect(second).toHaveAccessibleName("Review · 2");
  await first.locator(".dock-title-space").click();
  await expect(first.locator(".split-terminal")).toHaveAttribute(
    "data-active",
    "true",
  );
  const chooser = page.waitForEvent("filechooser");
  await (await uploadAction(page)).click();
  const picker = await chooser;
  await second.locator(".xterm-helper-textarea").focus();
  await expect(second.locator(".split-terminal")).toHaveAttribute(
    "data-active",
    "true",
  );
  const upload = page.waitForResponse((r) => r.url().endsWith("/upload"));
  await picker.setFiles({
    name: "owned.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("first pane"),
  });
  const result = await upload;
  expect(result.status()).toBe(201);
  expect((await result.json()).path).toContain(`/${id}/uploads/`);
  await expect(first.locator(".xterm-rows")).toContainText("owned.txt");
  await expect(second.locator(".xterm-rows")).not.toContainText("owned.txt");
});
