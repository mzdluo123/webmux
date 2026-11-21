import {
  test,
  expect,
  createShell,
  runCommand,
  expectTerminalLine,
  openSessionQueue,
  workspaceRefreshButton,
} from "./fixtures";
import { devices } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

test("isolated desktop and phone synchronize preferences in both directions", async ({
  browser,
  server,
  request,
}) => {
  const desktop = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    hasTouch: false,
    isMobile: false,
  });
  const phone = await browser.newContext({ ...devices["Pixel 7"] });
  try {
    const a = await desktop.newPage();
    const b = await phone.newPage();
    await Promise.all([
      a.goto(server.baseURL + "/settings"),
      b.goto(server.baseURL + "/settings"),
    ]);
    await expect(a.getByText("已连接 · 设置已同步")).toBeVisible();
    await expect(b.getByText("已连接 · 设置已同步")).toBeVisible();
    await a.getByRole("radio", { name: "深色", exact: true }).click();
    await expect(a.locator("html")).toHaveClass(/dark/);
    await expect(b.locator("html")).not.toHaveClass(/dark/);
    await a.getByLabel("终端字号", { exact: true }).fill("18");
    await a.getByLabel("终端字号", { exact: true }).press("Tab");
    await expect(a.getByLabel("终端字号", { exact: true })).toHaveValue("18");
    await expect(b.getByLabel("终端字号", { exact: true })).toHaveValue("15");
    await b.getByRole("radio", { name: "浅色", exact: true }).click();
    await expect(b.locator("html")).not.toHaveClass(/dark/);
    await expect(a.locator("html")).toHaveClass(/dark/);
    await expect(
      a.getByRole("switch", { name: "显示底部工具栏" }),
    ).not.toBeChecked();
    await expect(
      b.getByRole("switch", { name: "显示底部工具栏" }),
    ).toBeChecked();
    await a.getByRole("switch", { name: "显示底部工具栏" }).click();
    await b.getByRole("switch", { name: "显示底部工具栏" }).click();
    await expect(
      a.getByRole("switch", { name: "显示底部工具栏" }),
    ).toBeChecked();
    await expect(
      b.getByRole("switch", { name: "显示底部工具栏" }),
    ).not.toBeChecked();
    await b.getByRole("switch", { name: "Diff 自动换行" }).click();
    await expect(
      a.getByRole("switch", { name: "Diff 自动换行" }),
    ).toBeChecked();
    await a
      .locator("#quick-commands")
      .getByRole("button", { name: "添加快捷指令", exact: true })
      .click();
    const dialog = a.getByRole("dialog", { name: "添加快捷指令" });
    await dialog.getByLabel("名称", { exact: true }).fill("共享回显");
    await dialog.getByLabel("指令", { exact: true }).fill("echo shared");
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(
      b.getByRole("button", { name: "编辑 共享回显" }),
    ).toBeVisible();
    const serverValues = (await (await request.get("/api/settings")).json())
      .values;
    expect(serverValues).not.toHaveProperty("theme");
    expect(serverValues).not.toHaveProperty("fontSize");
    expect(serverValues).not.toHaveProperty("forceSoftKeys");
    await server.restart();
    await a.reload();
    await b.reload();
    await expect(a.locator("html")).toHaveClass(/dark/);
    await expect(b.locator("html")).not.toHaveClass(/dark/);
    await expect(a.getByLabel("终端字号", { exact: true })).toHaveValue("18");
    await expect(b.getByLabel("终端字号", { exact: true })).toHaveValue("15");
    await expect(
      a.getByRole("switch", { name: "显示底部工具栏" }),
    ).toBeChecked();
    await expect(
      b.getByRole("switch", { name: "显示底部工具栏" }),
    ).not.toBeChecked();
    await expect(
      b.getByRole("button", { name: "编辑 共享回显" }),
    ).toBeVisible();
    expect(
      await b.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  } finally {
    await desktop.close();
    await phone.close();
  }
});

test("legacy browser keys never overwrite server preferences", async ({
  page,
  request,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem("webmux.theme", "dark");
    localStorage.setItem("webmux.quickCommands", "[]");
    localStorage.setItem("webmux.settingsCache", "bad-cache");
    localStorage.setItem("webmux.tv-mode", "1");
    localStorage.setItem("webmux.headsetMode", "1");
  });
  await page.goto("/settings");
  await expect(page.getByText("已连接 · 设置已同步")).toBeVisible();
  await expect(page.locator("html")).toHaveClass(/\bdark\b/);
  await expect(page.locator("html")).not.toHaveAttribute("data-ui-mode", "tv");
  await expect(page.locator("html")).not.toHaveAttribute(
    "data-headset-mode",
    "true",
  );
  await expect(page.locator("#tv-mode, #headset-mode")).toHaveCount(0);
  const snapshot = await (await request.get("/api/settings")).json();
  expect(snapshot.values).not.toHaveProperty("theme");
  expect(snapshot.values.quickCommands).toBeNull();
  await expect(
    page.getByRole("button", { name: "编辑 恢复会话" }),
  ).toBeVisible();
});

test("corrupt settings preserve bytes while terminal APIs remain available", async ({
  server,
  page,
  request,
}) => {
  const path = join(server.stateDir, "settings.json");
  const prior = await readFile(path);
  try {
    await writeFile(path, "{corrupt-private-body");
    await server.restart();
    await page.goto("/settings");
    await expect(page.locator("#appearance").getByRole("alert")).toContainText(
      "settings.json",
    );
    await expect(page.getByRole("switch", { name: "代码行号" })).toBeDisabled();
    await expect(
      page.getByRole("switch", { name: "显示底部工具栏" }),
    ).toBeEnabled();
    await expect(
      page.getByRole("radio", { name: "深色", exact: true }),
    ).toBeEnabled();
    await page.getByRole("radio", { name: "深色", exact: true }).click();
    await expect(page.locator("html")).toHaveClass(/\bdark\b/);
    expect((await request.get("/api/settings")).status()).toBe(503);
    expect((await request.get("/api/health")).status()).toBe(200);
    expect(await readFile(path, "utf8")).toBe("{corrupt-private-body");
    await createShell(page, "settings unavailable shell");
    await runCommand(page, "printf 'TERMINAL_SURVIVES_FAILURE\\n'");
    await expectTerminalLine(page, "TERMINAL_SURVIVES_FAILURE");
  } finally {
    await writeFile(path, prior);
    await server.restart();
  }
});

test("loading and refreshing the home page never rewrites shared pins", async ({
  page,
  request,
}) => {
  const before = await (await request.get("/api/settings")).json();
  const homeWorktrees = { unavailable: ["saved-pin"] };
  const seeded = await (
    await request.patch("/api/settings", {
      data: { baseRevision: before.revision, changes: { homeWorktrees } },
    })
  ).json();
  const writes: string[] = [];
  page.on("request", (req) => {
    if (req.method() === "PATCH" && req.url().endsWith("/api/settings"))
      writes.push(req.url());
  });
  // Phones show the scope as the top bar title, so wait for the page itself.
  const refresh = workspaceRefreshButton(page);
  await page.goto("/");
  await openSessionQueue(page);
  await refresh.waitFor();
  await page.reload();
  await openSessionQueue(page);
  await refresh.waitFor();
  await expect(refresh).toHaveAttribute("aria-busy", "false");
  const refreshed = page.waitForResponse(
    (response) => response.url().endsWith("/api/repositories") && response.ok(),
  );
  await refresh.click();
  await refreshed;
  await expect(refresh).toHaveAttribute("aria-busy", "false");
  const after = await (await request.get("/api/settings")).json();
  expect(after.revision).toBe(seeded.revision);
  expect(after.values.homeWorktrees).toEqual(homeWorktrees);
  expect(writes).toEqual([]);
});
