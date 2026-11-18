import { describe, expect, it } from "vite-plus/test";
import { GitWorkingChangeSchema } from "../src/contracts/git-workbench-schemas";
import { SessionInfoSchema } from "../src/contracts/schemas";
import {
  joinCommitMessageDraft,
  mergeCommitMessageDrafts,
  splitCommitMessageDraft,
} from "../src/domains/workspace/git-commit-draft-model";
import { filterActiveWorktreeSessions } from "../src/domains/workspace/git-workbench-adapter";
import { isChangeReviewed } from "../src/domains/workspace/git-workbench-state";
import {
  computeLocalReviewProgress,
  findNextUnreviewedTaskFile,
  findNextUnreviewedWorkingChange,
} from "../src/hooks/workspace/git-review-sync.svelte";

describe("git commit draft model", () => {
  it("splits and rejoins subject and body without losing multiline content", () => {
    const raw = "feat(git): sync commit draft\n\n- optimistic lock\n- 409 card";
    const parsed = splitCommitMessageDraft(raw);
    expect(parsed).toEqual({
      subject: "feat(git): sync commit draft",
      body: "- optimistic lock\n- 409 card",
    });
    expect(joinCommitMessageDraft(parsed.subject, parsed.body)).toBe(raw);
  });

  it("merges local and remote drafts preserving both sides when distinct", () => {
    const merged = mergeCommitMessageDrafts(
      { subject: "feat(git): local subject", body: "local note A" },
      { subject: "feat(git): remote subject", body: "remote note B" },
    );
    expect(merged.subject).toBe(
      "feat(git): local subject / feat(git): remote subject",
    );
    expect(merged.body).toContain("--- 本机当前输入 ---\nlocal note A");
    expect(merged.body).toContain("--- 另一设备已保存版本 ---\nremote note B");
  });
});

describe("git review progress & next unreviewed jump", () => {
  const changes = [
    GitWorkingChangeSchema.parse({
      id: "staged:a.go",
      path: "a.go",
      group: "staged",
      status: "M",
      hunkStageAvailable: true,
      identityKey: "v1",
    }),
    GitWorkingChangeSchema.parse({
      id: "unstaged:b.go",
      path: "b.go",
      group: "unstaged",
      status: "M",
      hunkStageAvailable: true,
      identityKey: "v1",
    }),
    GitWorkingChangeSchema.parse({
      id: "untracked:c.go",
      path: "c.go",
      group: "untracked",
      status: "?",
      hunkStageAvailable: false,
      identityKey: "v1",
    }),
  ];

  it("computes local review progress and wraps around to next unreviewed item", () => {
    const reviewedPaths = ["staged:a.go::v=v1"];
    const summary = computeLocalReviewProgress(
      "uncommitted",
      changes,
      [],
      reviewedPaths,
      isChangeReviewed,
    );
    expect(summary).toEqual({ total: 3, reviewed: 1, remaining: 2 });

    const nextFromB = findNextUnreviewedWorkingChange(
      changes,
      "unstaged:b.go",
      (c) => isChangeReviewed(reviewedPaths, c),
    );
    expect(nextFromB?.id).toBe("untracked:c.go");

    const wrappedFromC = findNextUnreviewedWorkingChange(
      changes,
      "untracked:c.go",
      (c) => isChangeReviewed(reviewedPaths, c),
    );
    expect(wrappedFromC?.id).toBe("unstaged:b.go");
  });

  it("auto-invalidates local review token when identityKey changes", () => {
    const reviewedPaths = ["unstaged:b.go::v=v1"];
    expect(isChangeReviewed(reviewedPaths, changes[1]!)).toBe(true);
    expect(
      isChangeReviewed(reviewedPaths, { ...changes[1]!, identityKey: "v2" }),
    ).toBe(false);
  });

  it("finds next unreviewed task baseline file", () => {
    const taskFiles = [
      {
        oldPath: "",
        newPath: "one.ts",
        status: "M" as const,
        binary: false,
        additions: 2,
        deletions: 1,
        hunks: [],
      },
      {
        oldPath: "",
        newPath: "two.ts",
        status: "A" as const,
        binary: false,
        additions: 5,
        deletions: 0,
        hunks: [],
      },
    ];
    const next = findNextUnreviewedTaskFile(
      taskFiles,
      "one.ts",
      (p) => p === "one.ts",
    );
    expect(next).toBe("two.ts");
  });
});

describe("worktree active session filtering", () => {
  it("filters active non-exited sessions matching worktreeId or worktreePath", () => {
    const baseSess = {
      generation: "0123456789abcdef",
      title: "Agent",
      cwd: "/other",
      cmd: ["claude"],
      exitCode: null,
      listeningPorts: [],
      views: ["terminal" as const],
      view: "terminal" as const,
      transition: null,
      agent: null,
      preview: "",
      createdAt: "2026-04-12T08:00:00Z",
      lastActiveAt: "2026-04-12T08:01:00Z",
      cols: 80,
      rows: 24,
      clients: 1,
      attention: null,
      reportedCwd: null,
    };
    const sessions = [
      SessionInfoSchema.parse({
        ...baseSess,
        id: "aaaaaaa2",
        status: "running",
        state: "active",
        workspace: {
          repositoryId: "repo-1",
          repositoryName: "webmux",
          worktreeId: "wt-feat",
          worktreePath: "/work/webmux.worktrees/wt-feat",
          branch: "feat/v1-collab",
        },
      }),
      SessionInfoSchema.parse({
        ...baseSess,
        id: "bbbbbbb3",
        status: "exited",
        state: "active",
        workspace: {
          repositoryId: "repo-1",
          repositoryName: "webmux",
          worktreeId: "wt-feat",
          worktreePath: "/work/webmux.worktrees/wt-feat",
          branch: "feat/v1-collab",
        },
      }),
      SessionInfoSchema.parse({
        ...baseSess,
        id: "ccccccc4",
        status: "running",
        state: "active",
        workspace: {
          repositoryId: "repo-1",
          repositoryName: "webmux",
          worktreeId: "wt-main",
          worktreePath: "/work/webmux",
          branch: "main",
        },
      }),
    ];
    expect(
      filterActiveWorktreeSessions(
        sessions,
        "wt-feat",
        "/work/webmux.worktrees/wt-feat",
      ).map((s) => s.id),
    ).toEqual(["aaaaaaa2"]);
  });
});
