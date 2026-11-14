import { describe, expect, test } from "vite-plus/test";
import {
  buildGitRouteHref,
  forwardAuthNext,
  matchRoute,
  parseGitRouteQuery,
} from "../src/hooks/shared/route";

describe("route matching", () => {
  test("matches a valid session id", () => {
    expect(matchRoute("/s/abcdefgh")).toEqual({
      name: "terminal",
      id: "abcdefgh",
    });
    expect(matchRoute("/s/234567aa")).toEqual({
      name: "terminal",
      id: "234567aa",
    });
  });

  test("matches a valid code window id", () => {
    expect(matchRoute("/code/abcdefgh")).toEqual({
      name: "code",
      id: "abcdefgh",
    });
  });

  test("matches the settings pages", () => {
    expect(matchRoute("/settings")).toEqual({ name: "settings" });
    expect(matchRoute("/settings/tailscale")).toEqual({
      name: "settings-tailscale",
    });
    expect(matchRoute("/settings/tunnel")).toEqual({ name: "settings-tunnel" });
  });

  test("matches the forward authentication page", () => {
    expect(matchRoute("/forward-auth")).toEqual({ name: "forward-auth" });
    expect(matchRoute("/forward-auth/")).toEqual({ name: "sessions" });
    // Gateway paths never reach the SPA as routes of their own.
    expect(matchRoute("/forward/5173/")).toEqual({ name: "sessions" });
  });

  test("falls back to the session list", () => {
    expect(matchRoute("/")).toEqual({ name: "sessions" });
    expect(matchRoute("/unknown")).toEqual({ name: "sessions" });
  });

  test("keeps the current trailing-slash behavior", () => {
    expect(matchRoute("/settings/")).toEqual({ name: "sessions" });
    expect(matchRoute("/settings/tailscale/")).toEqual({ name: "sessions" });
    expect(matchRoute("/settings/tunnel/")).toEqual({ name: "sessions" });
    expect(matchRoute("/s/abcdefgh/")).toEqual({ name: "sessions" });
    expect(matchRoute("/code/abcdefgh/")).toEqual({ name: "sessions" });
  });

  test("rejects ids outside the base32 alphabet", () => {
    expect(matchRoute("/s/ABCDEFGH")).toEqual({ name: "sessions" });
    expect(matchRoute("/s/12345678")).toEqual({ name: "sessions" });
    expect(matchRoute("/s/abcdefg")).toEqual({ name: "sessions" });
    expect(matchRoute("/code/ABCDEFGH")).toEqual({ name: "sessions" });
  });

  test("matches standalone /git routes and rejects malformed segments", () => {
    expect(matchRoute("/git")).toEqual({ name: "git", repositoryId: "" });
    expect(matchRoute("/git/repo_01")).toEqual({
      name: "git",
      repositoryId: "repo_01",
    });
    const hexRepo = "0123456789abcdef0123456789abcdef";
    const hexWt = "fedcba9876543210fedcba9876543210";
    expect(matchRoute(`/git/${hexRepo}/worktrees/${hexWt}`)).toEqual({
      name: "git",
      repositoryId: hexRepo,
      worktreeId: hexWt,
    });
    expect(matchRoute("/git/")).toEqual({ name: "sessions" });
    expect(matchRoute("/git/repo_01/")).toEqual({ name: "sessions" });
    expect(matchRoute("/git/repo_01/worktrees/")).toEqual({ name: "sessions" });
    expect(matchRoute("/git/../etc")).toEqual({ name: "sessions" });
  });
});

describe("git workbench query contract", () => {
  test("parses deep-link query params including scope=task", () => {
    expect(
      parseGitRouteQuery(
        "?view=commit&scope=task&branch=feat%2Fshell&commit=0123456789abcdef&q=router&returnSession=abcdefgh",
      ),
    ).toEqual({
      view: "commit",
      scope: "task",
      branch: "feat/shell",
      commit: "0123456789abcdef",
      q: "router",
      returnSession: "abcdefgh",
    });
    expect(
      buildGitRouteHref({
        repositoryId: "repo-1",
        worktreeId: "wt-2",
        view: "log",
        scope: "task",
      }),
    ).toBe("/git/repo-1/worktrees/wt-2?view=log&scope=task");
    expect(
      parseGitRouteQuery("?view=invalid&scope=nope&returnSession=BAD"),
    ).toEqual({});
  });

  test("builds canonical /git hrefs preserving explicit view and omitting default scope", () => {
    expect(buildGitRouteHref({ repositoryId: "" })).toBe("/git");
    expect(buildGitRouteHref({ repositoryId: "repo-1", scope: "all" })).toBe(
      "/git/repo-1",
    );
    expect(
      buildGitRouteHref({ repositoryId: "repo-1", view: "log", scope: "all" }),
    ).toBe("/git/repo-1?view=log");
    expect(
      buildGitRouteHref({
        repositoryId: "repo-1",
        worktreeId: "wt-2",
        view: "commit",
        scope: "to-push",
        branch: "feat/route",
        returnSession: "abcdefgh",
      }),
    ).toBe(
      "/git/repo-1/worktrees/wt-2?view=commit&scope=to-push&branch=feat%2Froute&returnSession=abcdefgh",
    );
  });
});

describe("forward authentication return address", () => {
  test("accepts gateway paths with their query", () => {
    expect(forwardAuthNext("?next=%2Fforward%2F5173%2F")).toBe(
      "/forward/5173/",
    );
    expect(
      forwardAuthNext(
        `?next=${encodeURIComponent("/forward/5173/app?page=2&q=a b")}`,
      ),
    ).toBe("/forward/5173/app?page=2&q=a b");
  });

  test("rejects anything that could leave the gateway", () => {
    for (const next of [
      "",
      "/",
      "/api/sessions",
      "/forwardx/5173/",
      "https://evil.example/forward/1/",
      "//evil.example/",
      "/forward//evil.example/",
      "/forward/5173/\\evil",
      "/forward/5173/\r\nX: 1",
    ])
      expect(forwardAuthNext(`?next=${encodeURIComponent(next)}`)).toBeNull();
    expect(forwardAuthNext("")).toBeNull();
  });
});
