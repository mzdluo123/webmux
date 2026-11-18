import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import RelayManager from "../../src/components/settings/RelayManager.svelte";
import { relayStatus, relayPairing } from "../fixtures/relay";
import type {
  RelayPairing,
  RelayStatus,
} from "../../src/contracts/relay-schemas";
import {
  applyRelayAuth,
  clearRelayAuth,
  relayAuthRevision,
  relayOwnerAuthenticated,
} from "../../src/platform/relay-auth";
import { resetHostFeatures } from "./feature-fixture";
import { initializeLocale, locale } from "../../src/lib/locale";
import { m } from "$lib/paraglide/messages.js";

beforeEach(() => {
  initializeLocale();
  locale.setPreference("zh-CN");
});
afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  clearRelayAuth();
  resetHostFeatures();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  locale.setPreference("zh-CN");
});

const waiting: RelayPairing = {
  ...relayPairing,
  phase: "waiting",
  userCode: "ABCD-EFGH",
  verificationUrl: "https://github.com/login/device",
  expiresAt: Math.floor(Date.now() / 1000) + 600,
  pollIntervalSeconds: 1,
};
const paired: RelayStatus = {
  ...relayStatus,
  paired: true,
  ownerId: "1234567",
  ownerLogin: "relay-owner",
  boundPublicUrl:
    "https://h-0123456789abcdef0123456789abcdef.relay.example.test",
};
const dialog = () => page.getByRole("alertdialog");
const start = () =>
  page.getByRole("button", { name: m.relay_start_pairing(), exact: true });
const cancel = () =>
  page.getByRole("button", { name: m.relay_cancel_pairing(), exact: true });
const code = () => page.getByLabelText(m.relay_pairing_code());
const check = () =>
  page.getByRole("button", { name: m.relay_refresh(), exact: true });

function installHost() {
  const host = {
    status: { ...relayStatus },
    pairing: { ...relayPairing },
    requests: [] as { path: string; method: string; body: unknown }[],
    reply: undefined as
      | undefined
      | ((
          path: string,
          method: string,
        ) => Promise<Response> | Response | undefined),
  };
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      const method = init?.method || "GET";
      host.requests.push({
        path,
        method,
        body: init?.body ? JSON.parse(String(init.body)) : undefined,
      });
      const response = host.reply?.(path, method);
      if (response) return response;
      if (path === "/api/relay/enrollments") {
        if (method === "POST") host.pairing = { ...waiting };
        if (method === "DELETE")
          host.pairing = { ...relayPairing, phase: "cancelled" };
        return Response.json(host.pairing);
      }
      if (
        path === "/api/relay" ||
        path === "/api/relay/disconnect" ||
        path === "/api/relay/binding"
      )
        return Response.json(host.status);
      throw new Error("Unexpected isolated request " + path);
    }),
  );
  return host;
}
function observeNow() {
  document.dispatchEvent(new Event("visibilitychange"));
}
async function startPairing() {
  await start().click();
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  await expect.element(dialog()).not.toBeInTheDocument();
}

test("inline pairing has one direct action and prevents duplicate preparation", async () => {
  const host = installHost();
  const pending = Promise.withResolvers<Response>();
  host.reply = (path, method) => {
    if (path.endsWith("/enrollments") && method === "POST") {
      host.pairing = { ...relayPairing, phase: "preparing" };
      return pending.promise;
    }
  };
  render(RelayManager);
  await expect.element(start()).toBeEnabled();
  await expect.element(page.getByText(m.relay_pairing_trust())).toBeVisible();
  await expect.element(page.getByRole("switch")).not.toBeInTheDocument();
  const button = document.querySelector("button")!;
  await start().click();
  button.click();
  expect(
    host.requests.filter((request) => request.method === "POST"),
  ).toHaveLength(1);
  await expect
    .element(page.getByText(m.relay_pairing_preparing()))
    .toBeVisible();
  host.pairing = { ...waiting };
  pending.resolve(Response.json(waiting));
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  await expect
    .element(page.getByRole("link", { name: m.relay_verify_github() }))
    .toHaveAttribute("href", waiting.verificationUrl);
  expect(host.requests.some((request) => request.path.endsWith("/poll"))).toBe(
    false,
  );
});

test("navigation restores the same attempt without unmount cancellation or browser persistence", async () => {
  const host = installHost();
  const writes = vi.spyOn(Storage.prototype, "setItem");
  render(RelayManager);
  await startPairing();
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  await cleanup();
  expect(
    host.requests.filter((request) => request.method === "DELETE"),
  ).toHaveLength(0);
  render(RelayManager);
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  expect(
    host.requests.filter((request) => request.method === "POST"),
  ).toHaveLength(1);
  expect(
    writes.mock.calls.some((args) =>
      args.some((value) => String(value).includes(waiting.userCode)),
    ),
  ).toBe(false);
});

test("transient observer failures preserve the code and cancel, then automatically clear stale errors", async () => {
  const host = installHost();
  render(RelayManager);
  await startPairing();
  let fail = true;
  host.reply = (path, method) => {
    if (method === "GET" && path.endsWith("/enrollments") && fail)
      return Promise.reject(new TypeError("offline"));
  };
  observeNow();
  await expect
    .element(page.getByText(m.relay_error_host(), { exact: true }))
    .toBeVisible();
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  await expect.element(cancel()).toBeEnabled();
  fail = false;
  await expect
    .poll(() => document.body.textContent?.includes(m.relay_error_host()), {
      timeout: 8000,
    })
    .toBe(false);
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  expect(
    host.requests.filter((request) => request.method === "POST"),
  ).toHaveLength(1);
});

test.each(["denied", "expired", "interrupted"] as const)(
  "only host-authoritative %s ends the attempt and permits an explicit new start",
  async (phase) => {
    const host = installHost();
    host.pairing = { ...waiting, expiresAt: 1 };
    render(RelayManager);
    await expect.element(code()).toHaveTextContent(waiting.userCode);
    await expect
      .element(page.getByText(m.relay_pairing_expiry_check()))
      .toBeVisible();
    await expect.element(cancel()).toBeEnabled();
    host.pairing = { ...relayPairing, phase };
    observeNow();
    const message =
      phase === "denied"
        ? m.relay_error_denied()
        : phase === "expired"
          ? m.relay_error_expired()
          : m.relay_pairing_interrupted();
    await expect
      .element(page.getByText(message, { exact: true }))
      .toBeVisible();
    await expect.element(code()).not.toBeInTheDocument();
    await expect.element(start()).toBeEnabled();
    expect(
      host.requests.filter((request) => request.method === "POST"),
    ).toHaveLength(0);
  },
);

test("unknown creation response reconciles the existing attempt without replaying POST", async () => {
  const host = installHost();
  host.reply = (path, method) => {
    if (path.endsWith("/enrollments") && method === "POST") {
      host.pairing = { ...relayPairing, phase: "uncertain" };
      return Promise.reject(new TypeError("response lost"));
    }
  };
  render(RelayManager);
  await start().click();
  await expect
    .element(page.getByText(m.relay_pairing_uncertain()))
    .toBeVisible();
  await expect.element(cancel()).toBeEnabled();
  await check().click();
  expect(
    host.requests.filter((request) => request.method === "POST"),
  ).toHaveLength(1);
  await expect.element(start()).not.toBeInTheDocument();
  await cancel().click();
  await expect.element(start()).toBeEnabled();
  expect(
    host.requests.find((request) => request.method === "DELETE")?.body,
  ).toEqual({});
});

test("cancellation accepts authoritative late success and ignores an older observer reply", async () => {
  const host = installHost();
  render(RelayManager);
  await startPairing();
  const oldRead = Promise.withResolvers<Response>();
  let delayed = false;
  host.reply = (path, method) => {
    if (path.endsWith("/enrollments") && method === "GET" && !delayed) {
      delayed = true;
      return oldRead.promise;
    }
    if (path.endsWith("/enrollments") && method === "DELETE") {
      host.status = { ...paired };
      host.pairing = { ...relayPairing, phase: "paired" };
      return Response.json(host.pairing);
    }
  };
  observeNow();
  await expect.poll(() => delayed).toBe(true);
  await cancel().click();
  await expect
    .element(page.getByText(m.relay_owner_login({ login: paired.ownerLogin })))
    .toBeVisible();
  oldRead.resolve(Response.json(waiting));
  await expect.element(code()).not.toBeInTheDocument();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .not.toBeChecked();
  await expect.element(start()).not.toBeInTheDocument();
});

test.each([false, true])(
  "record deletion is complete independently of access closure (%s)",
  async (accessClosed) => {
    const host = installHost();
    host.status = { ...paired };
    host.reply = (path, method) => {
      if (path.endsWith("/binding") && method === "DELETE") {
        host.status = {
          ...relayStatus,
          operation: {
            kind: "delete",
            phase: "completed",
            error: "",
            deleted: true,
            accessClosed,
          },
        };
      }
    };
    render(RelayManager);
    await page
      .getByRole("button", { name: m.relay_danger_actions(), exact: true })
      .click();
    applyRelayAuth(
      {
        authenticated: true,
        csrfToken: "owner-csrf",
        expiresAt: Math.floor(Date.now() / 1000) + 600,
      },
      relayAuthRevision(),
    );
    await page
      .getByRole("button", { name: m.relay_delete_binding(), exact: true })
      .click();
    await expect.element(dialog()).toMatchTextContent(/永久失去旧公共地址/);
    await dialog()
      .getByRole("button", { name: m.relay_confirm_action(), exact: true })
      .click();
    await expect.element(start()).toBeEnabled();
    await expect
      .element(
        page.getByText(
          accessClosed ? m.relay_deleted_closed() : m.relay_deleted_unclosed(),
          { exact: true },
        ),
      )
      .toBeVisible();
    await expect
      .element(page.getByText(m.relay_operation_verifying()))
      .not.toBeInTheDocument();
    expect(relayOwnerAuthenticated()).toBe(false);
  },
);

test("weak recovery Token blocks enable but allows emergency disconnect", async () => {
  const host = installHost();
  host.status = { ...paired, tokenStrong: false };
  host.reply = (path, method) => {
    if (path.endsWith("/disconnect") && method === "POST") {
      host.status = {
        ...host.status,
        enabled: false,
        state: "off",
        operation: {
          ...relayStatus.operation,
          kind: "disable",
          phase: "completed",
          accessClosed: true,
        },
      };
    }
  };
  render(RelayManager);
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .toBeDisabled();
  host.status = { ...host.status, enabled: true, state: "ready" };
  observeNow();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .toBeEnabled();
  await page.getByRole("switch", { name: m.relay_allow_public() }).click();
  await expect.element(dialog()).toMatchTextContent(/持久终端进程不会退出/);
  await dialog()
    .getByRole("button", { name: m.relay_confirm_action(), exact: true })
    .click();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .not.toBeChecked();
});

test("WSS-only enable requires explicit consent; locale changes preserve confirmation and code", async () => {
  const host = installHost();
  render(RelayManager);
  await startPairing();
  locale.setPreference("en");
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  await expect.element(cancel()).toBeVisible();
  host.status = { ...paired };
  host.pairing = { ...relayPairing, phase: "paired" };
  observeNow();
  await page.getByRole("switch", { name: m.relay_allow_public() }).click();
  const confirmation = document.querySelector('[role="alertdialog"]');
  locale.setPreference("zh-CN");
  await expect
    .element(dialog().getByText(m.relay_consent(), { exact: true }))
    .toBeVisible();
  expect(document.querySelector('[role="alertdialog"]')).toBe(confirmation);
  expect(
    host.requests.filter((request) => request.method === "PUT"),
  ).toHaveLength(0);
  await expect
    .element(page.getByRole("link", { name: m.relay_open_owner() }))
    .not.toBeInTheDocument();
  host.reply = (path, method) => {
    if (path === "/api/relay" && method === "PUT") {
      host.status = {
        ...host.status,
        enabled: true,
        state: "ready",
        controlReady: true,
        publicUrl: paired.boundPublicUrl,
        operation: {
          ...relayStatus.operation,
          kind: "enable",
          phase: "completed",
        },
      };
    }
  };
  await dialog()
    .getByRole("button", { name: m.relay_confirm_enable_action(), exact: true })
    .click();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .toBeChecked();
  await expect
    .element(page.getByRole("link", { name: m.relay_open_owner() }))
    .toBeVisible();
  expect(
    host.requests.find((request) => request.method === "PUT")?.body,
  ).toEqual({ enabled: true, confirmPublic: true });
  expect(
    host.requests.filter((request) => request.method === "POST"),
  ).toHaveLength(1);
});

test.each(["zh-CN", "en"] as const)(
  "diagnostics progressively disclose CA trust, routing and account IDs (%s)",
  async (language) => {
    locale.setPreference(language);
    const host = installHost();
    host.status = {
      ...paired,
      connectorUrl: "wss://data.example.test/v1/connector",
      dataOrigin: "https://data.example.test",
    };
    render(RelayManager);
    await expect
      .element(
        page.getByText(
          m.relay_owner({ login: paired.ownerLogin, id: paired.ownerId }),
          { exact: true },
        ),
      )
      .not.toBeVisible();
    await page
      .getByRole("button", {
        name: m.relay_effective_diagnostics(),
        exact: true,
      })
      .click();
    await expect
      .element(page.getByText(m.relay_health(), { exact: true }))
      .toBeVisible();
    await page
      .getByRole("button", { name: m.relay_routing(), exact: true })
      .click();
    await expect
      .element(page.getByText(m.relay_control_ca_trust(), { exact: true }))
      .toBeVisible();
    await expect
      .element(page.getByText(host.status.connectorUrl, { exact: false }))
      .toBeVisible();
    await expect
      .element(
        page.getByText(
          m.relay_owner({ login: paired.ownerLogin, id: paired.ownerId }),
          { exact: true },
        ),
      )
      .toBeVisible();
    await expect.element(check()).not.toBeInTheDocument();
  },
);

test("copying the code requires a user action and does not change the authorization URL", async () => {
  const host = installHost();
  host.pairing = { ...waiting };
  const copy = vi
    .spyOn(navigator.clipboard, "writeText")
    .mockResolvedValue(undefined);
  render(RelayManager);
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  expect(copy).not.toHaveBeenCalled();
  await page
    .getByRole("button", { name: m.relay_copy_code(), exact: true })
    .click();
  expect(copy).toHaveBeenCalledTimes(1);
  expect(copy).toHaveBeenCalledWith(waiting.userCode);
  await expect
    .element(page.getByRole("link", { name: m.relay_verify_github() }))
    .toHaveAttribute("href", waiting.verificationUrl);
  host.pairing = { ...waiting, userCode: "JKLM-NPQR" };
  observeNow();
  await expect.element(code()).toHaveTextContent("JKLM-NPQR");
  await expect
    .element(
      page.getByRole("button", { name: m.relay_copy_code(), exact: true }),
    )
    .toBeVisible();
  expect(copy).toHaveBeenCalledTimes(1);
});

test("host retry state retains the same code until paired success and clears stale read errors", async () => {
  const host = installHost();
  host.pairing = {
    ...waiting,
    phase: "retrying",
    error: "unavailable",
    retryAt: Math.floor(Date.now() / 1000) + 5,
  };
  render(RelayManager);
  await expect
    .element(page.getByText(m.relay_pairing_retrying()))
    .toBeVisible();
  await expect.element(code()).toHaveTextContent(waiting.userCode);
  await expect.element(cancel()).toBeEnabled();
  host.status = { ...paired };
  host.pairing = { ...relayPairing, phase: "paired" };
  observeNow();
  await expect.element(page.getByText(m.relay_notice_paired())).toBeVisible();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .not.toBeChecked();
  await expect.element(code()).not.toBeInTheDocument();
  await expect
    .element(page.getByText(m.relay_pairing_retrying()))
    .not.toBeInTheDocument();
});

test("unknown enable response checks the host receipt without replay and clears the read error on recovery", async () => {
  const host = installHost();
  host.status = { ...paired };
  let offline = false;
  host.reply = (path, method) => {
    if (path === "/api/relay" && method === "PUT") {
      host.status = {
        ...paired,
        enabled: true,
        state: "connecting",
        operation: {
          ...relayStatus.operation,
          kind: "enable",
          phase: "submitting",
        },
      };
      offline = true;
      return Promise.reject(new TypeError("response lost"));
    }
    if (method === "GET" && offline)
      return Promise.reject(new TypeError("offline"));
  };
  render(RelayManager);
  await page.getByRole("switch", { name: m.relay_allow_public() }).click();
  await dialog()
    .getByRole("button", { name: m.relay_confirm_enable_action(), exact: true })
    .click();
  await expect
    .element(page.getByText(m.relay_error_host(), { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByText(m.relay_operation_verifying()))
    .toBeVisible();
  offline = false;
  host.status = {
    ...host.status,
    state: "ready",
    operation: { ...host.status.operation, phase: "completed" },
  };
  await check().click();
  await expect
    .element(page.getByText(m.relay_notice_completed()))
    .toBeVisible();
  await expect
    .element(page.getByText(m.relay_error_host(), { exact: true }))
    .not.toBeInTheDocument();
  expect(
    host.requests.filter((request) => request.method === "PUT"),
  ).toHaveLength(1);
});

test("terminal denial translates on a mounted locale change without creating a new attempt", async () => {
  const host = installHost();
  host.pairing = { ...relayPairing, phase: "denied", error: "forbidden" };
  render(RelayManager);
  await expect
    .element(page.getByText(m.relay_error_denied(), { exact: true }))
    .toBeVisible();
  locale.setPreference("en");
  await expect
    .element(page.getByText(m.relay_error_denied(), { exact: true }))
    .toBeVisible();
  await expect.element(start()).toBeEnabled();
  expect(
    host.requests.filter((request) => request.method === "POST"),
  ).toHaveLength(0);
});

test("unknown host deletion retains the binding without an infinite pending lock and allows confirmed recovery", async () => {
  const host = installHost();
  host.status = {
    ...paired,
    operation: {
      kind: "delete",
      phase: "uncertain",
      error: "unavailable",
      deleted: false,
      accessClosed: false,
    },
  };
  host.reply = (path, method) => {
    if (path.endsWith("/binding") && method === "DELETE") {
      host.status = {
        ...relayStatus,
        operation: {
          kind: "delete",
          phase: "completed",
          error: "",
          deleted: true,
          accessClosed: false,
        },
      };
    }
  };
  render(RelayManager);
  await expect
    .element(page.getByText(m.relay_delete_uncertain(), { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByText(m.relay_operation_verifying()))
    .not.toBeInTheDocument();
  await expect.element(start()).not.toBeInTheDocument();
  await check().click();
  expect(
    host.requests.filter((request) => request.method === "DELETE"),
  ).toHaveLength(0);
  await page
    .getByRole("button", { name: m.relay_danger_actions(), exact: true })
    .click();
  await page
    .getByRole("button", { name: m.relay_delete_binding(), exact: true })
    .click();
  await dialog()
    .getByRole("button", { name: m.relay_confirm_action(), exact: true })
    .click();
  await expect.element(start()).toBeEnabled();
  await expect
    .element(page.getByText(m.relay_deleted_unclosed(), { exact: true }))
    .toBeVisible();
  expect(
    host.requests.filter((request) => request.method === "DELETE"),
  ).toHaveLength(1);
});

test("a rejected mutation stays actionable until later authoritative success clears its stale error", async () => {
  const host = installHost();
  host.reply = (path, method) => {
    if (path.endsWith("/enrollments") && method === "POST")
      return Response.json({ error: "forbidden" }, { status: 403 });
  };
  render(RelayManager);
  await start().click();
  await expect
    .element(page.getByText(m.relay_error_denied(), { exact: true }))
    .toBeVisible();
  await check().click();
  await expect
    .element(page.getByText(m.relay_error_denied(), { exact: true }))
    .toBeVisible();
  host.status = { ...paired };
  host.pairing = { ...relayPairing, phase: "paired" };
  await check().click();
  await expect
    .element(page.getByText(m.relay_notice_paired(), { exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByText(m.relay_error_denied(), { exact: true }))
    .not.toBeInTheDocument();
  await expect
    .element(page.getByRole("switch", { name: m.relay_allow_public() }))
    .not.toBeChecked();
  expect(
    host.requests.filter((request) => request.method === "POST"),
  ).toHaveLength(1);
});
test("pairing polls keep cancel button anchored, omit idle error gap, and retain copy focus", async () => {
  const host = installHost();
  const pending = Promise.withResolvers<Response>();
  host.reply = (path, method) => {
    if (path.endsWith("/enrollments") && method === "POST") {
      host.pairing = { ...relayPairing, phase: "preparing" };
      return pending.promise;
    }
  };
  const container = Object.assign(document.createElement("section"), {
    style: "width: 390px; max-width: 100%; margin: 0 auto",
  });
  document.body.append(container);
  try {
    render(RelayManager, { target: container });
    await expect.element(start()).toBeEnabled();
    await start().click();
    await expect.element(cancel()).toBeVisible();
    expect(container.querySelector(".invisible")).toBeNull();
    const cancelPreparingLeft = cancel().element().getBoundingClientRect().left;

    host.reply = undefined;
    host.pairing = { ...waiting };
    pending.resolve(Response.json(waiting));
    await expect.element(code()).toHaveTextContent(waiting.userCode);
    await vi.waitFor(() =>
      expect(container.getAnimations({ subtree: true })).toHaveLength(0),
    );

    const verifyLink = page.getByRole("link", {
      name: m.relay_verify_github(),
    });
    await expect.element(verifyLink).toBeVisible();
    const cancelWaitingRect = cancel().element().getBoundingClientRect();
    const verifyWaitingRect = verifyLink.element().getBoundingClientRect();
    expect(cancelWaitingRect.left).toBeCloseTo(cancelPreparingLeft, 0);
    expect(verifyWaitingRect.left).toBeGreaterThan(cancelWaitingRect.left);
    expect(container.querySelector('[data-slot="scroll-area"]')).toBeNull();

    const copy = page.getByRole("button", { name: m.relay_copy_code() });
    (copy.element() as HTMLElement).focus();
    const selection = window.getSelection()!;
    selection.selectAllChildren(code().element());
    host.pairing = { ...waiting, phase: "retrying" };
    observeNow();
    await expect
      .element(page.getByText(m.relay_pairing_retrying()))
      .toBeVisible();
    expect(container.querySelector('[data-slot="scroll-area"]')).not.toBeNull();
    const cancelRetryRect = cancel().element().getBoundingClientRect();
    expect(cancelRetryRect.top).toBeCloseTo(cancelWaitingRect.top, 0);
    expect(cancelRetryRect.left).toBeCloseTo(cancelWaitingRect.left, 0);
    expect(document.activeElement).toBe(copy.element());
    expect(selection.toString()).toBe(waiting.userCode);
  } finally {
    container.remove();
  }
});
