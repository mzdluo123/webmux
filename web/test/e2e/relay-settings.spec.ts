import { test, expect } from "./fixtures";
import { relayStatus } from "../fixtures/relay";
import type { RelayStatus } from "../../src/contracts/relay-schemas";

test.use({ features: [] });

test("pair, enable, reconnect, rebind and delete through the Relay settings page", async ({
  page,
}) => {
  const publicUrl = "https://fixture.hosts.example.test";
  const handle = "A".repeat(43);
  const userCode = "ABCD-EFGH";
  const requests: { path: string; body: unknown }[] = [];
  let status: RelayStatus = { ...relayStatus };
  let polls = 0;

  const paired = (): RelayStatus => ({
    ...relayStatus,
    paired: true,
    ownerId: "123",
    ownerLogin: "relay-e2e",
    boundPublicUrl: publicUrl,
    controlReachability: "reachable",
    connectorUrl: "wss://data.example.test/v1/connector",
    dataOrigin: publicUrl,
    assignmentSource: "authenticated-control",
    assignmentRevision: "1",
    configRevision: "1",
    appliedRevision: "1",
    nodeId: "fixture-node",
    capabilities: ["wss"],
  });
  const connected = (): RelayStatus => ({
    ...paired(),
    enabled: true,
    state: "ready",
    controlReady: true,
    bulkReady: true,
    readyConnections: 2,
    publicUrl,
    dataReachability: "reachable",
  });

  // Only the Relay API is simulated. The app, routing, authentication bootstrap,
  // dialogs and controller run against the fixture's isolated real WebMux.
  await page.route("**/api/relay**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    if (path === "/api/relay/auth/session") {
      await route.fulfill({
        json: { authenticated: false, csrfToken: "", expiresAt: 0 },
      });
      return;
    }
    if (method === "GET" && path === "/api/relay") {
      await route.fulfill({ json: status });
      return;
    }
    if (method !== "GET") {
      requests.push({ path, body: request.postDataJSON() });
    }
    if (method === "POST" && path === "/api/relay/enrollments") {
      polls = 0;
      status = { ...status, enabled: false, publicUrl: "", pairing: true };
      await route.fulfill({
        status: 201,
        json: {
          version: 1,
          handle,
          userCode,
          verificationUrl: "https://github.com/login/device",
          expiresAt: Math.floor(Date.now() / 1000) + 600,
          pollIntervalSeconds: 1,
        },
      });
      return;
    }
    if (path === "/api/relay/enrollments/poll") {
      if (++polls === 1) {
        await route.fulfill({ status: 202, json: { error: "pending" } });
      } else {
        status = paired();
        await route.fulfill({ json: status });
      }
      return;
    }
    if (method === "PUT" && path === "/api/relay") {
      status = connected();
    } else if (path === "/api/relay/reconnect") {
      status = connected();
    } else if (path === "/api/relay/disconnect") {
      status = paired();
    } else if (method === "DELETE" && path === "/api/relay/binding") {
      status = { ...relayStatus };
    } else {
      await route.continue();
      return;
    }
    await route.fulfill({ json: status });
  });

  await page.goto("/settings/relay");
  const access = page.getByRole("switch", { name: "允许 Relay 公网访问" });
  const confirm = page.getByRole("alertdialog");
  await expect(access).toBeDisabled();
  await page
    .getByRole("button", { name: "开始 GitHub 配对", exact: true })
    .click();
  await expect(confirm).toContainText("能够读取转发内容");
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(page.getByLabel("GitHub 配对码")).toHaveText(userCode);
  await expect(page.getByText("GitHub 所有者：relay-e2e · 123")).toBeVisible();
  await expect(access).not.toBeChecked();

  await access.click();
  await expect(confirm).toContainText("启用公网 Shell 访问");
  await confirm.getByRole("button", { name: "确认启用", exact: true }).click();
  await expect(access).toBeChecked();
  await expect(
    page.getByRole("link", { name: "通过所有者账号打开 WebMux", exact: true }),
  ).toHaveAttribute("href", publicUrl + "/auth/relay/start");
  expect(requests.find((item) => item.path === "/api/relay")?.body).toEqual({
    enabled: true,
    confirmPublic: true,
  });

  await page.getByRole("button", { name: "重新连接", exact: true }).click();
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect
    .poll(() => requests.some((item) => item.path.endsWith("/reconnect")))
    .toBe(true);

  await access.click();
  await expect(confirm).toContainText("持久终端进程不会退出");
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(access).not.toBeChecked();
  await expect(
    page.getByRole("link", { name: "通过所有者账号打开 WebMux", exact: true }),
  ).toHaveCount(0);

  await page
    .getByRole("button", { name: "重新绑定 GitHub", exact: true })
    .click();
  await expect(confirm).toContainText("保留固定公共地址");
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(page.getByLabel("GitHub 配对码")).toHaveText(userCode);
  await expect(
    page.getByRole("button", { name: "重新绑定 GitHub", exact: true }),
  ).toBeVisible();
  expect(
    requests
      .filter((item) => item.path === "/api/relay/enrollments")
      .map((item) => item.body),
  ).toEqual([
    { rebind: false, confirmPublic: true },
    { rebind: true, confirmPublic: true },
  ]);
  const saved = await page.evaluate(() => [
    ...Object.values(localStorage),
    ...Object.values(sessionStorage),
    document.cookie,
  ]);
  expect(saved.join("\n")).not.toContain(handle);
  expect(saved.join("\n")).not.toContain(userCode);

  await page.getByRole("button", { name: "删除绑定", exact: true }).click();
  await expect(confirm).toContainText("远端确认撤销后才删除本地绑定");
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "开始 GitHub 配对", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("GitHub 所有者：relay-e2e · 123")).toHaveCount(0);
});

test("recover a server binding when this installation is locally unpaired", async ({
  page,
}) => {
  let status: RelayStatus = { ...relayStatus };
  const enrollmentRequests: unknown[] = [];
  let recovering = false;
  let transientFailure = true;
  let cancelFailure = true;
  let cancelled = false;
  await page.route("**/api/relay**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path === "/api/relay/auth/session") {
      await route.fulfill({
        json: { authenticated: false, csrfToken: "", expiresAt: 0 },
      });
    } else if (request.method() === "GET" && path === "/api/relay") {
      await route.fulfill({ json: status });
    } else if (
      request.method() === "DELETE" &&
      path === "/api/relay/enrollments"
    ) {
      if (cancelFailure) {
        cancelFailure = false;
        await route.fulfill({ status: 503, json: { error: "unavailable" } });
        return;
      }
      cancelled = true;
      status = { ...status, pairing: false };
      await route.fulfill({ status: 204 });
    } else if (path === "/api/relay/enrollments") {
      const body: unknown = request.postDataJSON();
      enrollmentRequests.push(body);
      recovering =
        typeof body === "object" &&
        body !== null &&
        "rebind" in body &&
        body.rebind === true;
      status = { ...status, pairing: true };
      await route.fulfill({
        status: 201,
        json: {
          version: 1,
          handle: "A".repeat(43),
          userCode: "REST-ORE1",
          verificationUrl: "https://github.com/login/device",
          expiresAt: Math.floor(Date.now() / 1000) + 600,
          pollIntervalSeconds: 1,
        },
      });
    } else if (path === "/api/relay/enrollments/poll") {
      if (!recovering) {
        if (transientFailure) {
          transientFailure = false;
          await route.fulfill({ status: 503, json: { error: "unavailable" } });
        } else {
          status = { ...status, pairing: false };
          await route.fulfill({ status: 409, json: { error: "conflict" } });
        }
        return;
      }
      status = {
        ...relayStatus,
        paired: true,
        ownerId: "123",
        ownerLogin: "recovered-owner",
        boundPublicUrl: "https://original.hosts.example.test",
      };
      await route.fulfill({ json: status });
    } else {
      await route.continue();
    }
  });

  await page.goto("/settings/relay");
  const confirm = page.getByRole("alertdialog");
  await page
    .getByRole("button", { name: "开始 GitHub 配对", exact: true })
    .click();
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  // A transport failure retains the handle, so the user can cancel it.
  const cancel = page.getByRole("button", { name: "取消配对", exact: true });
  await expect(page.getByRole("alert")).toBeVisible();
  await expect(cancel).toBeVisible();
  await cancel.click();
  await expect(cancel).toBeEnabled();
  await cancel.click();
  await expect.poll(() => cancelled).toBe(true);
  await page
    .getByRole("button", { name: "刷新 Relay 状态", exact: true })
    .click();
  await page
    .getByRole("button", { name: "开始 GitHub 配对", exact: true })
    .click();
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  const recover = page.getByRole("button", {
    name: "恢复已有绑定",
    exact: true,
  });
  await expect(recover).toBeVisible();
  await recover.click();
  await expect(confirm).toContainText("撤销旧密钥和登录访问");
  await confirm.getByRole("button", { name: "取消", exact: true }).click();
  expect(enrollmentRequests).toEqual([
    { rebind: false, confirmPublic: true },
    { rebind: false, confirmPublic: true },
  ]);
  await recover.click();
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(
    page.getByText("GitHub 所有者：recovered-owner · 123"),
  ).toBeVisible();
  await expect(
    page.getByText("https://original.hosts.example.test"),
  ).toBeVisible();
  await expect(
    page.getByRole("switch", { name: "允许 Relay 公网访问" }),
  ).not.toBeChecked();
  expect(enrollmentRequests).toEqual([
    { rebind: false, confirmPublic: true },
    { rebind: false, confirmPublic: true },
    { rebind: true, confirmPublic: true },
  ]);
});
