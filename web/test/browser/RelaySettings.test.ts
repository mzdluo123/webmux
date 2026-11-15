import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import RelayManager from "../../src/components/settings/RelayManager.svelte";
import { relayStatus } from "../fixtures/relay";
import { clearRelayAuth } from "../../src/platform/relay-auth";
import { resetHostPlugins } from "./plugin-fixture";

const enrollment = {
  version: 1,
  handle: "A".repeat(43),
  userCode: "ABCD-EFGH",
  verificationUrl: "https://github.com/login/device",
  expiresAt: Math.floor(Date.now() / 1000) + 600,
  pollIntervalSeconds: 1,
};

afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  clearRelayAuth();
  resetHostPlugins();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const dialog = () => page.getByRole("alertdialog");
async function startPairing() {
  await page
    .getByRole("button", { name: "开始 GitHub 配对", exact: true })
    .click();
  await expect.element(dialog()).toMatchTextContent(/能够读取转发内容/);
  await dialog().getByRole("button", { name: "确认操作", exact: true }).click();
  await expect
    .element(page.getByLabelText("GitHub 配对码"))
    .toHaveTextContent(enrollment.userCode);
}

test("cancelling in-flight pairing discards late successful owner state and keeps pairing material out of storage", async () => {
  const pending = Promise.withResolvers<Response>();
  let polling = false;
  const writes = vi.spyOn(Storage.prototype, "setItem");
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      if (path === "/api/relay") return Response.json(relayStatus);
      if (path === "/api/relay/enrollments/poll") {
        polling = true;
        return pending.promise;
      }
      if (path === "/api/relay/enrollments") {
        if (init?.method === "DELETE")
          return new Response(null, { status: 204 });
        return Response.json(enrollment, { status: 201 });
      }
      throw new Error("Unexpected isolated request " + path);
    }),
  );
  render(RelayManager);
  await startPairing();
  await expect.poll(() => polling).toBe(true);
  await page.getByRole("button", { name: "取消配对", exact: true }).click();
  pending.resolve(
    Response.json({
      ...relayStatus,
      paired: true,
      ownerId: "999999",
      ownerLogin: "revoked-owner",
    }),
  );
  const frame = Promise.withResolvers<void>();
  requestAnimationFrame(() => frame.resolve());
  await frame.promise;
  await expect
    .element(
      page.getByRole("button", { name: "开始 GitHub 配对", exact: true }),
    )
    .toBeVisible();
  await expect
    .element(page.getByText("revoked-owner", { exact: false }))
    .not.toBeInTheDocument();
  expect(
    writes.mock.calls.some((args) =>
      args.some(
        (value) =>
          String(value).includes(enrollment.handle) ||
          String(value).includes(enrollment.userCode),
      ),
    ),
  ).toBe(false);
});

test("weak recovery Token blocks enable but does not block emergency disconnect", async () => {
  let status = { ...relayStatus, paired: true, tokenStrong: false };
  const mutations: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      if (init?.method === "POST") {
        mutations.push(path);
        status = { ...status, enabled: false, state: "off" };
      }
      return Response.json(status);
    }),
  );
  render(RelayManager);
  await expect
    .element(page.getByRole("switch", { name: "允许 Relay 公网访问" }))
    .toBeDisabled();
  await cleanup();
  status = { ...status, enabled: true, state: "ready" };
  render(RelayManager);
  await page.getByRole("switch", { name: "允许 Relay 公网访问" }).click();
  await expect.element(dialog()).toMatchTextContent(/持久终端进程不会退出/);
  await dialog().getByRole("button", { name: "确认操作", exact: true }).click();
  await expect
    .element(page.getByRole("switch", { name: "允许 Relay 公网访问" }))
    .not.toBeChecked();
  expect(mutations).toEqual(["/api/relay/disconnect"]);
});

test("pending remote deletion keeps identity visible rather than claiming unpaired", async () => {
  const paired = { ...relayStatus, paired: true, ownerId: "1234567" };
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (_input, init) => {
      if (init?.method === "DELETE")
        return Response.json({ error: "pending" }, { status: 202 });
      return Response.json(paired);
    }),
  );
  render(RelayManager);
  await page.getByRole("button", { name: "删除绑定", exact: true }).click();
  await dialog().getByRole("button", { name: "确认操作", exact: true }).click();
  await expect
    .element(page.getByText("远端撤销尚未确认", { exact: false }))
    .toBeVisible();
  await expect
    .element(page.getByText("GitHub 所有者：账号 · 1234567", { exact: true }))
    .toBeVisible();
});

test("WSS enable remains usable without P2P and off-state bound address is not an open link", async () => {
  let status = {
    ...relayStatus,
    paired: true,
    boundPublicUrl:
      "https://h-0123456789abcdef0123456789abcdef.relay.example.test",
  };
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      paths.push(new URL(String(input), location.href).pathname);
      if (init?.method === "PUT")
        status = { ...status, enabled: true, state: "connecting" };
      return Response.json(status);
    }),
  );
  render(RelayManager);
  await expect
    .element(page.getByText("当前未选择 p2p 插件", { exact: false }))
    .toBeVisible();
  await expect
    .element(page.getByText(status.boundPublicUrl, { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByRole("link", { name: "通过所有者账号打开 WebMux" }))
    .not.toBeInTheDocument();
  await page.getByRole("switch", { name: "允许 Relay 公网访问" }).click();
  await dialog().getByRole("button", { name: "确认启用", exact: true }).click();
  await expect
    .element(page.getByRole("switch", { name: "允许 Relay 公网访问" }))
    .toBeChecked();
  expect(paths.every((path) => path === "/api/relay")).toBe(true);
});
