import { afterEach, expect, test, vi } from "vite-plus/test";
import type {
  GitStashEntry,
  PushPreview,
  SwitchPreview,
} from "../../src/contracts/schemas";
import { createGitDiscardController } from "../../src/hooks/workspace/git-discard-controller.svelte";
import { createGitDivergenceController } from "../../src/hooks/workspace/git-divergence-controller.svelte";
import { createGitOperationsController } from "../../src/hooks/workspace/git-operations-controller.svelte";
import { createGitPushController } from "../../src/hooks/workspace/git-push-controller.svelte";
import { createGitRefsManageController } from "../../src/hooks/workspace/git-refs-manage-controller.svelte";
import { createSmartCheckoutController } from "../../src/hooks/workspace/git-smart-checkout.svelte";
import { createGitStashDialogsController } from "../../src/hooks/workspace/git-stash-dialogs.svelte";
import { createGitStagingController } from "../../src/hooks/workspace/git-workbench-staging.svelte";
import { emptyGitViewState } from "../../src/hooks/workspace/git-workbench-actions";
import { session, worktree } from "./shell-fixture";

afterEach(() => vi.restoreAllMocks());

const envelope = { confirmationId: "first", expiresAt: null };

test("changing a publish destination invalidates its preview even before the debounced request", async () => {
  const preview: PushPreview = {
    ...envelope,
    branch: "topic",
    commit: "abc",
    remote: "origin",
    pushUrl: "file:///demo/origin",
    target: "topic",
    publish: true,
    commits: { total: 0, items: [] },
    blockers: [],
    warnings: [],
  };
  const nextPreview = Promise.withResolvers<void>();
  const fetch = vi
    .spyOn(globalThis, "fetch")
    .mockImplementation(async (input, init) => {
      const url = String(input);
      if (url.endsWith("/remotes")) return Response.json({ remotes: [] });
      if (url.endsWith("/push-preview")) {
        const body = JSON.parse(String(init?.body)) as { remote?: string };
        if (body.remote !== "mirror") return Response.json(preview);
        await nextPreview.promise;
        return Response.json({
          ...preview,
          remote: "mirror",
          confirmationId: "second",
        });
      }
      throw new Error(`Unexpected fetch: ${url}`);
    });
  const push = createGitPushController({
    getRepoId: () => "sample",
    getRepoName: () => "sample",
    getWorktreeId: () => "main",
    onStatusBanner: () => {},
  });
  try {
    push.openDialog("topic");
    await expect.poll(() => push.preview?.confirmationId).toBe("first");

    push.setPublishRemote("mirror");
    expect(push.preview).toBeNull();
    await push.confirmPush();

    push.setPublishTarget("another-topic");
    push.setPublishTarget("topic");
    await push.confirmPush();
    expect(push.preview).toBeNull();
    expect(
      fetch.mock.calls.every(([url]) => !String(url).endsWith("/push")),
    ).toBe(true);

    nextPreview.resolve();
    await expect.poll(() => push.preview?.confirmationId).toBe("second");
  } finally {
    push.dispose();
  }
});

test("a failed safe-pop preview cannot execute using the earlier apply confirmation", async () => {
  const entry: GitStashEntry = {
    stashRef: "stash@{0}",
    index: 0,
    oid: "abc",
    shortOid: "abc",
    parents: [],
    hasUntracked: true,
    summary: "Demo changes",
    createdAt: "2026-11-19",
  };
  const popPreview = Promise.withResolvers<Response>();
  const fetch = vi
    .spyOn(globalThis, "fetch")
    .mockImplementation(async (input, init) => {
      const url = String(input);
      if (!url.endsWith("/stash-apply-preview"))
        throw new Error(`Unexpected fetch: ${url}`);
      const body = JSON.parse(String(init?.body)) as { mode: string };
      if (body.mode === "pop") return popPreview.promise;
      return Response.json({
        ...envelope,
        repositoryId: "demo",
        worktreeId: "main",
        worktreePath: "/demo",
        mode: "apply",
        entry,
        reinstateIndex: false,
        detached: false,
        stagedCount: 0,
        unstagedCount: 1,
        untrackedCount: 1,
        overlappingPaths: ["src/file.txt"],
        sessions: [],
        blockers: [],
        warnings: [],
      });
    });
  const stash = createGitStashDialogsController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    patchView: () => {},
    onRefreshList: async () => {},
    onAfterStashMutation: async () => {},
    onInProgressUpdated: () => {},
    onOpenConflictResolver: () => {},
  });
  try {
    stash.openApplyDialog(entry, "apply");
    await expect.poll(() => stash.applyPreview?.confirmationId).toBe("first");
    stash.setApplyMode("pop");
    expect(stash.applyPreview).toBeNull();
    await stash.confirmApply();
    popPreview.resolve(
      Response.json({ error: "Worktree changed" }, { status: 409 }),
    );
    await expect.poll(() => stash.dialogError).toContain("Worktree changed");
    await stash.confirmApply();
    expect(
      fetch.mock.calls.every(([url]) =>
        String(url).endsWith("/stash-apply-preview"),
      ),
    ).toBe(true);
  } finally {
    stash.dispose();
  }
});

test("stash pop needs_review closes dialog, updates banner and preserved selection without opening conflict resolver", async () => {
  const entry: GitStashEntry = {
    stashRef: "stash@{0}",
    index: 0,
    oid: "1".repeat(40),
    shortOid: "1111111",
    parents: [],
    hasUntracked: false,
    summary: "WIP on main",
    createdAt: "2026-11-19T00:00:00Z",
  };
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    if (url.endsWith("/stash-apply-preview")) {
      return Response.json({
        ...envelope,
        repositoryId: "demo",
        worktreeId: "main",
        worktreePath: "/demo",
        mode: "pop",
        entry,
        reinstateIndex: false,
        detached: false,
        stagedCount: 0,
        unstagedCount: 1,
        untrackedCount: 0,
        overlappingPaths: [],
        sessions: [],
        blockers: [],
        warnings: [],
      });
    }
    if (url.endsWith("/stash-pop")) {
      return Response.json({
        operationId: "op-pop-review",
        kind: "stash_pop",
        outcome: "needs_review",
        stashRef: entry.stashRef,
        stashOid: entry.oid,
        stashDropped: false,
        stashPreserved: true,
        summary: "贮藏内容已恢复；丢弃贮藏引用失败，条目已保留供人工核对",
        inProgress: {
          active: false,
          worktreeId: "main",
          worktreePath: "/demo",
          detached: false,
          conflictedCount: 0,
          conflictedFiles: [],
          canContinue: false,
          canAbort: false,
          continueBlockers: [],
        },
        worktree: worktree({ id: "main", repositoryId: "demo", dirty: true }),
      });
    }
    if (url.includes("/git/operations")) {
      return Response.json({ operations: [] });
    }
    throw new Error(`Unexpected fetch: ${url}`);
  });

  let banner = "";
  const refreshedPreferredOids: Array<string | null | undefined> = [];
  const openConflict = vi.fn();
  const operations = createGitOperationsController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    onOutcomeRestored: () => {},
    onStatusBanner: () => {},
  });
  const stash = createGitStashDialogsController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    patchView: (patch) => {
      if (patch.statusBanner !== undefined) banner = patch.statusBanner;
    },
    operations,
    onRefreshList: async (preferred) => {
      refreshedPreferredOids.push(preferred);
    },
    onAfterStashMutation: async () => {},
    onInProgressUpdated: () => {},
    onOpenConflictResolver: openConflict,
  });

  try {
    stash.openApplyDialog(entry, "pop");
    await expect.poll(() => stash.applyPreview?.confirmationId).toBe("first");
    await stash.confirmApply();

    expect(stash.applyOpen).toBe(false);
    expect(stash.applyPreview).toBeNull();
    expect(banner).toContain("条目已保留供人工核对");
    expect(refreshedPreferredOids).toEqual([entry.oid]);
    expect(openConflict).not.toHaveBeenCalled();
  } finally {
    stash.dispose();
    operations.dispose();
  }
});

test("stash create and branch rename debounce text input, drop stale confirmationId, and clear consumed previews on error", async () => {
  const activeSession = session({ id: "sessaaaa", title: "claude" });
  let createPreviewCalls = 0;
  let renamePreviewCalls = 0;

  const fetch = vi
    .spyOn(globalThis, "fetch")
    .mockImplementation(async (input, init) => {
      const url = String(input);
      if (url.endsWith("/stash-preview")) {
        createPreviewCalls += 1;
        const body = JSON.parse(String(init?.body ?? "{}")) as {
          message?: string;
          includeUntracked?: boolean;
          keepIndex?: boolean;
        };
        return Response.json({
          confirmationId: `stash-c-${createPreviewCalls}`,
          expiresAt: null,
          repositoryId: "demo",
          worktreeId: "main",
          worktreePath: "/demo",
          detached: false,
          message: body.message,
          includeUntracked: Boolean(body.includeUntracked),
          keepIndex: Boolean(body.keepIndex),
          stagedCount: 1,
          unstagedCount: 0,
          untrackedCount: 0,
          affectedPaths: ["a.ts"],
          sessions: [activeSession],
          blockers: [],
          warnings: [],
        });
      }
      if (url.endsWith("/stash")) {
        return Response.json({ error: "lock busy" }, { status: 409 });
      }
      if (url.endsWith("/branches/rename-preview")) {
        renamePreviewCalls += 1;
        const body = JSON.parse(String(init?.body ?? "{}")) as {
          oldName: string;
          newName: string;
        };
        return Response.json({
          confirmationId: `rename-c-${renamePreviewCalls}`,
          expiresAt: null,
          repositoryId: "demo",
          oldName: body.oldName,
          newName: body.newName,
          headOid: "1".repeat(40),
          subject: "feat",
          affectedWorktrees: [],
          sessions: [activeSession],
          blockers: [],
          warnings: [],
        });
      }
      if (url.endsWith("/branches/rename")) {
        return Response.json({ error: "ref changed" }, { status: 409 });
      }
      throw new Error(`Unexpected fetch: ${url}`);
    });

  const stash = createGitStashDialogsController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    patchView: () => {},
    onRefreshList: async () => {},
    onAfterStashMutation: async () => {},
    onInProgressUpdated: () => {},
    onOpenConflictResolver: () => {},
  });
  let view = emptyGitViewState("demo");
  const refs = createGitRefsManageController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    getRepository: () => undefined,
    getSessions: () => [activeSession],
    getView: () => view,
    patchView: (patch) => {
      view = { ...view, ...patch };
    },
  });

  try {
    stash.openCreateDialog(false);
    await expect
      .poll(() => stash.createPreview?.confirmationId)
      .toBe("stash-c-1");
    stash.setAcknowledgeSessions(true);

    // Editing message clears preview & session ack immediately and blocks confirm during debounce.
    stash.setCreateMessage("wip: updated");
    expect(stash.createPreview).toBeNull();
    expect(stash.acknowledgeSessions).toBe(false);
    await stash.confirmCreate();
    expect(
      fetch.mock.calls.some(([url]) => String(url).endsWith("/git/stash")),
    ).toBe(false);

    await expect
      .poll(() => stash.createPreview?.confirmationId)
      .toBe("stash-c-2");
    // Unacknowledged active session still blocks confirmCreate.
    await stash.confirmCreate();
    expect(
      fetch.mock.calls.some(([url]) => String(url).endsWith("/git/stash")),
    ).toBe(false);

    stash.setAcknowledgeSessions(true);
    await stash.confirmCreate();
    expect(stash.createPreview).toBeNull();
    expect(stash.dialogError).toContain("lock busy");

    // Branch rename: opening with same name sends zero preview requests.
    refs.openRenameDialog("feat/old");
    expect(renamePreviewCalls).toBe(0);
    expect(refs.loadingPreview).toBe(false);

    refs.setRenameNewName("feat/new");
    expect(refs.renamePreview).toBeNull();
    expect(refs.loadingPreview).toBe(true);
    await expect
      .poll(() => refs.renamePreview?.confirmationId)
      .toBe("rename-c-1");
    refs.setRenameAckSessions(true);

    // Reverting to old name cancels preview without firing a request.
    refs.setRenameNewName("feat/old");
    expect(refs.renamePreview).toBeNull();
    expect(refs.renameAckSessions).toBe(false);
    expect(refs.loadingPreview).toBe(false);
    expect(renamePreviewCalls).toBe(1);

    refs.setRenameNewName("feat/final");
    await expect
      .poll(() => refs.renamePreview?.confirmationId)
      .toBe("rename-c-2");
    refs.setRenameAckSessions(true);
    await refs.confirmRename();
    expect(refs.renamePreview).toBeNull();
    expect(refs.dialogError).toContain("ref changed");
  } finally {
    stash.dispose();
    refs.dispose();
  }
});

test("discard, smart checkout, and divergence controllers clear stale confirmations and enforce execution guards", async () => {
  const activeSession = session({ id: "sessbbbb", title: "shell" });
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    if (url.endsWith("/discard-preview")) {
      return Response.json({
        confirmationId: "discard-c-1",
        expiresAt: null,
        repositoryId: "demo",
        worktreeId: "main",
        worktreePath: "/demo",
        scope: "hunks",
        summary: "丢弃 1 个变更块",
        items: [{ path: "a.ts", status: "M", action: "overwrite_tracked" }],
        hunkCount: 1,
        lineCount: 0,
        trackedOverwritePaths: ["a.ts"],
        untrackedDeletePaths: [],
        sessions: [activeSession],
        blockers: [],
        warnings: [],
      });
    }
    if (url.endsWith("/discard")) {
      return Response.json({ error: "io failure" }, { status: 500 });
    }
    if (url.endsWith("/merge-preview")) {
      return Response.json({
        confirmationId: "merge-c-1",
        expiresAt: null,
        repositoryId: "demo",
        worktreeId: "main",
        worktreePath: "/demo",
        kind: "merge",
        targetRef: "origin/main",
        targetKind: "remote",
        targetOid: "b".repeat(40),
        targetSubject: "feat: upstream",
        directionSummary: "Merge origin/main into main",
        recoveryHint: "git merge --abort",
        fastForward: false,
        alreadyUpToDate: false,
        commits: { total: 1, items: [] },
        sessions: [activeSession],
        blockers: [],
        warnings: [],
      });
    }
    if (url.endsWith("/merge")) {
      return Response.json({ error: "merge rejected" }, { status: 409 });
    }
    throw new Error(`Unexpected fetch: ${url}`);
  });

  let hasUnstaged = true;
  const discard = createGitDiscardController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    patchView: () => {},
    getEntryIdentity: () =>
      hasUnstaged
        ? {
            area: "unstaged",
            path: "a.ts",
            baseOid: "1".repeat(40),
            targetSha256: "2".repeat(64),
          }
        : undefined,
    onAfterDiscard: async () => {},
  });

  let view = emptyGitViewState("demo");
  const operations = createGitOperationsController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    onOutcomeRestored: () => {},
    onStatusBanner: () => {},
  });
  const divergence = createGitDivergenceController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    getView: () => view,
    patchView: (patch) => {
      view = { ...view, ...patch };
    },
    operations,
    onAfterMutation: async () => {},
  });

  const resolvedTarget: SwitchPreview["target"] = {
    kind: "local",
    branch: "feat/target",
    commit: "a".repeat(40),
    subject: "feat: target",
    createsBranch: false,
    upstream: null,
  };
  const plainPreview: SwitchPreview = {
    confirmationId: "",
    expiresAt: null,
    worktree: worktree({ id: "main", hasUntracked: true }),
    target: resolvedTarget,
    sessions: [],
    blockers: [{ code: "changes", message: "dirty" }],
    warnings: [],
  };
  const smartPreviewWait = Promise.withResolvers<void>();
  const smart = createSmartCheckoutController({
    getWorktree: () => worktree({ id: "main", hasUntracked: true }),
    getTarget: () => ({ kind: "local", branch: "feat/target" }),
    getPlainPreview: () => plainPreview,
    getBusy: () => false,
    setBusy: () => {},
    getSource: () => ({
      smartPreview: async (tree, req) => {
        if (!req.includeUntracked) await smartPreviewWait.promise;
        return {
          confirmationId: req.includeUntracked ? "smart-1" : "smart-2",
          expiresAt: null,
          repositoryId: "demo",
          worktreeId: "main",
          worktree: tree,
          target: resolvedTarget,
          includeUntracked: Boolean(req.includeUntracked),
          stagedCount: 0,
          unstagedCount: 1,
          untrackedCount: 1,
          dirtyPaths: ["a.ts"],
          hasActiveAgent: false,
          sessions: [],
          blockers: [],
          warnings: [],
        };
      },
      smartCheckout: async () => {
        throw new Error("should not execute while previewing");
      },
    }),
    onDone: () => {},
    onChange: () => {},
    onReloadPlainPreview: () => {},
  });

  try {
    discard.openDiscardHunk("unstaged:a.ts", "hunk-0");
    await expect
      .poll(() => discard.preview?.confirmationId)
      .toBe("discard-c-1");
    // Unacknowledged active session blocks confirmDiscard.
    await discard.confirmDiscard();
    expect(discard.preview?.confirmationId).toBe("discard-c-1");

    discard.setAcknowledgeSessions(true);
    await discard.confirmDiscard();
    // Non-stale failure clears consumed preview & session acknowledgement.
    expect(discard.preview).toBeNull();
    expect(discard.acknowledgeSessions).toBe(false);

    // refreshPreviewAfterStale when unstaged entry disappeared keeps preview null.
    hasUnstaged = false;
    await discard.refreshPreviewAfterStale();
    expect(discard.preview).toBeNull();

    // Smart checkout: toggling includeUntracked immediately clears old preview and disables canExecute.
    smart.syncPlainPreview(plainPreview);
    await expect.poll(() => smart.preview?.confirmationId).toBe("smart-1");
    expect(smart.canExecute).toBe(true);
    smart.setIncludeUntracked(false);
    expect(smart.preview).toBeNull();
    expect(smart.canExecute).toBe(false);
    smartPreviewWait.resolve();
    await expect.poll(() => smart.preview?.confirmationId).toBe("smart-2");
    expect(smart.canExecute).toBe(true);

    // Divergence: unacknowledged session blocks confirmDivergence, and execution failure clears preview.
    await divergence.openMergePreview("remote", "origin/main");
    expect(divergence.preview?.confirmationId).toBe("merge-c-1");
    await divergence.confirmDivergence();
    expect(divergence.preview?.confirmationId).toBe("merge-c-1");

    divergence.setAcknowledgeSessions(true);
    await divergence.confirmDivergence();
    expect(divergence.preview).toBeNull();
    expect(divergence.acknowledgeSessions).toBe(false);
    expect(divergence.dialogError).toContain("merge rejected");
  } finally {
    discard.dispose();
    divergence.dispose();
    operations.dispose();
    smart.dispose();
  }
});

test("GitStagingController cancels closed commit preview refreshes and isolates in-flight confirmCommit from stage and preview actions", async () => {
  let previewSeq = 0;
  let blockStaging = false;
  let stagingGate = Promise.withResolvers<void>();
  const commitGate = Promise.withResolvers<Response>();
  let capturedCommitSignal: AbortSignal | null | undefined;
  const stageCalls: string[] = [];
  const previewCalls: number[] = [];

  const buildPreviewJson = (id: string) => ({
    confirmationId: id,
    expiresAt: null,
    repositoryId: "demo",
    worktreeId: "main",
    worktreePath: "/demo",
    headState: "branch",
    branch: "main",
    headOid: "1".repeat(40),
    headParents: [],
    headPush: { state: "unpushed", branches: [], more: 0 },
    amend: false,
    amendPushed: false,
    signoff: false,
    expectedTreeOid: "tree-1",
    indexDigest: "digest-1",
    stagedFiles: [
      {
        oldPath: "a.ts",
        newPath: "a.ts",
        status: "M",
        oldMode: "100644",
        newMode: "100644",
        oldOid: "1111",
        newOid: "2222",
        modeChanged: false,
        binary: false,
      },
    ],
    unstagedCount: 1,
    untrackedCount: 0,
    conflictedCount: 0,
    identity: {
      authorName: "Alice",
      authorEmail: "alice@example.com",
      committerName: "Alice",
      committerEmail: "alice@example.com",
      signEnabled: false,
    },
    executableHooks: [],
    blockers: [],
    warnings: [],
  });

  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input);
    if (url.endsWith("/git/staging")) {
      if (blockStaging) await stagingGate.promise;
      return Response.json({
        unborn: false,
        headOid: "1".repeat(40),
        conflicted: [],
        staged: [
          {
            area: "staged",
            path: "a.ts",
            status: "M",
            hunkStageAvailable: false,
            identity: {
              area: "staged",
              path: "a.ts",
              baseOid: "1111",
              targetOid: "2222",
            },
          },
        ],
        unstaged: [
          {
            area: "unstaged",
            path: "b.ts",
            status: "M",
            hunkStageAvailable: false,
            identity: {
              area: "unstaged",
              path: "b.ts",
              baseOid: "3333",
              targetSha256: "4".repeat(64),
            },
          },
        ],
      });
    }
    if (url.includes("/git/staging/diff")) {
      return Response.json({
        area: "staged",
        path: "a.ts",
        status: "M",
        binary: false,
        hunkStageAvailable: false,
        identity: {
          area: "staged",
          path: "a.ts",
          baseOid: "1111",
          targetOid: "2222",
        },
        hunks: [],
      });
    }
    if (url.endsWith("/git/commit-preview")) {
      const n = ++previewSeq;
      previewCalls.push(n);
      return Response.json(buildPreviewJson(`commit-c-${n}`));
    }
    if (url.endsWith("/git/stage") || url.endsWith("/git/unstage")) {
      stageCalls.push(url);
      return new Response(null, { status: 204 });
    }
    if (url.endsWith("/git/commit")) {
      capturedCommitSignal = init?.signal;
      return commitGate.promise;
    }
    if (url.includes("/git/operations")) {
      return Response.json({ operations: [] });
    }
    throw new Error(`Unexpected fetch: ${url}`);
  });

  let view = {
    ...emptyGitViewState("demo"),
    commitMessage: "feat: race-free commit",
  };
  const staging = createGitStagingController({
    getRepoId: () => "demo",
    getWorktreeId: () => "main",
    getView: () => view,
    patchView: (patch) => {
      view = { ...view, ...patch };
    },
  });

  try {
    await staging.refresh("demo", "main");
    await staging.openCommitPreview(false);
    expect(view.commitPreview).not.toBeNull();

    // 1. Mark preview stale, start refreshCommitPreview(), then close dialog while refresh is in-flight:
    // closing the dialog must keep commitPreview null and never reopen it after refresh resolves!
    view = {
      ...view,
      commitPreview: { ...view.commitPreview!, stale: true },
    };
    blockStaging = true;
    stagingGate = Promise.withResolvers<void>();
    const refreshTask = staging.refreshCommitPreview();
    staging.closeCommitPreview();
    expect(view.commitPreview).toBeNull();

    const previewsBeforeResolve = previewCalls.length;
    blockStaging = false;
    stagingGate.resolve();
    await refreshTask;
    expect(view.commitPreview).toBeNull();
    // Only syncPreflightMetadata from refresh may hit commit-preview, never openCommitPreview reopening commitPreview!
    expect(previewCalls.length).toBeLessThanOrEqual(previewsBeforeResolve + 1);

    // 2. Reopen preview and submit confirmCommit(): concurrent stageOrUnstage, stageOrUnstageGroup,
    // openCommitPreview, and refreshCommitPreview must be ignored without aborting confirmCommit!
    await staging.openCommitPreview(false);
    expect(view.commitPreview).not.toBeNull();
    const previewCountBeforeSubmit = previewCalls.length;

    const commitTask = staging.confirmCommit();
    expect(staging.commitSubmitting).toBe(true);
    expect(capturedCommitSignal?.aborted).toBe(false);

    await staging.stageOrUnstage("stage", "unstaged:b.ts");
    await staging.stageOrUnstageGroup("unstaged");
    await staging.openCommitPreview(true);
    await staging.refreshCommitPreview();

    expect(capturedCommitSignal?.aborted).toBe(false);
    expect(stageCalls).toEqual([]);
    expect(previewCalls.length).toBe(previewCountBeforeSubmit);

    commitGate.resolve(
      Response.json({
        operationId: "op-commit-1",
        outcome: "matched",
        commitOid: "a".repeat(40),
        treeOid: "tree-1",
        expectedTreeOid: "tree-1",
        parents: ["1".repeat(40)],
        subject: "feat: race-free commit",
        reviewReasons: [],
      }),
    );
    await commitTask;
    expect(staging.commitSubmitting).toBe(false);
    expect(view.commitPreview).toBeNull();
    expect(view.lastCommitOutcome?.status).toBe("matched");
  } finally {
    staging.dispose();
  }
});
