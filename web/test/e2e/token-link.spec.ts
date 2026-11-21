import { test, expect } from "@playwright/test";
import { useWorkbenchBranches, workspaceRefreshButton } from "./fixtures";
import { startE2EServer, type E2EServer } from "./server.mjs";

const token = "e2e a&b#c";
let server: E2EServer;

test.beforeAll(async () => {
  server = await startE2EServer({ token });
});

test.afterAll(async () => {
  await server?.close();
});

test("a #token= link signs in and leaves no token in the address or history", async ({
  page,
}) => {
  // Listen before navigating: the snapshot frame arrives with the page.
  const sessions = new Promise<string>((resolve) =>
    page.on("websocket", (socket) => {
      if (new URL(socket.url()).pathname === "/ws/sessions")
        socket.once("framereceived", () => resolve(socket.url()));
    }),
  );
  await page.goto(`${server.baseURL}/#token=${encodeURIComponent(token)}`);
  await expect(page.getByRole("textbox", { name: "任务描述" })).toBeVisible();
  expect(page.url()).toBe(`${server.baseURL}/`);
  expect(await page.evaluate(() => localStorage.getItem("webmux.token"))).toBe(
    token,
  );
  // The app's own socket authenticates through its subprotocol: no token in
  // the URL and no cookie, which every port of the host would share.
  expect(await sessions).not.toContain("token");
  expect(await page.evaluate(() => document.cookie)).not.toContain(
    "webmux_token",
  );
  expect(
    await page.evaluate(
      () =>
        new Promise<boolean>((resolve) => {
          const bare = new WebSocket(`ws://${location.host}/ws/sessions`);
          bare.onopen = () => {
            bare.close();
            resolve(true);
          };
          bare.onerror = () => resolve(false);
        }),
    ),
  ).toBe(false);
  await page.goBack();
  expect(page.url()).not.toContain("token");
});

test("an invalid #token= link shows the token form", async ({ page }) => {
  await page.goto(`${server.baseURL}/#token=wrong`);
  await expect(page.getByLabel("共享 Token", { exact: true })).toHaveValue(
    "wrong",
  );
  await expect(page.getByRole("alert")).toBeVisible();
  expect(page.url()).toBe(`${server.baseURL}/`);
});

test("a mid-session 401 clears workspace state and allows signing in again", async ({
  page,
}) => {
  await page.goto(`${server.baseURL}/#token=${encodeURIComponent(token)}`);
  if (await useWorkbenchBranches(page)) await page.keyboard.press("Escape");
  await expect(page.getByRole("searchbox", { name: "搜索会话" })).toBeVisible();
  await page.getByRole("searchbox", { name: "搜索会话" }).fill("discard me");
  await page.route("**/api/repositories", async (route) => {
    await route.fulfill({ status: 401, json: { error: "token expired" } });
  });
  await workspaceRefreshButton(page).click();
  await expect(page.getByLabel("共享 Token", { exact: true })).toBeVisible();
  await expect(page.getByRole("alert")).toBeVisible();
  await expect(page.getByRole("searchbox", { name: "搜索会话" })).toHaveCount(
    0,
  );
  await page.unroute("**/api/repositories");
  await page.getByLabel("共享 Token", { exact: true }).fill(token);
  await page.getByRole("button", { name: "连接 WebMux →" }).click();
  const search = page.getByRole("searchbox", { name: "搜索会话" });
  await expect(search).toBeVisible();
  await expect(search).toHaveValue("");
});

test("returning browser reloads into the shell skeleton without flashing the login card", async ({
  page,
}) => {
  await page.goto(`${server.baseURL}/#token=${encodeURIComponent(token)}`);
  await expect(page.getByRole("textbox", { name: "任务描述" })).toBeVisible();

  // Observe every DOM mutation across reload to prove the login headline never flashes.
  await page.addInitScript(() => {
    const win = window as unknown as { __sawLoginHeadline?: boolean };
    win.__sawLoginHeadline = false;
    const scan = () => {
      const text = document.body?.textContent ?? "";
      if (
        text.includes("你的终端，始终在线。") ||
        text.includes("Your terminal, always online.")
      ) {
        win.__sawLoginHeadline = true;
      }
    };
    const observer = new MutationObserver(scan);
    const start = () => {
      scan();
      if (document.body)
        observer.observe(document.body, {
          childList: true,
          subtree: true,
          characterData: true,
        });
    };
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", start, { once: true });
    } else {
      start();
    }
  });

  const holdHealth = Promise.withResolvers<void>();
  let healthRequested = false;
  await page.route("**/api/health", async (route) => {
    healthRequested = true;
    await holdHealth.promise;
    await route.continue();
  });

  const reload = page.reload({ waitUntil: "domcontentloaded" });
  await expect.poll(() => healthRequested).toBe(true);
  await expect(
    page.getByRole("main", { name: "正在恢复工作区" }),
  ).toBeVisible();
  await expect(page.getByText("你的终端，始终在线。")).toHaveCount(0);

  holdHealth.resolve();
  await reload;
  await expect(page.getByRole("textbox", { name: "任务描述" })).toBeVisible();
  expect(
    await page.evaluate(
      () =>
        (window as unknown as { __sawLoginHeadline?: boolean })
          .__sawLoginHeadline,
    ),
  ).toBe(false);
});
