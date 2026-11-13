import { describe, expect, test } from "vitest";
import {
  GitBranchItemSchema,
  GitWorktreeContextSchema,
} from "../src/contracts/git-workbench-schemas";
import {
  GitCommitPreviewSchema,
  GitStagingOverviewSchema,
} from "../src/contracts/schemas";
import {
  groupAndSortGitBranches,
  resolveGitBranchActions,
  resolveGitBranchUpstream,
} from "../src/domains/workspace/git-branches-model";
import {
  mapGitCommitPreviewResponse,
  mapGitStagingOverview,
} from "../src/domains/workspace/git-workbench-adapter";
import {
  isChangeReviewed,
  toggleReviewedChange,
} from "../src/domains/workspace/git-workbench-state";

function makeContext(overrides: Record<string, unknown> = {}) {
  return GitWorktreeContextSchema.parse({
    repositoryId: "repo-1",
    repositoryName: "webmux",
    worktreeId: "wt-main",
    worktreePath: "/work/webmux",
    worktreeKind: "main",
    checkedOutBranch: "main",
    headOid: "1111111111111111111111111111111111111111",
    headState: "branch",
    ahead: 2,
    behind: 0,
    upstream: "origin/main",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    signEnabled: false,
    executableHooks: [],
    ...overrides,
  });
}

function makeBranch(overrides: Record<string, unknown>) {
  return GitBranchItemSchema.parse({
    name: "main",
    kind: "local",
    head: "1111111111111111111111111111111111111111",
    shortHead: "1111111",
    subject: "init",
    committedAt: "2026-04-12T08:00:00Z",
    ...overrides,
  });
}

describe("version-bound reviewed state (A03)", () => {
  test("invalidates reviewed mark automatically when file identityKey changes", () => {
    const v1 = {
      id: "unstaged:web/src/app.ts",
      path: "web/src/app.ts",
      identityKey: "oid-1:oid-2:100644:100644",
    };
    const v2 = {
      id: "unstaged:web/src/app.ts",
      path: "web/src/app.ts",
      identityKey: "oid-1:oid-3:100644:100644",
    };

    const reviewed = toggleReviewedChange([], v1);
    expect(isChangeReviewed(reviewed, v1)).toBe(true);
    expect(isChangeReviewed(reviewed, v2)).toBe(false);

    const reviewedV2 = toggleReviewedChange(reviewed, v2);
    expect(reviewedV2).toHaveLength(1);
    expect(isChangeReviewed(reviewedV2, v2)).toBe(true);
    expect(isChangeReviewed(reviewedV2, v1)).toBe(false);
    expect(toggleReviewedChange(reviewedV2, v2)).toEqual([]);
  });
});

describe("shared branch upstream & action rules (UI-05, U08)", () => {
  test("never reports fake 0 for missing, deleted, or uncounted upstreams", () => {
    const noUpstream = resolveGitBranchUpstream(
      makeBranch({ name: "feat/local-only" }),
    );
    expect(noUpstream.kind).toBe("none");
    expect(noUpstream.badgeText).toBe("无上游");
    expect(noUpstream.ahead).toBeUndefined();
    expect(noUpstream.behind).toBeUndefined();

    const gone = resolveGitBranchUpstream(
      makeBranch({
        name: "feat/gone",
        upstream: "origin/feat/gone",
        upstreamGone: true,
      }),
    );
    expect(gone.kind).toBe("gone");
    expect(gone.badgeText).toBe("上游已删");
    expect(gone.ahead).toBeUndefined();

    const uncounted = resolveGitBranchUpstream(
      makeBranch({
        name: "feat/uncounted",
        upstream: "origin/feat/uncounted",
      }),
    );
    expect(uncounted.kind).toBe("uncounted");
    expect(uncounted.badgeText).toBe("待统计");
    expect(uncounted.ahead).toBeUndefined();

    const diverged = resolveGitBranchUpstream(
      makeBranch({
        name: "main",
        upstream: "origin/main",
        ahead: 2,
        behind: 3,
      }),
    );
    expect(diverged.kind).toBe("diverged");
    expect(diverged.badgeText).toBe("↑2 ↓3");
  });

  test("sorts current checked-out and favorite branches first and protects occupied worktrees", () => {
    const context = makeContext();
    const branches = [
      makeBranch({
        name: "origin/main",
        kind: "remote",
        head: "2222222222222222222222222222222222222222",
        shortHead: "2222222",
      }),
      makeBranch({
        name: "chore/plain",
        head: "3333333333333333333333333333333333333333",
        shortHead: "3333333",
      }),
      makeBranch({
        name: "feat/fav-occupied",
        head: "4444444444444444444444444444444444444444",
        shortHead: "4444444",
        favorite: true,
        occupiedBy: {
          worktreeId: "wt-2",
          worktreePath: "/work/wt-2",
          activeSessions: 2,
        },
      }),
      makeBranch({
        name: "main",
        upstream: "origin/main",
        ahead: 2,
        behind: 0,
      }),
    ];

    const grouped = groupAndSortGitBranches(branches, "main");
    expect(grouped.current.map((b) => b.name)).toEqual(["main"]);
    expect(grouped.favorites.map((b) => b.name)).toEqual(["feat/fav-occupied"]);
    expect(grouped.remote.map((b) => b.name)).toEqual(["origin/main"]);

    const occupiedActions = resolveGitBranchActions({
      item: branches[2]!,
      context,
      compareBaseOid: context.headOid,
    });
    expect(occupiedActions.some((a) => a.kind === "checkout")).toBe(false);
    expect(
      occupiedActions.some((a) => a.kind === "open-occupied-worktree"),
    ).toBe(true);
    const delOccupied = occupiedActions.find((a) => a.kind === "delete");
    expect(delOccupied?.disabled).toBe(true);
    expect(delOccupied?.disabledReason).toContain("另一工作树");

    const currentActions = resolveGitBranchActions({
      item: branches[3]!,
      context,
    });
    expect(currentActions.find((a) => a.kind === "push")?.disabled).toBeFalsy();
    const delCurrent = currentActions.find((a) => a.kind === "delete");
    expect(delCurrent?.disabled).toBe(true);
    expect(delCurrent?.disabledReason).toContain("当前工作树");
  });
});

describe("staging & commit preview adapter (A01, A12)", () => {
  test("detects CLI pre-staged entries and injects cli_prestaged warning in preview", () => {
    const overview = GitStagingOverviewSchema.parse({
      unborn: false,
      headOid: "1111111111111111111111111111111111111111",
      conflicted: [],
      staged: [
        {
          area: "staged",
          path: "cli-only.go",
          status: "M",
          hunkStageAvailable: true,
          identity: {
            area: "staged",
            path: "cli-only.go",
            baseOid: "aaaa",
            targetOid: "bbbb",
          },
        },
        {
          area: "staged",
          path: "ui-staged.go",
          status: "A",
          hunkStageAvailable: true,
          identity: {
            area: "staged",
            path: "ui-staged.go",
            targetOid: "cccc",
          },
        },
      ],
      unstaged: [],
    });

    const mapped = mapGitStagingOverview(
      overview,
      new Map(),
      new Set(["ui-staged.go"]),
    );
    expect(
      mapped.find((m) => m.change.path === "cli-only.go")?.change
        .preStagedFromCli,
    ).toBe(true);
    expect(
      mapped.find((m) => m.change.path === "ui-staged.go")?.change
        .preStagedFromCli,
    ).toBe(false);

    const rawPreview = GitCommitPreviewSchema.parse({
      confirmationId: "confirm-1",
      expiresAt: "2026-04-12T09:00:00Z",
      repositoryId: "repo-1",
      worktreeId: "wt-main",
      worktreePath: "/work/webmux",
      headState: "branch",
      branch: "main",
      headOid: "1111111111111111111111111111111111111111",
      headParents: [],
      headPush: { state: "unpushed", branches: [], more: 0 },
      amend: false,
      amendPushed: false,
      signoff: false,
      expectedTreeOid: "tree-1",
      indexDigest: "sha256-1",
      identity: {
        authorName: "Alice",
        authorEmail: "alice@example.com",
        committerName: "Alice",
        committerEmail: "alice@example.com",
        signEnabled: false,
      },
      executableHooks: ["pre-commit"],
      stagedFiles: [
        {
          oldPath: "cli-only.go",
          newPath: "cli-only.go",
          status: "M",
          oldMode: "100644",
          newMode: "100644",
          oldOid: "aaaa",
          newOid: "bbbb",
          modeChanged: false,
          binary: false,
        },
      ],
      unstagedCount: 0,
      untrackedCount: 0,
      conflictedCount: 0,
      blockers: [],
      warnings: [],
    });

    const previewModel = mapGitCommitPreviewResponse(
      "webmux",
      rawPreview,
      mapped.map((m) => m.change),
      true,
    );
    expect(previewModel.warnings.some((w) => w.code === "cli_prestaged")).toBe(
      true,
    );
    expect(previewModel.stagedFiles[0]?.preStagedFromCli).toBe(true);
  });
});
