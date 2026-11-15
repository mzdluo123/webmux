import { afterEach, expect, test } from "vite-plus/test";
import {
  CapabilitiesSchema,
  PluginIDSchema,
} from "../../src/contracts/plugin-schemas";
import {
  pluginAvailable,
  pluginEnabled,
  resetPluginCapabilities,
  setPluginCapabilities,
} from "../../src/platform/plugin-host.svelte";

afterEach(resetPluginCapabilities);

test("optional plugins remain denied until an authenticated snapshot is accepted", () => {
  resetPluginCapabilities();
  for (const id of PluginIDSchema.options)
    expect(pluginEnabled(id)).toBe(false);
  expect(pluginAvailable("terminal.pty")).toBe(false);
});

test("enabled selection and runtime availability are distinct facts", () => {
  const snapshot = CapabilitiesSchema.parse({
    version: 1,
    plugins: PluginIDSchema.options.map((id) =>
      id === "code"
        ? {
            id,
            enabled: true,
            status: "unavailable",
            reason: "service_unavailable",
          }
        : { id, enabled: false, status: "disabled" },
    ),
    capabilities: [
      { id: "terminal.pty", available: true },
      { id: "code.browse", available: false, reason: "service_unavailable" },
    ],
  });
  setPluginCapabilities(snapshot);
  expect(pluginEnabled("code")).toBe(true);
  expect(pluginAvailable("code.browse")).toBe(false);
  expect(pluginAvailable("terminal.pty")).toBe(true);
  expect(pluginAvailable("unknown.capability")).toBe(false);
  resetPluginCapabilities();
  expect(pluginEnabled("code")).toBe(false);
  expect(pluginAvailable("terminal.pty")).toBe(false);
});
