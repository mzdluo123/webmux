import { afterEach, describe, expect, it, vi } from "vitest";
import {
  gitDeleteTaskBaseline,
  gitPutTaskBaseline,
  gitTaskBaseline,
  gitTaskChanges,
  gitTaskDiff,
} from "../src/platform/git-task-api";
import { gitConflictReason } from "../src/platform/git-api";
import { APIError } from "../src/platform/api";

describe("git-task-api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("calls GET/PUT/DELETE task-baseline and GET task-changes/task-diff with Zod validation", async () => {
    const calls: Array<{ url: string; method: string; body?: string }> = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = typeof input === "string" ? input : input.toString();
      const method = init?.method ?? "GET";
      const body = typeof init?.body === "string" ? init.body : undefined;
      calls.push({ url, method, body });

      if (url.endsWith("/task-baseline") && method === "DELETE") {
        return new Response(null, { status: 204 });
      }
      if (url.endsWith("/task-baseline")) {
        return Response.json({
          baseline: {
            baselineId: "tb_1",
            repositoryId: "repo-1",
            worktreeId: "wt-1",
            worktreePath: "/work/wt-1",
            baseOid: "a".repeat(40),
            unborn: false,
            capturedAt: "2026-04-12T10:00:00Z",
            source: "worktree_create",
            cleanAtStart: true,
            preExistingChanges: {
              staged: 0,
              unstaged: 0,
              untracked: 0,
              conflicted: 0,
            },
          },
        });
      }
      if (url.includes("/task-changes")) {
        return Response.json({
          status: "ready",
          baseline: {
            baselineId: "tb_1",
            repositoryId: "repo-1",
            worktreeId: "wt-1",
            worktreePath: "/work/wt-1",
            baseOid: "a".repeat(40),
            unborn: false,
            capturedAt: "2026-04-12T10:00:00Z",
            source: "worktree_create",
            cleanAtStart: true,
            preExistingChanges: {
              staged: 0,
              unstaged: 0,
              untracked: 0,
              conflicted: 0,
            },
          },
          headOid: "b".repeat(40),
          headUnborn: false,
          commitCount: 2,
          uncommittedCount: 0,
          changes: {
            entries: [],
            total: 0,
            nextCursor: "",
            contentVersion: "v1",
            checkedAt: "2026-04-12T10:05:00Z",
          },
        });
      }
      return Response.json({
        snapshotId: "snap-1",
        kind: "text",
        reason: "",
        checkedAt: "2026-04-12T10:05:00Z",
        change: {
          nodeId: "YS50eHQ",
          changeId: "cid-1",
          group: "task",
          oldPath: "a.txt",
          newPath: "a.txt",
          status: "M",
          oldOid: "a".repeat(40),
          newOid: "0".repeat(40),
          oldMode: "100644",
          newMode: "100644",
          reviewVersion: "rv-1",
        },
        old: {
          nodeId: "YS50eHQ",
          path: "a.txt",
          absolutePath: "/work/wt-1/a.txt",
          kind: "text",
          mediaType: "",
          size: 4,
          lineCount: 1,
          oid: "a".repeat(40),
          contentVersion: "cv-1",
          text: "old\n",
          reason: "",
          referenceable: true,
          downloadable: true,
          source: `commit:${"a".repeat(40)}`,
        },
        new: {
          nodeId: "YS50eHQ",
          path: "a.txt",
          absolutePath: "/work/wt-1/a.txt",
          kind: "text",
          mediaType: "",
          size: 4,
          lineCount: 1,
          oid: "",
          contentVersion: "cv-2",
          text: "new\n",
          reason: "",
          referenceable: true,
          downloadable: true,
          source: "worktree",
        },
      });
    });

    const getRes = await gitTaskBaseline("repo-1", "wt-1");
    expect(getRes.baseline?.baselineId).toBe("tb_1");

    await gitPutTaskBaseline("repo-1", "wt-1", { baseOid: "a".repeat(40) });
    const tc = await gitTaskChanges("repo-1", "wt-1", { q: "src" });
    expect(tc.commitCount).toBe(2);

    const td = await gitTaskDiff("repo-1", "wt-1", { path: "a.txt" });
    expect(td.change.group).toBe("task");

    await gitDeleteTaskBaseline("repo-1", "wt-1");
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual([
      "GET /api/repositories/repo-1/worktrees/wt-1/git/task-baseline",
      "PUT /api/repositories/repo-1/worktrees/wt-1/git/task-baseline",
      "GET /api/repositories/repo-1/worktrees/wt-1/git/task-changes?q=src",
      "GET /api/repositories/repo-1/worktrees/wt-1/git/task-diff?path=a.txt",
      "DELETE /api/repositories/repo-1/worktrees/wt-1/git/task-baseline",
    ]);
  });

  it("recognizes no_task_baseline in gitConflictReason", () => {
    const err = new APIError(409, "no baseline", "no_task_baseline");
    expect(gitConflictReason(err)).toBe("no_task_baseline");
  });
});
