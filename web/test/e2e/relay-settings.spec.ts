import { test, expect } from "./fixtures";
import { relayStatus } from "../fixtures/relay";
import type {
  RelayStatus,
  RelayPairing,
} from "../../src/contracts/relay-schemas";
import type { Page } from "@playwright/test";

const publicUrl = "https://fixture.hosts.example.test";
const idle = (): RelayPairing => ({
  phase: "idle",
  userCode: "",
  verificationUrl: "",
  expiresAt: 0,
  pollIntervalSeconds: 0,
  retryAt: 0,
  error: "",
});
const waiting = (): RelayPairing => ({
  ...idle(),
  phase: "waiting",
  userCode: "ABCD-EFGH",
  verificationUrl: "https://github.com/login/device",
  expiresAt: Math.floor(Date.now() / 1000) + 600,
  pollIntervalSeconds: 1,
});
const paired = (): RelayStatus => ({
  ...relayStatus,
  paired: true,
  ownerId: "123",
  ownerLogin: "relay-e2e",
  boundPublicUrl: publicUrl,
});

async function relayFixture(page: Page, initial = relayStatus) {
  const model = {
    status: { ...initial },
    pairing: idle(),
    mutations: [] as { method: string; path: string; body: unknown }[],
    loseCreateResponse: false,
    loseMutationResponse: false,
    failReads: false,
  };
  // The real app, routing and authentication bootstrap use an isolated server.
  // Only Relay HTTP is simulated; no production pairing or settings writes.
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
    if (method === "GET") {
      if (model.failReads) {
        await route.fulfill({ status: 503, json: { error: "unavailable" } });
      } else {
        await route.fulfill({
          json:
            path === "/api/relay/enrollments" ? model.pairing : model.status,
        });
      }
      return;
    }
    model.mutations.push({ method, path, body: request.postDataJSON() });
    if (path === "/api/relay/enrollments") {
      model.pairing =
        method === "POST" ? waiting() : { ...idle(), phase: "cancelled" };
      model.status = { ...model.status, pairing: method === "POST" };
      if (model.loseCreateResponse) {
        model.loseCreateResponse = false;
        await route.abort("failed");
      } else {
        await route.fulfill({
          status: method === "POST" ? 201 : 200,
          json: model.pairing,
        });
      }
      return;
    }
    const kind = path.endsWith("/binding")
      ? "delete"
      : path.endsWith("/disconnect")
        ? "disable"
        : path.endsWith("/reconnect")
          ? "reconnect"
          : "enable";
    model.status = kind === "delete" ? { ...relayStatus } : paired();
    if (kind === "enable" || kind === "reconnect") {
      model.status = {
        ...model.status,
        enabled: true,
        state: "ready",
        controlReady: true,
        bulkReady: true,
        readyConnections: 2,
        publicUrl,
      };
    }
    model.status.operation = {
      kind,
      phase: "completed",
      error: "",
      deleted: kind === "delete",
      accessClosed: false,
    };
    if (kind === "delete") model.pairing = idle();
    if (model.loseMutationResponse) {
      model.loseMutationResponse = false;
      await route.abort("failed");
    } else {
      await route.fulfill({ json: model.status });
    }
  });
  return model;
}

test("pairing survives reload and navigation; temporary failures recover without recreation", async ({
  page,
}) => {
  const model = await relayFixture(page);
  await page.goto("/settings/relay");
  await expect(
    page.getByRole("switch", { name: "允许 Relay 公网访问" }),
  ).toHaveCount(0);
  await page
    .getByRole("button", { name: "使用 GitHub 配对", exact: true })
    .click();
  const code = page.getByLabel("GitHub 配对码");
  await expect(code).toHaveText("ABCD-EFGH");
  await expect(
    page.getByRole("region", { name: "当前浏览器的实际交互路径", exact: true }),
  ).not.toBeVisible();
  await expect(
    page.getByRole("link", { name: "前往 GitHub 授权", exact: true }),
  ).toHaveAttribute("href", "https://github.com/login/device");
  await page.reload();
  await expect(code).toHaveText("ABCD-EFGH");
  await page.goto("/settings");
  await page.goto("/settings/relay");
  await expect(code).toHaveText("ABCD-EFGH");
  expect(model.mutations).toEqual([
    {
      method: "POST",
      path: "/api/relay/enrollments",
      body: { confirmPublic: true },
    },
  ]);

  model.pairing = {
    ...model.pairing,
    phase: "retrying",
    error: "unavailable",
    retryAt: Math.floor(Date.now() / 1000) + 1,
  };
  await page.waitForResponse(
    async (response) =>
      new URL(response.url()).pathname === "/api/relay/enrollments" &&
      response.request().method() === "GET" &&
      (await response.json()).phase === "retrying",
  );
  await expect(
    page.getByRole("button", { name: "取消配对", exact: true }),
  ).toBeEnabled();
  await expect(code).toHaveText("ABCD-EFGH");
  model.failReads = true;
  await expect(
    page.getByRole("button", { name: "检查当前状态", exact: true }),
  ).toBeVisible();
  model.failReads = false;
  model.status = paired();
  model.pairing = { ...idle(), phase: "paired" };
  await page.getByRole("button", { name: "检查当前状态", exact: true }).click();
  const access = page.getByRole("switch", { name: "允许 Relay 公网访问" });
  await expect(access).not.toBeChecked();
  await expect(
    page.getByText("GitHub 账号：relay-e2e", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(code).toHaveCount(0);
  expect(model.mutations).toHaveLength(1);
  const saved = await page.evaluate(() => [
    ...Object.values(localStorage),
    ...Object.values(sessionStorage),
    document.cookie,
  ]);
  expect(saved.join("\n")).not.toContain("ABCD-EFGH");
});

test("lost create response reconciles existing attempt and explicit cancel permits a new attempt", async ({
  page,
}) => {
  const model = await relayFixture(page);
  model.loseCreateResponse = true;
  await page.goto("/settings/relay");
  await page
    .getByRole("button", { name: "使用 GitHub 配对", exact: true })
    .click();
  await expect(page.getByLabel("GitHub 配对码")).toHaveText("ABCD-EFGH");
  expect(
    model.mutations.filter((entry) => entry.method === "POST"),
  ).toHaveLength(1);
  await page.getByRole("button", { name: "取消配对", exact: true }).click();
  await expect(page.getByLabel("GitHub 配对码")).toHaveCount(0);
  await page
    .getByRole("button", { name: "使用 GitHub 配对", exact: true })
    .click();
  await expect(page.getByLabel("GitHub 配对码")).toHaveText("ABCD-EFGH");
  expect(model.mutations.map((entry) => entry.method)).toEqual([
    "POST",
    "DELETE",
    "POST",
  ]);
});

test("enable requires consent and deleted binding does not claim old access is closed", async ({
  page,
}) => {
  const model = await relayFixture(page, paired());
  await page.goto("/settings/relay");
  const access = page.getByRole("switch", { name: "允许 Relay 公网访问" });
  const confirm = page.getByRole("alertdialog");
  await expect(access).not.toBeChecked();
  model.loseMutationResponse = true;
  await access.click();
  await expect(confirm).toContainText("能够读取转发内容");
  await expect(confirm).toContainText("公网 Shell");
  await confirm.getByRole("button", { name: "确认启用", exact: true }).click();
  await expect(access).toBeChecked();
  await expect(
    page.getByRole("link", { name: "通过所有者账号打开 WebMux", exact: true }),
  ).toHaveAttribute("href", publicUrl + "/auth/relay/start");
  expect(
    model.mutations.filter((entry) => entry.path === "/api/relay"),
  ).toHaveLength(1);
  await access.click();
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(access).not.toBeChecked();
  await expect(
    page.getByRole("link", { name: "通过所有者账号打开 WebMux", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "危险操作", exact: true }).click();
  await page.getByRole("button", { name: "删除绑定", exact: true }).click();
  await expect(confirm).toContainText("永久失去旧公共地址");
  await confirm.getByRole("button", { name: "确认操作", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "使用 GitHub 配对", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByText(/旧.*访问.*未确认|未确认.*旧.*访问/),
  ).toBeVisible();
  await expect(access).toHaveCount(0);
  expect(model.status.operation).toMatchObject({
    phase: "completed",
    deleted: true,
    accessClosed: false,
  });
});
