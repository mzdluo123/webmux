import { afterEach, expect, test, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import {
  type GitCommitOutcome,
  type GitWorkbenchViewState,
} from "../../src/contracts/git-workbench-schemas";
import { runConfirmCommitFlow } from "../../src/hooks/workspace/git-commit-execution";
import { createGitOperationsController } from "../../src/hooks/workspace/git-operations-controller.svelte";
import { buildGitWorkbenchFixture } from "../../stories/git-workbench-fixtures";
import Harness from "./GitWorkbenchHarness.svelte";

afterEach(async () => {
  vi.restoreAllMocks();
  localStorage.clear();
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

test("renders 24-28px status bar across desktop and phone viewports and expands operation history popover within viewport", async () => {
  await page.viewport(1280, 900);
  const initialState = buildGitWorkbenchFixture("commit-needs-review");
  await render(Harness, { initialState });

  const statusBar = page.getByTestId("git-workbench-status-bar");
  await expect.element(statusBar).toBeVisible();
  const desktopHeight = statusBar.element().getBoundingClientRect().height;
  expect(desktopHeight).toBeGreaterThanOrEqual(24);
  expect(desktopHeight).toBeLessThanOrEqual(28);
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );

  // Check Phone viewport (390x844) does NOT inflate status bar beyond 28px or overflow horizontally
  await page.viewport(390, 844);
  const phoneHeight = statusBar.element().getBoundingClientRect().height;
  expect(phoneHeight).toBeGreaterThanOrEqual(24);
  expect(phoneHeight).toBeLessThanOrEqual(28);
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );

  // Open operation history popover on Phone viewport and inspect collision-padded bounds & redacted output
  const trigger = page.getByTestId("git-operations-popover-trigger");
  await trigger.click();

  const popover = page.getByTestId("git-operations-popover");
  await expect.element(popover).toBeVisible();
  const rect = popover.element().getBoundingClientRect();
  expect(rect.left).toBeGreaterThanOrEqual(0);
  expect(rect.right).toBeLessThanOrEqual(window.innerWidth);
  expect(rect.top).toBeGreaterThanOrEqual(0);
  expect(rect.bottom).toBeLessThanOrEqual(window.innerHeight);

  await expect.element(popover.getByText("op:8f31c0d9e4")).toBeVisible();
  await expect
    .element(popover.getByTestId("git-operation-output"))
    .toBeVisible();
});

test("blocks blind commit re-submission when last operation is uncertain until user refreshes and verifies", async () => {
  await page.viewport(1280, 900);
  const initialState = {
    ...buildGitWorkbenchFixture("operation-uncertain"),
    mode: "commit" as const,
  };
  await render(Harness, { initialState });

  const banner = page.getByTestId("git-commit-outcome-banner");
  await expect.element(banner).toBeVisible();
  expect(banner.element().textContent).toContain("[uncertain]");

  const previewBtn = page.getByRole("button", { name: "提交预览…" });
  const pushBtn = page.getByRole("button", { name: "提交并推送…" });
  await expect.element(previewBtn).toBeDisabled();
  await expect.element(pushBtn).toBeDisabled();

  await banner.getByRole("button", { name: "刷新历史与暂存区核实" }).click();
  await expect.element(previewBtn).toBeEnabled();
});

test("recovers in-flight commit on network failure via GET /git/operations without duplicate POST /git/commit", async () => {
  let commitPostCount = 0;
  let capturedClientReqId = "";

  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = typeof input === "string" ? input : input.toString();
    if (
      url.endsWith("/api/repositories/repo-story/worktrees/wt-main/git/commit")
    ) {
      commitPostCount += 1;
      const body = JSON.parse(String(init?.body ?? "{}")) as {
        clientRequestId?: string;
      };
      capturedClientReqId = body.clientRequestId ?? "";
      throw new TypeError("Failed to fetch");
    }
    if (url.includes("/api/repositories/repo-story/git/operations")) {
      return new Response(
        JSON.stringify({
          operations: [
            {
              id: "op-recovered-browser-1",
              repositoryId: "repo-story",
              worktreeId: "wt-main",
              clientRequestId: capturedClientReqId,
              kind: "commit",
              status: "succeeded",
              preview: {
                worktreePath: "/work/webmux",
                headState: "branch",
                branch: "main",
                headOid: "2222222222222222222222222222222222222222",
                expectedTreeOid: "7316cdbc05a7070b8a86588a9bbaaf90fddca4e4",
              },
              subject: "feat(git): stage S workbench foundations",
              actualOid: "9999999999999999999999999999999999999999",
              actualTreeOid: "7316cdbc05a7070b8a86588a9bbaaf90fddca4e4",
              parents: ["2222222222222222222222222222222222222222"],
              fetchResults: [],
              reviewReasons: [],
              summary:
                "已提交 9999999：feat(git): stage S workbench foundations",
              sanitizedOutput:
                "[main 9999999] feat(git): stage S workbench foundations",
              startedAt: "2026-04-14T10:00:00Z",
              finishedAt: "2026-04-14T10:00:01Z",
            },
          ],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      );
    }
    throw new Error(`Unexpected fetch URL: ${url}`);
  });

  let restoredOutcome: GitCommitOutcome | null = null;
  const opsCtrl = createGitOperationsController({
    getRepoId: () => "repo-story",
    getWorktreeId: () => "wt-main",
    onOutcomeRestored: (o) => {
      restoredOutcome = o;
    },
    onStatusBanner: () => {},
  });

  let view: GitWorkbenchViewState = buildGitWorkbenchFixture("commit-preview");

  await runConfirmCommitFlow({
    repoId: "repo-story",
    wtId: "wt-main",
    confirmationId: "confirm-browser-1",
    current: view,
    signal: new AbortController().signal,
    operations: opsCtrl,
    clearWebmuxStaged: () => {},
    refreshStaging: async () => {},
    patchView: (patch) => {
      view = { ...view, ...patch };
    },
  });

  expect(commitPostCount).toBe(1);
  expect(capturedClientReqId).toMatch(/^commit-/);
  expect(view.commitPreview).toBeNull();
  expect(view.commitMessage).toBe("");
  expect(view.lastCommitOutcome?.status).toBe("matched");
  expect(view.lastCommitOutcome?.operationId).toBe("op-recovered-browser-1");
  expect(restoredOutcome).toBeNull();
  opsCtrl.dispose();
});
