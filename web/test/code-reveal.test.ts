import { afterEach, describe, expect, test } from "vitest";
import { checkRevealTarget } from "../src/domains/code/code-reveal";
import { setClientTransport } from "../src/platform/transport";

const context = {
  contextId: "ctx",
  rootPath: "/repo",
  rootKind: "worktree",
  browseAvailable: true,
  diffAvailable: true,
  ignoreAvailable: true,
  reason: "",
  writableAvailable: true,
};

function useTransport(locate: (path: string) => Response | Promise<Response>) {
  const paths: string[] = [];
  setClientTransport({
    origin: "http://test",
    fetch: async (path: string) => {
      paths.push(path);
      if (path === "/sessions/s/code/context") return Response.json(context);
      if (path.startsWith("/sessions/s/code/locate?")) return locate(path);
      return new Response("not found", {
        status: 404,
        statusText: "not found",
      });
    },
    upload: () => Promise.reject(new Error("unused")),
    openStream: () => {
      throw new Error("unused");
    },
  });
  return paths;
}

afterEach(() => setClientTransport(null));

describe("checkRevealTarget", () => {
  test("reports targets outside the workspace root for in-place notices", async () => {
    useTransport(
      () => new Response("forbidden", { status: 403, statusText: "forbidden" }),
    );
    await expect(
      checkRevealTarget("/sessions/s/code", {
        path: "/repo.worktrees/sync/cloudbuild.release.yaml",
        rawPath: "/repo.worktrees/sync/cloudbuild.release.yaml:129",
        line: 129,
      }),
    ).resolves.toBe("链接目标不在工作区内");
  });

  test("reports missing targets and resolves clean on success", async () => {
    const paths = useTransport((path) =>
      path.includes("cloudbuild")
        ? Response.json({
            nodeId: "n1",
            path: "/repo/cloudbuild.release.yaml",
            kind: "file",
          })
        : new Response("gone", { status: 404, statusText: "gone" }),
    );
    await expect(
      checkRevealTarget("/sessions/s/code", { path: "missing.yaml" }),
    ).resolves.toBe("链接目标不存在");
    await expect(
      checkRevealTarget("/sessions/s/code", {
        path: "/repo/cloudbuild.release.yaml",
      }),
    ).resolves.toBe("");
    expect(paths[0]).toBe("/sessions/s/code/context");
  });

  test("defers to the panel when the context request itself fails", async () => {
    setClientTransport({
      origin: "http://test",
      fetch: async () =>
        new Response("down", { status: 500, statusText: "down" }),
      upload: () => Promise.reject(new Error("unused")),
      openStream: () => {
        throw new Error("unused");
      },
    });
    await expect(
      checkRevealTarget("/sessions/s/code", { path: "/repo/x.ts" }),
    ).resolves.toBe("");
  });
});
