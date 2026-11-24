import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { consumeAuthFragment } from "../src/platform/auth-api";
import {
  defaultDeviceLabel,
  localhostRedirectUrl,
} from "../src/platform/webauthn";

describe("webauthn platform helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("maps loopback IP URLs to localhost while preserving path, query, and fragment", () => {
    expect(
      localhostRedirectUrl({
        protocol: "http:",
        hostname: "127.0.0.1",
        port: "7681",
        pathname: "/onboarding",
        search: "",
        hash: "#setup=ABCD-EFGH-JKLM",
      } as Location),
    ).toBe("http://localhost:7681/onboarding#setup=ABCD-EFGH-JKLM");

    expect(
      localhostRedirectUrl({
        protocol: "http:",
        hostname: "::1",
        port: "7681",
        pathname: "/pair",
        search: "",
        hash: "#code=1234",
      } as Location),
    ).toBe("http://localhost:7681/pair#code=1234");

    expect(
      localhostRedirectUrl({
        protocol: "http:",
        hostname: "localhost",
        port: "7681",
        pathname: "/",
        search: "",
        hash: "",
      } as Location),
    ).toBeNull();

    expect(
      localhostRedirectUrl({
        protocol: "https:",
        hostname: "webmux.example.com",
        port: "",
        pathname: "/",
        search: "",
        hash: "",
      } as Location),
    ).toBeNull();
  });

  it("consumes and strips #setup= and #code= fragments from location", () => {
    const loc = {
      pathname: "/onboarding",
      search: "",
      hash: "#setup=WXYZ-1234-ABCD",
    };
    const replaceState = vi.fn(
      (_state: unknown, _title: string, url: string) => {
        const idx = url.indexOf("#");
        loc.hash = idx >= 0 ? url.slice(idx) : "";
      },
    );
    vi.stubGlobal("location", loc);
    vi.stubGlobal("history", { state: null, replaceState });

    expect(consumeAuthFragment("#setup=")).toBe("WXYZ-1234-ABCD");
    expect(replaceState).toHaveBeenCalledWith(null, "", "/onboarding");
    expect(loc.hash).toBe("");

    loc.pathname = "/pair";
    loc.hash = "#code=PAIR-CODE-99";
    expect(consumeAuthFragment("#setup=")).toBe("");
    expect(consumeAuthFragment("#code=")).toBe("PAIR-CODE-99");
    expect(loc.hash).toBe("");
  });

  it("returns a non-empty device label", () => {
    expect(defaultDeviceLabel().length).toBeGreaterThan(0);
  });
});
