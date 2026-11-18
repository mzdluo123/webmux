import { afterEach, expect, test, vi } from "vite-plus/test";
import type { GitStashEntry, PushPreview } from "../../src/contracts/schemas";
import { createGitPushController } from "../../src/hooks/workspace/git-push-controller.svelte";
import { createGitStashDialogsController } from "../../src/hooks/workspace/git-stash-dialogs.svelte";

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
