import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import { m } from "$lib/paraglide/messages.js";
import RelaySettings from "../../src/components/settings/RelaySettings.svelte";
import type { RelayStatus } from "../../src/contracts/relay-schemas";
import type { RelaySettingsView } from "../../src/hooks/settings/relay-settings.svelte";
import { initializeLocale, locale } from "../../src/lib/locale";
import { relayPairing, relayStatus } from "../fixtures/relay";

beforeEach(() => {
  initializeLocale();
  locale.setPreference("en");
});
afterEach(async () => {
  await cleanup();
  await userEvent.cleanup();
  locale.setPreference("zh-CN");
});

const checkedAt = "2026-11-25T10:42:16Z";
const completedAt = "2026-11-25T10:42:24Z";
const publicUrl = "https://h-browser-test.relay.example.test";
const ready: RelayStatus = {
  ...relayStatus,
  paired: true,
  enabled: true,
  controlReady: true,
  dataReady: true,
  state: "ready",
  nodeId: "relay-west",
  boundPublicUrl: publicUrl,
  publicUrl,
  dataReachability: "reachable",
  dataCheckedAt: checkedAt,
};
const wssSelection = {
  supported: true,
  preferred: "wss",
  active: "wss",
} as const;

async function renderSettings(status: RelayStatus) {
  const view: RelaySettingsView = {
    status,
    pairing: relayPairing,
    loading: false,
    busy: false,
    phase: "idle",
    error: "",
    notice: "",
  };
  const ontransport = vi.fn();
  const rendered = await render(RelaySettings, {
    view,
    ontransport,
    onrefresh: vi.fn(),
    onpair: vi.fn(),
    oncancel: vi.fn(),
    onenable: vi.fn(),
    ondisconnect: vi.fn(),
    onreconnect: vi.fn(),
    ondelete: vi.fn(),
  });
  return {
    ontransport,
    update: (changes: Partial<RelaySettingsView>) =>
      rendered.rerender({ view: { ...view, ...changes } }),
  };
}

const section = () =>
  page.getByRole("region", { name: m.relay_transport_title() });
const choices = () =>
  section().getByRole("radiogroup", { name: m.relay_transport_choice() });
const wss = () =>
  choices().getByRole("radio", { name: m.relay_transport_wss_default() });
const quic = () =>
  choices().getByRole("radio", { name: m.relay_transport_quic_experimental() });
const apply = () =>
  section().getByRole("button", { name: m.relay_transport_apply() });
const fieldValue = (name: string) =>
  section()
    .getByText(name, { exact: true })
    .element()
    .nextElementSibling?.textContent?.trim();

function expectTransport(active: string, preferred: string) {
  expect(fieldValue(m.relay_transport_active())).toBe(active);
  expect(fieldValue(m.relay_transport_preferred())).toBe(preferred);
}

test("keyboard selection is a draft; applying verifies and can switch back explicitly", async () => {
  const { ontransport, update } = await renderSettings({
    ...ready,
    transportSelection: wssSelection,
  });
  await expect.element(apply()).toBeDisabled();
  await wss().click();
  await userEvent.keyboard("{ArrowRight}");
  await expect.element(quic()).toHaveFocus();
  await expect.element(quic()).toHaveAttribute("aria-checked", "true");
  expectTransport("WSS/TCP", "WSS/TCP");
  expect(ontransport).not.toHaveBeenCalled();

  await userEvent.keyboard("{Tab}{Enter}");
  expect(ontransport).toHaveBeenCalledExactlyOnceWith("quic");
  await update({ busy: true, testingTransport: "quic" });
  await expect
    .element(section().getByRole("status"))
    .toHaveTextContent(m.relay_transport_testing({ protocol: "QUIC/UDP" }));
  await expect.element(apply()).toBeDisabled();
  await expect.element(wss()).toBeDisabled();
  expectTransport("WSS/TCP", "WSS/TCP");

  await update({
    status: {
      ...ready,
      transportSelection: {
        supported: true,
        preferred: "quic",
        active: "quic",
        lastAttempt: {
          nodeId: ready.nodeId,
          protocol: "quic",
          stage: "commit",
          error: "",
          durationMs: 250,
          completedAt,
        },
      },
    },
  });
  await expect
    .element(section().getByText(m.relay_transport_succeeded()))
    .toBeVisible();
  expectTransport("QUIC/UDP", "QUIC/UDP");
  await expect.element(apply()).toBeDisabled();
  await expect
    .element(page.getByRole("link", { name: m.relay_open_owner() }))
    .toHaveAttribute("href", `${publicUrl}/auth/relay/start`);
  await wss().click();
  await apply().click();
  expect(ontransport.mock.calls).toEqual([["quic"], ["wss"]]);
});

test("another client's pending and failed QUIC test preserve WSS and separate observations", async () => {
  const { ontransport, update } = await renderSettings({
    ...ready,
    transportSelection: { ...wssSelection, pending: "quic" },
  });
  await expect.element(quic()).toHaveAttribute("aria-checked", "true");
  await expect.element(apply()).toBeDisabled();
  expectTransport("WSS/TCP", "WSS/TCP");

  await update({
    status: {
      ...ready,
      transportSelection: {
        ...wssSelection,
        lastAttempt: {
          nodeId: ready.nodeId,
          protocol: "quic",
          stage: "handshake",
          error: "expired",
          durationMs: 5000,
          completedAt,
        },
      },
    },
  });
  for (const language of ["en", "zh-CN"] as const) {
    locale.setPreference(language);
    await expect
      .element(section().getByRole("alert"))
      .toHaveTextContent(
        m.relay_transport_failed({ reason: m.relay_transport_timed_out() }),
      );
    expectTransport("WSS/TCP", "WSS/TCP");
    expect(fieldValue(m.relay_transport_node())).toBe(ready.nodeId);
    expect(fieldValue(m.relay_transport_tested())).toBe("QUIC/UDP");
    expect(fieldValue(m.relay_transport_stage())).toBe(
      m.relay_transport_handshake(),
    );
    expect(fieldValue(m.relay_transport_duration())).toBe(
      m.relay_transport_duration_ms({
        duration: new Intl.NumberFormat(language).format(5000),
      }),
    );
    await expect
      .element(section().getByRole("time"))
      .toHaveAttribute("datetime", completedAt);
    const probeTime = new Intl.DateTimeFormat(language, {
      dateStyle: "medium",
      timeStyle: "medium",
    }).format(Date.parse(checkedAt));
    await expect
      .element(page.getByText(probeTime, { exact: true }))
      .toHaveAttribute("datetime", checkedAt);
    await expect
      .element(page.getByText(m.relay_reach_reachable(), { exact: true }))
      .toBeVisible();
  }
  expect(ontransport).not.toHaveBeenCalled();
});

test("an old host shows that testing is unavailable", async () => {
  await renderSettings(ready);
  await expect
    .element(section().getByText(m.relay_transport_unsupported_host()))
    .toBeVisible();
  await expect.element(choices()).not.toBeInTheDocument();
  await expect.element(apply()).not.toBeInTheDocument();
});

test("an unsupported node disables trials while active WSS remains visible", async () => {
  const { ontransport } = await renderSettings({
    ...ready,
    transportSelection: { ...wssSelection, supported: false },
  });
  await expect
    .element(section().getByText(m.relay_transport_unsupported_node()))
    .toBeVisible();
  await expect.element(quic()).toBeDisabled();
  await expect.element(apply()).toBeDisabled();
  expectTransport("WSS/TCP", "WSS/TCP");
  expect(ontransport).not.toHaveBeenCalled();
});

test("a saved QUIC preference with no active route can recover on a WSS-only node", async () => {
  const { ontransport, update } = await renderSettings({
    ...ready,
    state: "connecting",
    dataReady: false,
    dataReachability: "unknown",
    dataCheckedAt: undefined,
    publicUrl: undefined,
    transportSelection: {
      supported: false,
      preferred: "quic",
      lastAttempt: {
        nodeId: ready.nodeId,
        protocol: "quic",
        stage: "prepare",
        error: "incompatible_version",
        durationMs: 0,
        completedAt,
      },
    },
  });
  await expect
    .element(section().getByText(m.relay_transport_wss_recovery()))
    .toBeVisible();
  expectTransport(m.relay_none(), "QUIC/UDP");
  await expect.element(quic()).toBeDisabled();
  await expect.element(wss()).toBeEnabled();
  await apply().click();
  expect(ontransport).toHaveBeenCalledExactlyOnceWith("wss");
  await update({
    status: {
      ...ready,
      transportSelection: { ...wssSelection, supported: false },
    },
  });
  expectTransport("WSS/TCP", "WSS/TCP");
  await expect.element(apply()).toBeDisabled();
});
