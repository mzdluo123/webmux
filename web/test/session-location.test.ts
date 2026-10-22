import { describe, expect, test } from "vitest";
import {
  locationHints,
  matchesSessionQuery,
  resolveSessionPath,
} from "../src/session-location";
import type { ReportedCwd, SessionInfo } from "../src/schemas";

const reported = (path: string, remote = false, host = ""): ReportedCwd => ({
  path,
  host,
  remote,
  since: "2026-10-22T00:00:00Z",
});

describe("home-page location hints", () => {
  test("does not repeat a launch directory already shown by the group", () => {
    const hints = locationHints(
      { cwd: "/repo/worktree", reportedCwd: null },
      "/repo/worktree",
    );
    expect(hints).toEqual({ launch: null, reported: null });
  });

  test("shows a launch directory below the worktree root", () => {
    expect(
      locationHints(
        { cwd: "/repo/worktree/src", reportedCwd: null },
        "/repo/worktree",
      ).launch,
    ).toBe("/repo/worktree/src");
  });

  test("shows the shell directory only when it changes or is remote", () => {
    expect(
      locationHints({ cwd: "/repo", reportedCwd: reported("/repo") }).reported,
    ).toBeNull();
    expect(
      locationHints({ cwd: "/repo", reportedCwd: reported("/repo/src") })
        .reported?.path,
    ).toBe("/repo/src");
    expect(
      locationHints({
        cwd: "/repo",
        reportedCwd: reported("/repo", true, "build.example"),
      }).reported?.remote,
    ).toBe(true);
  });

  test("a report never replaces the launch directory used for grouping", () => {
    const hints = locationHints(
      { cwd: "/repo/worktree", reportedCwd: reported("/elsewhere") },
      "/repo/worktree",
    );
    expect(hints.launch).toBeNull();
    expect(hints.reported?.path).toBe("/elsewhere");
  });
});

describe("session search", () => {
  const session = {
    title: "Build agent",
    cwd: "/work",
    reportedCwd: reported("/srv/build", true, "build.example"),
  } as SessionInfo;

  test("matches the shell directory and host", () => {
    expect(matchesSessionQuery(session, "/srv")).toBe(true);
    expect(matchesSessionQuery(session, "build.example")).toBe(true);
    expect(matchesSessionQuery(session, "repository", "repository")).toBe(true);
    expect(
      matchesSessionQuery(session, "feature/x", undefined, "feature/x"),
    ).toBe(true);
  });

  test("does not match unrelated text", () => {
    expect(matchesSessionQuery(session, "missing")).toBe(false);
  });
});

describe("resolving a path from the shell directory", () => {
  const root = "/repo/worktree";

  test("resolves relative and absolute paths inside the root", () => {
    expect(resolveSessionPath("/repo/worktree/src", "main.go", root)).toEqual({
      ok: true,
      path: "/repo/worktree/src/main.go",
      relative: "src/main.go",
    });
    expect(
      resolveSessionPath("/repo/worktree/src", "../README.md", root),
    ).toEqual({
      ok: true,
      path: "/repo/worktree/README.md",
      relative: "README.md",
    });
    expect(resolveSessionPath("/tmp", "/repo/worktree/a/b.ts", root)).toEqual({
      ok: true,
      path: "/repo/worktree/a/b.ts",
      relative: "a/b.ts",
    });
  });

  test("rejects paths outside the code root", () => {
    expect(
      resolveSessionPath("/repo/worktree/src", "../../outside", root),
    ).toEqual({ ok: false, reason: "outside" });
    expect(resolveSessionPath("/repo/worktree", "/etc/passwd", root)).toEqual({
      ok: false,
      reason: "outside",
    });
  });

  test("rejects empty, root and invalid inputs", () => {
    expect(resolveSessionPath(root, " ", root)).toEqual({
      ok: false,
      reason: "empty",
    });
    expect(resolveSessionPath(root, ".", root)).toEqual({
      ok: false,
      reason: "root",
    });
    expect(resolveSessionPath(root, "bad\u0000name", root)).toEqual({
      ok: false,
      reason: "invalid",
    });
  });
});
