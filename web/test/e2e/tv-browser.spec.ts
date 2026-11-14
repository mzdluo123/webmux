import type { APIRequestContext, Locator, Page } from "@playwright/test";
import {
  expect,
  expectTerminalLine,
  holdTerminalReattach,
  test,
} from "./fixtures";

// Keep the full marker out of readline's redrawable command echo.
const tvCommand = "printf 'TV_%s_MARKER\\n' ONCE; seq 1 120";

async function markerCount(request: APIRequestContext, id: string) {
  const buffer = await (await request.get(`/api/sessions/${id}/buffer`)).text();
  return (buffer.match(/TV_ONCE_MARKER/g) ?? []).length;
}

// Drive actual remote key events, never focus(), locator.press(), Tab or clicks.
// Geometry guides each move; the page, not the test, chooses the next control.
async function remoteTo(page: Page, target: Locator) {
  await expect(target).toBeVisible();
  const tried = new Map<string, Set<string>>();
  for (let step = 0; step < 120; step++) {
    if (await target.evaluate((element) => document.activeElement === element))
      return;
    const destination = await target.boundingBox();
    if (!destination) throw new Error("TV destination has no rectangle");
    const active = await page.evaluate(() => {
      const element = document.activeElement;
      const rect = element?.getBoundingClientRect();
      const state = window as typeof window & {
        __tvRemoteNodes?: WeakMap<Element, string>;
      };
      const nodes = (state.__tvRemoteNodes ??= new WeakMap<Element, string>());
      const key = element
        ? (nodes.get(element) ?? crypto.randomUUID())
        : "body";
      if (element) nodes.set(element, key);
      return {
        key,
        name:
          element?.getAttribute("aria-label") ||
          (element?.matches("button, a, [role=menuitem], [role=radio]")
            ? element.textContent?.trim()
            : element?.tagName) ||
          "body",
        x: rect ? rect.x + rect.width / 2 : 0,
        y: rect ? rect.y + rect.height / 2 : 0,
        left: rect?.left ?? 0,
        right: rect?.right ?? 0,
        top: rect?.top ?? 0,
        bottom: rect?.bottom ?? 0,
        menu: !!element?.closest('[role="menu"], [role="listbox"]'),
      };
    });
    const dx = destination.x + destination.width / 2 - active.x;
    const dy = destination.y + destination.height / 2 - active.y;
    const distanceX = Math.max(
      0,
      destination.x - active.right,
      active.left - destination.x - destination.width,
    );
    const distanceY = Math.max(
      0,
      destination.y - active.bottom,
      active.top - destination.y - destination.height,
    );
    const preferVertical =
      distanceX === distanceY
        ? Math.abs(dy) > Math.abs(dx)
        : distanceY > distanceX;
    const horizontal = dx < 0 ? "ArrowLeft" : "ArrowRight";
    const vertical = dy < 0 ? "ArrowUp" : "ArrowDown";
    const preferred = active.menu
      ? [vertical, horizontal]
      : preferVertical
        ? [vertical, horizontal]
        : [horizontal, vertical];
    const visited = tried.get(active.key) ?? new Set<string>();
    const key = [
      ...preferred,
      "ArrowDown",
      "ArrowRight",
      "ArrowUp",
      "ArrowLeft",
    ].find((key) => !visited.has(key));
    if (!key)
      throw new Error(
        `Remote focus cannot reach ${(await target.getAttribute("aria-label")) ?? (await target.textContent())} from ${active.name}`,
      );
    visited.add(key);
    tried.set(active.key, visited);
    await page.keyboard.press(key);
  }
  throw new Error("TV remote navigation did not settle");
}

async function select(page: Page, target: Locator) {
  await remoteTo(page, target);
  await page.keyboard.press("Enter");
}

for (const width of [1280, 1920]) {
  test(`TV remote creates, sends once, releases capture and reconnects at ${width}`, async ({
    page,
    request,
  }, info) => {
    test.skip(!!info.project.use.isMobile, "TV large-screen remote workflow");
    await page.setViewportSize({ width, height: width === 1280 ? 720 : 1080 });
    await page.addInitScript(() => {
      localStorage.setItem("webmux.tv-mode", "1");
      localStorage.setItem("webmux.lastLaunch", "custom");
      localStorage.setItem(
        "webmux.customCommand",
        "/bin/bash --noprofile --norc -i",
      );
    });
    const prefs = await (await request.get("/api/settings")).json();
    expect(
      (
        await request.patch("/api/settings", {
          data: {
            baseRevision: prefs.revision,
            changes: {
              quickCommands: [
                {
                  id: "tv-output",
                  name: "TV 输出",
                  command: tvCommand,
                  enter: true,
                },
              ],
            },
          },
        })
      ).ok(),
    ).toBe(true);
    const reconnect = await holdTerminalReattach(page);
    await page.goto("/");
    await expect(page.locator("html")).toHaveAttribute("data-ui-mode", "tv");
    const create = page
      .getByRole("main")
      .getByRole("button", { name: "＋ 新建会话", exact: true })
      .first();
    await select(page, create);
    await expect(page.getByRole("dialog")).toBeVisible();
    await select(
      page,
      page.getByRole("button", { name: "创建并进入 →", exact: true }),
    );
    await expect(page).toHaveURL(/\/s\/[a-z0-9]+$/);
    const id = new URL(page.url()).pathname.split("/").at(-1)!;
    await expectTerminalLine(page, "webmux-e2e$");
    const menu = page.getByRole("button", { name: "终端菜单", exact: true });
    await select(page, menu);
    await remoteTo(
      page,
      page.getByRole("menuitem", { name: "快捷指令", exact: true }),
    );
    await page.keyboard.press("ArrowRight");
    await select(page, page.getByRole("menuitem", { name: /TV 输出/ }));
    await expectTerminalLine(page, "120");
    expect(await markerCount(request, id)).toBe(1);
    const firstRow = page.locator(".xterm-rows > div").first();
    const latest = Number(await firstRow.textContent());
    await select(
      page,
      page.getByRole("button", { name: "向上滚动", exact: true }),
    );
    await expect
      .poll(async () => Number(await firstRow.textContent()))
      .toBeLessThan(latest);
    await select(
      page,
      page.getByRole("button", { name: "向下滚动", exact: true }),
    );
    await expect
      .poll(async () => Number(await firstRow.textContent()))
      .toBe(latest);
    await select(
      page,
      page.getByRole("button", { name: "终端输入", exact: true }),
    );
    await page.keyboard.type("printf 'TV_KEYBOARD_OK\\n'");
    await page.keyboard.press("Enter");
    await expectTerminalLine(page, "TV_KEYBOARD_OK");
    await page.keyboard.press("Escape");
    await expect(
      page.getByRole("button", { name: "终端输入", exact: true }),
    ).toHaveAttribute("aria-pressed", "false");
    await expect(page).toHaveURL(new RegExp(`/s/${id}$`));
    await select(page, menu);
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menu")).toHaveCount(0);
    await expect(page).toHaveURL(new RegExp(`/s/${id}$`));
    await reconnect.drop();
    reconnect.release();
    await expectTerminalLine(page, "TV_KEYBOARD_OK");
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-ui-mode", "tv");
    await expectTerminalLine(page, "TV_KEYBOARD_OK");
    expect(
      (await (await request.get("/api/sessions")).json()).map(
        (session: { id: string }) => session.id,
      ),
    ).toEqual([id]);
    await select(page, menu);
    await select(
      page,
      page.getByRole("menuitemcheckbox", { name: /^草稿模式/ }),
    );
    await expect(page.getByRole("textbox", { name: "草稿内容" })).toBeVisible();
    if (
      await page.evaluate(() =>
        document.activeElement?.matches("input, textarea"),
      )
    )
      await page.keyboard.press("Escape");
    await select(page, menu);
    await remoteTo(
      page,
      page.getByRole("menuitem", { name: "快捷指令", exact: true }),
    );
    await page.keyboard.press("ArrowRight");
    await select(page, page.getByRole("menuitem", { name: /TV 输出/ }));
    const draft = page.getByRole("textbox", { name: "草稿内容" });
    await expect.poll(() => draft.inputValue()).toContain(tvCommand);
    const staged = await draft.inputValue();
    if (
      await page.evaluate(() =>
        document.activeElement?.matches("input, textarea"),
      )
    )
      await page.keyboard.press("Escape");
    await select(
      page,
      page.getByRole("button", { name: "清空草稿", exact: true }),
    );
    await expect(page.getByRole("alertdialog")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("alertdialog")).toHaveCount(0);
    await expect(draft).toHaveValue(staged);
    expect(await markerCount(request, id)).toBe(1);
    await select(page, page.getByRole("button", { name: "发送", exact: true }));
    await expect(draft).not.toBeVisible();
    await expect.poll(() => markerCount(request, id)).toBe(2);
    const second = await request.post("/api/sessions", {
      data: {
        cmd: ["/bin/bash", "--noprofile", "--norc", "-i"],
        name: "TV existing session",
      },
    });
    expect(second.status()).toBe(201);
    const secondId = (await second.json()).session.id;
    await select(
      page,
      page.getByRole("button", { name: "快速切换会话", exact: true }),
    );
    const switcher = page.getByRole("dialog", {
      name: "快速切换会话",
      exact: true,
    });
    const selected = switcher.locator(
      '[data-slot="command-item"][data-selected]',
    );
    for (
      let step = 0;
      step < 4 &&
      !(await selected.allTextContents())
        .join("")
        .includes("TV existing session");
      step++
    ) {
      await page.keyboard.press("ArrowDown");
    }
    await expect(selected).toContainText("TV existing session");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/s/${secondId}$`));
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  });
}
