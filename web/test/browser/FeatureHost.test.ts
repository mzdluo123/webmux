import { afterEach, expect, test } from "vite-plus/test";
import {
  CapabilitiesSchema,
  FeatureIDSchema,
} from "../../src/contracts/feature-schemas";
import {
  featureAvailable,
  featureEnabled,
  resetFeatureCapabilities,
  setFeatureCapabilities,
} from "../../src/platform/feature-host.svelte";

afterEach(resetFeatureCapabilities);

test("optional features remain denied until an authenticated snapshot is accepted", () => {
  resetFeatureCapabilities();
  for (const id of FeatureIDSchema.options)
    expect(featureEnabled(id)).toBe(false);
  expect(featureAvailable("terminal.pty")).toBe(false);
});

test("enabled selection and runtime availability are distinct facts", () => {
  const snapshot = CapabilitiesSchema.parse({
    version: 2,
    features: FeatureIDSchema.options.map((id) =>
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
  setFeatureCapabilities(snapshot);
  expect(featureEnabled("code")).toBe(true);
  expect(featureAvailable("code.browse")).toBe(false);
  expect(featureAvailable("terminal.pty")).toBe(true);
  expect(featureAvailable("unknown.capability")).toBe(false);
  resetFeatureCapabilities();
  expect(featureEnabled("code")).toBe(false);
  expect(featureAvailable("terminal.pty")).toBe(false);
});
