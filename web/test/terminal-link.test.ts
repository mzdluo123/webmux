import { afterEach, describe, expect, test, vi } from "vite-plus/test";
import {
  openTerminalExternalLink,
  parseTerminalExternalLink,
} from "../src/domains/terminal/terminal-link";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("terminal external OSC 8 links", () => {
  test("accepts http(s) links and normalizes IDN hostnames to Punycode", () => {
    expect(
      parseTerminalExternalLink(
        "  https://github.com/mzdluo123/webmux/pull/657 ",
      ),
    ).toBe("https://github.com/mzdluo123/webmux/pull/657");
    expect(parseTerminalExternalLink("http://127.0.0.1:8080/health?x=1")).toBe(
      "http://127.0.0.1:8080/health?x=1",
    );
    expect(parseTerminalExternalLink("https://аррӏе.com/login")).toBe(
      "https://xn--80ak6aa92e.com/login",
    );
  });

  test("rejects non-http protocols, relative paths and malformed URLs", () => {
    for (const candidate of [
      "",
      "   ",
      "javascript:alert(1)",
      "JAVASCRIPT:alert(1)",
      "data:text/html,<script>alert(1)</script>",
      "file:///etc/passwd",
      "ftp://example.com/archive.tar.gz",
      "/sessions/abc",
      "not a url",
    ]) {
      expect(parseTerminalExternalLink(candidate)).toBeNull();
    }
  });

  test("opens validated links in a detached tab with noopener and noreferrer", () => {
    const openSpy = vi.fn(() => null);
    vi.stubGlobal("window", { open: openSpy });

    expect(
      openTerminalExternalLink("https://github.com/mzdluo123/webmux/pull/657"),
    ).toBe(true);
    expect(openSpy).toHaveBeenCalledTimes(1);
    expect(openSpy).toHaveBeenCalledWith(
      "https://github.com/mzdluo123/webmux/pull/657",
      "_blank",
      "noopener,noreferrer",
    );

    expect(openTerminalExternalLink("javascript:alert(1)")).toBe(false);
    expect(openSpy).toHaveBeenCalledTimes(1);
  });
});
