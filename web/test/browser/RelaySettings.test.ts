import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import RelayManager from "../../src/components/settings/RelayManager.svelte";
import { relayStatus } from "../fixtures/relay";
import { clearRelayAuth } from "../../src/platform/relay-auth";
import { resetHostFeatures } from "./feature-fixture";
import { initializeLocale, locale } from "../../src/lib/locale";
import { m } from "$lib/paraglide/messages.js";

beforeEach(() => {
  initializeLocale();
  locale.setPreference("zh-CN");
});

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
  resetHostFeatures();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  locale.setPreference("zh-CN");
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

test.each(["zh-CN", "en"] as const)(
  "public control trust explains CA verification without changing data endpoint diagnostics (%s)",
  async (language) => {
    locale.setPreference(language);
    const status = {
      ...relayStatus,
      connectorUrl: "wss://data.example.test/v1/connector",
      dataOrigin: "https://data.example.test",
    };
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>(async () => Response.json(status)),
    );
    render(RelayManager);

    await page
      .getByText(m.relay_effective_diagnostics(), { exact: true })
      .click();
    await expect
      .element(page.getByText(m.relay_control_trust(), { exact: true }))
      .toBeVisible();
    await expect
      .element(page.getByText(m.relay_control_ca_trust(), { exact: true }))
      .toBeVisible();
    await expect
      .element(page.getByText(relayStatus.controlEndpoint, { exact: false }))
      .toBeVisible();
    await expect
      .element(page.getByText(status.connectorUrl, { exact: false }))
      .toBeVisible();
    await expect
      .element(page.getByText(status.dataOrigin, { exact: false }))
      .toBeVisible();
  },
);

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

test("mounted locale changes preserve the enable confirmation and require explicit public consent", async () => {
  let status = {
    ...relayStatus,
    paired: true,
    ownerId: "1234567",
    ownerLogin: "locale-owner",
  };
  const mutations: { path: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      if (init?.method === "PUT") {
        mutations.push({ path, body: JSON.parse(String(init.body)) });
        status = { ...status, enabled: true, state: "ready" };
      }
      return Response.json(status);
    }),
  );
  render(RelayManager);
  await page.getByRole("switch", { name: m.relay_allow_public() }).click();
  await expect
    .element(dialog().getByText(m.relay_consent(), { exact: true }))
    .toBeVisible();
  const confirmation = document.querySelector('[role="alertdialog"]');
  locale.setPreference("en");
  await expect
    .element(dialog().getByText(m.relay_confirm_enable(), { exact: true }))
    .toBeVisible();
  await expect
    .element(dialog().getByText(m.relay_consent(), { exact: true }))
    .toBeVisible();
  expect(document.querySelector('[role="alertdialog"]')).toBe(confirmation);
  expect(mutations).toEqual([]);
  expect(
    document.getElementById("relay-enabled")?.getAttribute("aria-checked"),
  ).toBe("false");
  locale.setPreference("zh-CN");
  await expect
    .element(dialog().getByText(m.relay_confirm_enable(), { exact: true }))
    .toBeVisible();
  expect(mutations).toEqual([]);
  await dialog()
    .getByRole("button", { name: m.relay_confirm_enable_action(), exact: true })
    .click();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .toBeChecked();
  await expect
    .element(
      page.getByText(m.relay_owner({ login: "locale-owner", id: "1234567" }), {
        exact: true,
      }),
    )
    .toBeVisible();
  await expect
    .element(page.getByText(m.relay_notice_completed(), { exact: true }))
    .toBeVisible();
  locale.setPreference("en");
  await expect
    .element(page.getByText(m.relay_notice_completed(), { exact: true }))
    .toBeVisible();
  expect(mutations).toEqual([
    { path: "/api/relay", body: { enabled: true, confirmPublic: true } },
  ]);
});

test("mounted locale changes keep device pairing state without restarting enrollment or enabling access", async () => {
  const mutations: { path: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      if (
        init?.method === "POST" ||
        init?.method === "DELETE" ||
        init?.method === "PUT"
      ) {
        mutations.push({ path, body: JSON.parse(String(init.body)) });
      }
      if (init?.method === "DELETE") return new Response(null, { status: 204 });
      if (path.endsWith("/enrollments/poll"))
        return Response.json({ error: "pending" });
      if (path.endsWith("/enrollments"))
        return Response.json({ ...enrollment, pollIntervalSeconds: 60 });
      return Response.json(relayStatus);
    }),
  );
  render(RelayManager);
  await startPairing();
  locale.setPreference("en");
  await expect
    .element(page.getByLabelText(m.relay_pairing_code()))
    .toHaveTextContent(enrollment.userCode);
  await expect
    .element(
      page.getByRole("button", { name: m.relay_cancel_pairing(), exact: true }),
    )
    .toBeVisible();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .not.toBeChecked();
  locale.setPreference("zh-CN");
  await expect
    .element(page.getByLabelText(m.relay_pairing_code()))
    .toHaveTextContent(enrollment.userCode);
  expect(
    mutations.filter((entry) => entry.path === "/api/relay/enrollments"),
  ).toEqual([
    {
      path: "/api/relay/enrollments",
      body: { rebind: false, confirmPublic: true },
    },
  ]);
  expect(mutations.some((entry) => entry.path === "/api/relay")).toBe(false);
  await page
    .getByRole("button", { name: m.relay_cancel_pairing(), exact: true })
    .click();
  await expect
    .element(page.getByText(m.relay_notice_cancelled(), { exact: true }))
    .toBeVisible();
  expect(mutations.at(-1)).toEqual({
    path: "/api/relay/enrollments",
    body: { handle: enrollment.handle },
  });
});

test("mounted locale changes translate retained authorization errors and diagnostic reasons", async () => {
  const enroll = vi.fn();
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      if (path.endsWith("/enrollments") && init?.method === "POST") {
        enroll();
        return Response.json({ error: "denied" }, { status: 403 });
      }
      return Response.json({ ...relayStatus, error: "turn-unavailable" });
    }),
  );
  render(RelayManager);
  await expect
    .element(page.getByText(m.relay_diag_turn(), { exact: true }))
    .toBeVisible();
  await page
    .getByRole("button", { name: m.relay_start_pairing(), exact: true })
    .click();
  await dialog()
    .getByRole("button", { name: m.relay_confirm_action(), exact: true })
    .click();
  await expect
    .element(page.getByText(m.relay_error_denied(), { exact: true }))
    .toBeVisible();
  locale.setPreference("en");
  await expect
    .element(page.getByText(m.relay_error_denied(), { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByText(m.relay_diag_turn(), { exact: true }))
    .toBeVisible();
  await expect
    .element(
      page.getByRole("button", { name: m.relay_start_pairing(), exact: true }),
    )
    .toBeVisible();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .not.toBeChecked();
  expect(enroll).toHaveBeenCalledTimes(1);
});
