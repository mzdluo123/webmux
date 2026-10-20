import { test, expect } from "@playwright/test";
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
  await page.goto(`${server.baseURL}/#token=${encodeURIComponent(token)}`);
  await expect(
    page
      .locator("header")
      .getByRole("button", { name: "＋ 新建会话", exact: true }),
  ).toBeVisible();
  expect(page.url()).toBe(`${server.baseURL}/`);
  expect(await page.evaluate(() => localStorage.getItem("webmux.token"))).toBe(
    token,
  );
  expect(await page.evaluate(() => document.cookie)).toContain(
    `webmux_token=${encodeURIComponent(token)}`,
  );
  expect(
    await page.evaluate(
      () =>
        new Promise<boolean>((resolve) => {
          const socket = new WebSocket(`ws://${location.host}/ws/sessions`);
          socket.onopen = () => {
            socket.close();
            resolve(true);
          };
          socket.onerror = () => resolve(false);
        }),
    ),
  ).toBe(true);
  await page.goBack();
  expect(page.url()).not.toContain("token");
});

test("an invalid #token= link shows the token form", async ({ page }) => {
  await page.goto(`${server.baseURL}/#token=wrong`);
  await expect(page.getByLabel("共享 Token")).toHaveValue("wrong");
  await expect(page.getByRole("alert")).toHaveText("authentication required");
  expect(page.url()).toBe(`${server.baseURL}/`);
});
