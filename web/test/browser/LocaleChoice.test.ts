import { afterEach, beforeEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import ConnectionGate from "../../src/components/shell/ConnectionGate.svelte";
import InitInstructionField from "../../src/components/sessions/InitInstructionField.svelte";
import Settings from "../../src/pages/Settings.svelte";
import { PluginIDSchema } from "../../src/contracts/plugin-schemas";
import { QUICK_COMMAND_PRESETS } from "../../src/domains/settings/quick-commands";
import { emptySettings } from "../../src/domains/settings/settings-intent";
import {
  reloadSettings,
  stopSettings,
} from "../../src/domains/settings/settings.svelte";
import {
  clearHostStorage,
  setActiveHostId,
  store,
  stored,
} from "../../src/platform/storage";
import { initializeLocale, locale } from "../../src/lib/locale";
import { enableCreationPlugins, resetCreationPlugins } from "./plugin-fixture";

const originalURL = location.href;

beforeEach(async () => {
  await enableCreationPlugins(PluginIDSchema.options);
  localStorage.clear();
  initializeLocale();
  locale.setPreference("zh-CN");
});

afterEach(async () => {
  vi.restoreAllMocks();
  await cleanup();
  await userEvent.cleanup();
  resetCreationPlugins();
  vi.unstubAllGlobals();
  history.replaceState(null, "", originalURL);
  setActiveHostId(null);
  localStorage.clear();
  locale.setPreference("zh-CN");
  await page.viewport(1024, 900);
});

test("ConnectionGate switches language live while preserving typed token draft", async () => {
  render(ConnectionGate, {
    checking: false,
    token: "",
    error: "",
    onconnect: () => {},
  });

  await expect.element(page.getByText("你的终端，始终在线。")).toBeVisible();
  const tokenInput = page.getByLabelText("共享 Token");
  await tokenInput.fill("secret-token-draft");

  await page.getByRole("radio", { name: "English", exact: true }).click();

  expect(document.documentElement.lang).toBe("en");
  expect(localStorage.getItem("webmux.locale")).toBe("en");
  await expect
    .element(page.getByText("Your terminal, always online."))
    .toBeVisible();
  await expect
    .element(
      page.getByRole("button", { name: "Connect to WebMux →", exact: true }),
    )
    .toBeVisible();
  await expect
    .element(page.getByLabelText("Shared Token"))
    .toHaveValue("secret-token-draft");

  // Clearing host-scoped storage preserves device-scoped locale preference.
  setActiveHostId("remote-a");
  store("token", "ephemeral");
  clearHostStorage("remote-a");
  expect(stored("token")).toBe("");
  expect(stored("locale")).toBe("en");
  setActiveHostId(null);

  await page.getByRole("radio", { name: "简体中文", exact: true }).click();
  expect(document.documentElement.lang).toBe("zh-CN");
  await expect.element(page.getByText("你的终端，始终在线。")).toBeVisible();
});

test("Settings switches language live even when server settings are unavailable", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async () =>
      Response.json({ error: "unavailable" }, { status: 503 }),
    ),
  );
  stopSettings();
  await reloadSettings();

  render(Settings);
  const group = page.getByRole("radiogroup", { name: "语言", exact: true });
  await expect.element(group).toBeVisible();

  await group.getByRole("radio", { name: "English", exact: true }).click();

  expect(document.documentElement.lang).toBe("en");
  await expect
    .element(
      page.getByRole("navigation", { name: "Settings sections", exact: true }),
    )
    .toBeVisible();
  await expect
    .element(page.getByRole("link", { name: "Appearance", exact: true }))
    .toBeVisible();
  await expect
    .element(page.getByRole("link", { name: "Launch configs", exact: true }))
    .toBeVisible();

  // Now verify Follow system reacts to browser language changes.
  vi.spyOn(navigator, "languages", "get").mockReturnValue(["en-US", "en"]);
  await page
    .getByRole("radiogroup", { name: "Language", exact: true })
    .getByRole("radio", { name: "Follow system", exact: true })
    .click();
  expect(localStorage.getItem("webmux.locale")).toBeNull();
  expect(locale.current).toBe("en");

  vi.spyOn(navigator, "languages", "get").mockReturnValue(["zh-CN"]);
  window.dispatchEvent(new Event("languagechange"));
  expect(locale.current).toBe("zh-CN");
  await expect
    .element(page.getByRole("navigation", { name: "设置分区", exact: true }))
    .toBeVisible();

  // Verify synced server settings never write locale to /api/settings.
  const patchCalls: unknown[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input, init) => {
      const path = new URL(String(input), location.href).pathname;
      if (path === "/api/settings" && init?.method === "PATCH") {
        patchCalls.push(init.body);
      }
      if (path === "/api/settings") {
        return Response.json({
          revision: 0,
          updatedAt: "",
          values: emptySettings,
        });
      }
      if (path === "/api/launch-configs") return Response.json([]);
      if (path === "/api/config") {
        return Response.json({ defaultCwd: "/tmp", portDetection: true });
      }
      return Response.json({ error: "unavailable" }, { status: 503 });
    }),
  );
  await reloadSettings();
  await page.getByRole("radio", { name: "English", exact: true }).click();
  expect(patchCalls).toEqual([]);
});

test("InitInstructionField renders English preset labels while inserting canonical reference tokens", async () => {
  locale.setPreference("en");
  render(InitInstructionField, {
    id: "repo-init",
    value: "",
    commands: QUICK_COMMAND_PRESETS.map((preset) => ({ ...preset })),
  });

  const chip = page.getByRole("button", {
    name: "Insert WebMux tools",
    exact: true,
  });
  await expect.element(chip).toBeVisible();
  await expect.element(chip).toHaveTextContent("WebMux tools");
  await expect.element(chip).toHaveAttribute("title", "Insert {{WebMux 工具}}");
  await chip.click();
  await expect
    .element(page.getByRole("textbox", { name: "Init instruction" }))
    .toHaveValue("{{WebMux 工具}}");
});
