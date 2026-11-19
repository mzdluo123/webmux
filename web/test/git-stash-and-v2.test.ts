import { afterEach, describe, expect, test, vi } from "vite-plus/test";
import {
  GitBranchItemSchema,
  GitHistoryRowSchema,
  type GitWorkbenchViewState,
  GitWorktreeContextSchema,
} from "../src/contracts/git-workbench-schemas";
import {
  GitInProgressOperationSchema,
  GitStashApplyResultSchema,
} from "../src/contracts/schemas";
import { resolveGitBranchActions } from "../src/domains/workspace/git-branches-model";
import { formatInProgressHeadline } from "../src/domains/workspace/git-divergence-model";
import { gitOperationKindLabel } from "../src/domains/workspace/git-operations-model";
import {
  applyShellResolveAmendDraft,
  applyShellToggleAmend,
} from "../src/domains/workspace/git-shell-actions";
import {
  containsRedactedRemoteMask,
  validateAuthorOverrideInput,
} from "../src/domains/workspace/git-v2-adapter";
import {
  readGitTargetMemory,
  writeGitTargetMemory,
} from "../src/domains/workspace/git-workbench-memory";
import { buildCommitPreviewModel } from "../src/domains/workspace/git-workbench-state";
import {
  buildGitRouteHref,
  parseGitRouteQuery,
} from "../src/hooks/shared/route";
import {
  runConfirmCommitFlow,
  runOpenCommitPreview,
} from "../src/hooks/workspace/git-commit-execution";
import { canConfirmPushPreview } from "../src/hooks/workspace/git-push-execution";
import { canConfirmBranchRename } from "../src/hooks/workspace/git-refs-manage-actions";
import {
  canConfirmStashApply,
  canConfirmStashCreate,
  canConfirmStashDrop,
} from "../src/hooks/workspace/git-stash-dialogs-actions";

afterEach(() => {
  vi.restoreAllMocks();
});

function makeV2ViewState(
  overrides: Partial<GitWorkbenchViewState> = {},
): GitWorkbenchViewState {
  const context = GitWorktreeContextSchema.parse({
    repositoryId: "repo-v2",
    repositoryName: "webmux",
    worktreeId: "wt-main",
    worktreePath: "/work/webmux",
    worktreeKind: "main",
    checkedOutBranch: "main",
    headOid: "1111111111111111111111111111111111111111",
    headState: "branch",
    ahead: 1,
    behind: 0,
    upstream: "origin/main",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    committerName: "Alice",
    committerEmail: "alice@example.com",
    signEnabled: false,
    executableHooks: [],
  });
  const headCommit = GitHistoryRowSchema.parse({
    oid: "1111111111111111111111111111111111111111",
    shortOid: "1111111",
    parents: ["2222222222222222222222222222222222222222"],
    lane: 0,
    activeLanes: [0],
    edges: [],
    directParentsMissing: false,
    subject: "feat(git): original head message",
    authorName: "Alice",
    authorEmail: "alice@example.com",
    authorAt: "2026-04-12T08:00:00Z",
    committerName: "Alice",
    committerEmail: "alice@example.com",
    committerAt: "2026-04-12T08:00:00Z",
    refs: [{ name: "main", kind: "head", worktreeId: "wt-main" }],
    changedPaths: [],
    branches: ["main"],
  });

  return {
    mode: "commit",
    context,
    historyState: "ready",
    historyReason: "",
    scope: "all",
    branchFilter: null,
    searchQuery: "",
    pathFilter: "",
    commits: [headCommit],
    commitDetails: {},
    selectedCommitOid: headCommit.oid,
    selectedParentByCommit: {},
    selectedCommitFilePath: null,
    workingChanges: [],
    activeChangeId: null,
    reviewedPaths: [],
    commitMessage: "",
    branches: [],
    commitPreview: null,
    lastCommitOutcome: null,
    pushPreview: null,
    statusBanner: "",
    ...overrides,
  };
}

describe("Git Workbench V2 (#550): Stash, Commit Amend/Author/Signoff, Refs & Remotes", () => {
  test("parseGitRouteQuery and buildGitRouteHref round-trip ?view=stash", () => {
    const parsed = parseGitRouteQuery("?view=stash&q=auth");
    expect(parsed.view).toBe("stash");
    expect(parsed.q).toBe("auth");

    const href = buildGitRouteHref({
      repositoryId: "repo-1",
      worktreeId: "wt-1",
      view: "stash",
    });
    expect(href).toBe("/git/repo-1/worktrees/wt-1?view=stash");
  });

  test("validateAuthorOverrideInput enforces Name <email> and rejects option-like dashes", () => {
    expect(validateAuthorOverrideInput("")).toEqual({ valid: true, error: "" });
    expect(
      validateAuthorOverrideInput("Co Author <co@example.invalid>"),
    ).toEqual({
      valid: true,
      error: "",
    });
    expect(validateAuthorOverrideInput("missing-email").valid).toBe(false);
    expect(validateAuthorOverrideInput("-bad <co@example.invalid>").valid).toBe(
      false,
    );
    expect(
      validateAuthorOverrideInput("Co Author <-bad@example.invalid>").valid,
    ).toBe(false);
  });

  test("containsRedactedRemoteMask detects *** placeholder in remote URLs", () => {
    expect(
      containsRedactedRemoteMask("https://user:***@github.com/org/repo.git"),
    ).toBe(true);
    expect(containsRedactedRemoteMask("https://github.com/org/repo.git")).toBe(
      false,
    );
  });

  test("Amend draft protection never silently overwrites non-empty draft and supports replace/merge/keep", () => {
    // Empty draft loads HEAD subject immediately.
    const fromEmpty = applyShellToggleAmend(makeV2ViewState(), true);
    expect(fromEmpty.commitAmend).toBe(true);
    expect(fromEmpty.commitMessage).toBe("feat(git): original head message");
    expect(fromEmpty.amendDraftConflict).toBeNull();

    // Non-empty differing draft opens conflict prompt without overwriting!
    const withDraft = makeV2ViewState({ commitMessage: "wip: my new draft" });
    const prompted = applyShellToggleAmend(withDraft, true);
    expect(prompted.commitAmend).toBe(true);
    expect(prompted.commitMessage).toBe("wip: my new draft");
    expect(prompted.amendDraftConflict).toEqual({
      headMessage: "feat(git): original head message",
      currentDraft: "wip: my new draft",
    });

    const merged = applyShellResolveAmendDraft(prompted, "merge");
    expect(merged.commitMessage).toBe(
      "wip: my new draft\n\nfeat(git): original head message",
    );
    expect(merged.amendDraftConflict).toBeNull();
  });

  test("buildCommitPreviewModel allows empty stagedFiles for Amend, blocks unborn HEAD, and disables followUpPush when pushed", () => {
    // Amend on normal branch with 0 staged files succeeds (message/author/signoff rewrite).
    const amendClean = buildCommitPreviewModel(
      makeV2ViewState({
        commitMessage: "feat(git): amend message + signoff",
        commitAmend: true,
        commitSignoff: true,
        commitAuthorOverride: "Bob <bob@example.invalid>",
      }),
      true,
    );
    expect(amendClean.amend).toBe(true);
    expect(amendClean.blockers).toEqual([]);
    expect(amendClean.followUpPush).toBe(true);
    expect(amendClean.authorOverridden).toBe(true);
    expect(amendClean.authorName).toBe("Bob");
    expect(amendClean.authorEmail).toBe("bob@example.invalid");
    expect(amendClean.signoff).toBe(true);
    expect(amendClean.signoffTrailer).toBe(
      "Signed-off-by: Alice <alice@example.com>",
    );

    // Amend on pushed HEAD forces followUpPush=false and emits pushed warning.
    const amendPushed = buildCommitPreviewModel(
      makeV2ViewState({
        commitMessage: "feat(git): amend pushed",
        commitAmend: true,
        commitAmendPushed: true,
      }),
      true,
    );
    expect(amendPushed.amendPushed).toBe(true);
    expect(amendPushed.followUpPush).toBe(false);
    expect(amendPushed.warnings.map((w) => w.code)).toContain("amend_pushed");

    // Amend on unborn HEAD is blocked.
    const unbornState = makeV2ViewState({
      commitMessage: "initial",
      commitAmend: true,
    });
    unbornState.context = {
      ...unbornState.context,
      headState: "unborn",
      headOid: null,
    };
    const amendUnborn = buildCommitPreviewModel(unbornState, false);
    expect(amendUnborn.blockers.map((b) => b.code)).toContain("unborn_head");
  });

  test("resolveGitBranchActions exposes rename/upstream/create-tag on local branches and create/delete-tag on tags", () => {
    const context = makeV2ViewState().context;
    const localBranch = GitBranchItemSchema.parse({
      name: "feat/stash",
      kind: "local",
      head: "1111111111111111111111111111111111111111",
      shortHead: "1111111",
      subject: "feat: stash",
      committedAt: "2026-04-12T08:00:00Z",
    });
    const localActions = resolveGitBranchActions({
      item: localBranch,
      context,
    }).map((a) => a.kind);
    expect(localActions).toContain("rename-branch");
    expect(localActions).toContain("set-upstream");
    expect(localActions).toContain("create-tag");

    const tagItem = GitBranchItemSchema.parse({
      name: "v1.0.0",
      kind: "tag",
      head: "1111111111111111111111111111111111111111",
      shortHead: "1111111",
      subject: "release v1.0.0",
      committedAt: "2026-04-12T08:00:00Z",
      tagKind: "annotated",
    });
    const tagActions = resolveGitBranchActions({
      item: tagItem,
      context,
    }).map((a) => a.kind);
    expect(tagActions).toContain("create-tag");
    expect(tagActions).toContain("delete-tag");
    expect(tagActions).not.toContain("checkout");
    expect(tagActions).not.toContain("push");
  });

  test("operation and sequencer labels cover stash, branch rename, tags, remotes, and unstash conflict", () => {
    expect(gitOperationKindLabel("stash_push")).toBe("新建贮藏");
    expect(gitOperationKindLabel("stash_pop")).toBe("弹出贮藏");
    expect(gitOperationKindLabel("branch_rename")).toBe("重命名分支");
    expect(gitOperationKindLabel("tag_create")).toBe("新建标签");
    expect(gitOperationKindLabel("remote_add")).toBe("添加远程仓库");

    const unstashOp = GitInProgressOperationSchema.parse({
      active: true,
      kind: "unstash",
      worktreeId: "wt-main",
      worktreePath: "/work/webmux",
      detached: false,
      sourceLabel: "stash@{0}",
      conflictedCount: 1,
      conflictedFiles: [],
      canContinue: false,
      canAbort: false,
      continueBlockers: [],
    });
    expect(formatInProgressHeadline(unstashOp)).toContain(
      "原贮藏条目已安全保留",
    );
  });

  test("GitStashApplyResultSchema accepts completed, conflicted, and needs_review outcomes", () => {
    const baseResult = {
      operationId: "op-stash-pop-1",
      kind: "stash_pop",
      stashRef: "stash@{0}",
      stashOid: "1111111111111111111111111111111111111111",
      stashDropped: false,
      stashPreserved: true,
      summary: "已恢复贮藏内容；丢弃贮藏引用失败，条目已保留供人工核对",
      inProgress: {
        active: false,
        worktreeId: "wt-main",
        worktreePath: "/work/webmux",
        detached: false,
        conflictedCount: 0,
        conflictedFiles: [],
        canContinue: false,
        canAbort: false,
        continueBlockers: [],
      },
      worktree: {
        id: "wt-main",
        repositoryId: "repo-v2",
        path: "/work/webmux",
        kind: "main",
        branch: "main",
        head: "1111111111111111111111111111111111111111",
        detached: false,
        locked: false,
        lockReason: "",
        prunable: false,
        available: true,
        dirty: true,
        hasUntracked: false,
        hasIgnored: false,
        error: "",
        upstream: null,
        ahead: null,
        behind: null,
        upstreamGone: false,
        lastCommit: null,
        changes: { staged: 0, unstaged: 1, untracked: 0, conflicted: 0 },
        operation: null,
      },
    };

    for (const outcome of [
      "completed",
      "conflicted",
      "needs_review",
    ] as const) {
      const parsed = GitStashApplyResultSchema.parse({
        ...baseResult,
        outcome,
      });
      expect(parsed.outcome).toBe(outcome);
    }
    expect(
      GitStashApplyResultSchema.safeParse({
        ...baseResult,
        outcome: "unknown_outcome",
      }).success,
    ).toBe(false);
  });

  test("confirmation guards reject stale parameters, pending debounce, blockers, and unacknowledged sessions", () => {
    const createPreview = {
      confirmationId: "confirm-create-1",
      expiresAt: null,
      repositoryId: "repo-v2",
      worktreeId: "wt-main",
      worktreePath: "/work/webmux",
      detached: false,
      message: "wip",
      includeUntracked: true,
      keepIndex: false,
      stagedCount: 1,
      unstagedCount: 0,
      untrackedCount: 1,
      affectedPaths: ["a.ts"],
      sessions: [],
      blockers: [],
      warnings: [],
    };
    expect(
      canConfirmStashCreate({
        createOpen: true,
        repoId: "repo-v2",
        wtId: "wt-main",
        preview: createPreview,
        loadingPreview: false,
        submitting: false,
        hasPendingDebounce: false,
        includeUntracked: true,
        keepIndex: false,
        trimmedMessage: "wip",
        acknowledgeSessions: false,
      }),
    ).toBe(true);
    expect(
      canConfirmStashCreate({
        createOpen: true,
        repoId: "repo-v2",
        wtId: "wt-main",
        preview: createPreview,
        loadingPreview: false,
        submitting: false,
        hasPendingDebounce: true,
        includeUntracked: true,
        keepIndex: false,
        trimmedMessage: "wip",
        acknowledgeSessions: false,
      }),
    ).toBe(false);
    expect(
      canConfirmStashCreate({
        createOpen: true,
        repoId: "repo-v2",
        wtId: "wt-main",
        preview: createPreview,
        loadingPreview: false,
        submitting: false,
        hasPendingDebounce: false,
        includeUntracked: false,
        keepIndex: false,
        trimmedMessage: "wip",
        acknowledgeSessions: false,
      }),
    ).toBe(false);

    const stashEntry = {
      stashRef: "stash@{0}",
      index: 0,
      oid: "abc",
      shortOid: "abc",
      parents: [],
      hasUntracked: false,
      summary: "wip",
      createdAt: "2026-11-19T00:00:00Z",
    };
    expect(
      canConfirmStashApply({
        applyOpen: true,
        repoId: "repo-v2",
        wtId: "wt-main",
        preview: {
          confirmationId: "confirm-apply-1",
          expiresAt: null,
          repositoryId: "repo-v2",
          worktreeId: "wt-main",
          worktreePath: "/work/webmux",
          mode: "pop",
          entry: stashEntry,
          reinstateIndex: false,
          detached: false,
          stagedCount: 0,
          unstagedCount: 1,
          untrackedCount: 0,
          overlappingPaths: [],
          sessions: [],
          blockers: [],
          warnings: [],
        },
        target: { ...stashEntry, oid: "different-oid" },
        mode: "pop",
        reinstateIndex: false,
        loadingPreview: false,
        submitting: false,
        acknowledgeSessions: false,
      }),
    ).toBe(false);

    expect(
      canConfirmStashDrop({
        dropOpen: true,
        repoId: "repo-v2",
        preview: {
          confirmationId: "confirm-drop-1",
          expiresAt: null,
          repositoryId: "repo-v2",
          entry: stashEntry,
          fileCount: 1,
          recoveryCommand: "git stash apply abc",
          blockers: [{ code: "locked", message: "blocked" }],
          warnings: [],
        },
        target: stashEntry,
        loadingPreview: false,
        submitting: false,
      }),
    ).toBe(false);

    expect(
      canConfirmBranchRename({
        renameOpen: true,
        loadingPreview: false,
        submitting: false,
        hasPendingDebounce: false,
        oldName: "feat/old",
        newName: "feat/new",
        acknowledgeSessions: false,
        preview: {
          confirmationId: "confirm-rename-1",
          expiresAt: null,
          repositoryId: "repo-v2",
          oldName: "feat/old",
          newName: "feat/new",
          headOid: "1111111111111111111111111111111111111111",
          subject: "feat",
          affectedWorktrees: [],
          sessions: [],
          blockers: [],
          warnings: [],
        },
      }),
    ).toBe(true);
    expect(
      canConfirmBranchRename({
        renameOpen: true,
        loadingPreview: false,
        submitting: false,
        hasPendingDebounce: true,
        oldName: "feat/old",
        newName: "feat/new",
        acknowledgeSessions: false,
        preview: {
          confirmationId: "confirm-rename-1",
          expiresAt: null,
          repositoryId: "repo-v2",
          oldName: "feat/old",
          newName: "feat/new",
          headOid: "1111111111111111111111111111111111111111",
          subject: "feat",
          affectedWorktrees: [],
          sessions: [],
          blockers: [],
          warnings: [],
        },
      }),
    ).toBe(false);

    expect(
      canConfirmPushPreview({
        open: true,
        repoId: "repo-v2",
        preview: {
          confirmationId: "confirm-push-1",
          expiresAt: null,
          branch: "feat/a",
          commit: "abc",
          remote: "origin",
          pushUrl: "https://example.invalid/r.git",
          target: "feat/a",
          publish: true,
          commits: { total: 0, items: [] },
          blockers: [],
          warnings: [],
        },
        running: false,
        loading: false,
        hasPendingDebounce: true,
        publishing: true,
        remote: "origin",
        target: "feat/a",
      }),
    ).toBe(false);
  });

  test("runOpenCommitPreview ignores AbortError and runConfirmCommitFlow marks preview stale on non-stale failure and isolates switched targets", async () => {
    const abortCtrl = new AbortController();
    vi.spyOn(globalThis, "fetch").mockImplementationOnce(async () => {
      abortCtrl.abort();
      throw new DOMException("Aborted", "AbortError");
    });

    let view = makeV2ViewState();
    await runOpenCommitPreview({
      repoId: "repo-v2",
      wtId: "wt-main",
      view,
      followUpPush: false,
      signal: abortCtrl.signal,
      isCurrent: () => false,
      setConfirmationId: () => {},
      getLatestView: () => view,
      patchView: (patch) => {
        view = { ...view, ...patch };
      },
    });
    expect(view.statusBanner).toBe("");

    // Non-stale commit failure marks commitPreview stale because confirmationId was consumed.
    const previewModel = buildCommitPreviewModel(
      makeV2ViewState({
        commitMessage: "feat: commit test",
        commitAmend: true,
      }),
      false,
    );
    let failView = makeV2ViewState({
      commitMessage: "feat: commit test",
      commitPreview: previewModel,
    });
    vi.spyOn(globalThis, "fetch").mockImplementationOnce(async () =>
      Response.json({ error: "pre-commit hook failed" }, { status: 500 }),
    );
    await runConfirmCommitFlow({
      repoId: "repo-v2",
      wtId: "wt-main",
      confirmationId: "consumed-id-1",
      current: failView,
      signal: new AbortController().signal,
      clearWebmuxStaged: () => {},
      refreshStaging: async () => {},
      patchView: (patch) => {
        failView = { ...failView, ...patch };
      },
    });
    expect(failView.commitPreview?.stale).toBe(true);
    expect(failView.commitPreview?.dialogError).toContain(
      "pre-commit hook failed",
    );

    // Target switched mid-commit: saves cleared draft memory only for original (repoId, wtId) without mutating new target view.
    writeGitTargetMemory("repo-a", "wt-a", { commitMessage: "feat: repo-a" });
    let switchedView = makeV2ViewState({
      commitMessage: "wip: repo-b untouched draft",
      commitPreview: null,
    });
    const sourceSnapshot = makeV2ViewState({
      commitMessage: "feat: repo-a",
      commitPreview: previewModel,
    });
    vi.spyOn(globalThis, "fetch").mockImplementationOnce(async () =>
      Response.json({
        operationId: "op-commit-a",
        outcome: "matched",
        commitOid: "3333333333333333333333333333333333333333",
        treeOid: "4444444444444444444444444444444444444444",
        expectedTreeOid: "4444444444444444444444444444444444444444",
        parents: ["1111111111111111111111111111111111111111"],
        subject: "feat: repo-a",
        reviewReasons: [],
      }),
    );
    const patchSpy = vi.fn((patch: Partial<GitWorkbenchViewState>) => {
      switchedView = { ...switchedView, ...patch };
    });
    await runConfirmCommitFlow({
      repoId: "repo-a",
      wtId: "wt-a",
      confirmationId: "confirm-repo-a",
      current: sourceSnapshot,
      signal: new AbortController().signal,
      isCurrentTarget: () => false,
      clearWebmuxStaged: () => {},
      refreshStaging: async () => {},
      patchView: patchSpy,
    });
    expect(patchSpy).not.toHaveBeenCalled();
    expect(switchedView.commitMessage).toBe("wip: repo-b untouched draft");
    expect(readGitTargetMemory("repo-a", "wt-a").commitMessage).toBe("");
  });
});
