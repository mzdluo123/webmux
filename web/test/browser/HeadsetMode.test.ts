import { afterEach, expect, test, vi } from "vitest";
import { headsetMode } from "../../src/hooks/shared/headset-mode.svelte";
import { deviceToggle } from "../../src/hooks/shared/device-preferences.svelte";
import { hasTouchInput } from "../../src/platform/touch-input";
import { setActiveHostId, stored } from "../../src/platform/storage";

const saved = localStorage.getItem("webmux.headsetMode");

afterEach(() => {
  vi.restoreAllMocks();
  headsetMode.enabled = false;
  setActiveHostId(null);
  if (saved === null) localStorage.removeItem("webmux.headsetMode");
  else localStorage.setItem("webmux.headsetMode", saved);
});

test("headset presentation is opt-in and shared across hosts on this device", () => {
  localStorage.removeItem("webmux.headsetMode");
  expect(deviceToggle("headsetMode", false).enabled).toBe(false);
  localStorage.setItem("webmux.headsetMode", "unknown");
  expect(deviceToggle("headsetMode", false).enabled).toBe(false);

  setActiveHostId("headset-a");
  headsetMode.enabled = true;
  expect(localStorage.getItem("webmux.headsetMode")).toBe("1");
  expect(localStorage.getItem("webmux.host.headset-a.headsetMode")).toBeNull();
  setActiveHostId("headset-b");
  expect(stored("headsetMode")).toBe("1");
  expect(deviceToggle("headsetMode", false).enabled).toBe(true);
});

test.each([true, false])(
  "headset presentation never changes coarse-pointer capability %s",
  (coarse) => {
    const matchMedia = window.matchMedia.bind(window);
    vi.spyOn(window, "matchMedia").mockImplementation((query) => {
      const result = matchMedia(query);
      if (query === "(any-pointer: coarse)")
        Object.defineProperty(result, "matches", { value: coarse });
      return result;
    });
    headsetMode.enabled = false;
    expect(hasTouchInput()).toBe(coarse);
    headsetMode.enabled = true;
    expect(hasTouchInput()).toBe(coarse);
  },
);
