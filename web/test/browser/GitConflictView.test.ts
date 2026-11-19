import { afterEach, expect, test, vi } from "vite-plus/test";
import { page, userEvent } from "vite-plus/test/browser";
import { cleanup, render } from "vitest-browser-svelte/pure";
import GitConflictView from "../../src/components/workspace/GitConflictView.svelte";
import type {
  GitConflictFileDetail,
  GitInProgressOperation,
  GitResolveConflictResponse,
  GitSaveConflictResponse,
} from "../../src/contracts/schemas";
import { createGitConflictController } from "../../src/hooks/workspace/git-conflict-controller.svelte";
import { buildGitWorkbenchFixture } from "../../stories/git-workbench-fixtures";
import Harness from "./GitWorkbenchHarness.svelte";

const conflictApiMocks = vi.hoisted(() => ({
  gitConflictDetail: vi.fn(),
  gitSaveConflict: vi.fn(),
  gitResolveConflict: vi.fn(),
}));

vi.mock("../../src/platform/git-conflict-api", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("../../src/platform/git-conflict-api")
    >();
  return {
    ...actual,
    gitConflictDetail: conflictApiMocks.gitConflictDetail,
    gitSaveConflict: conflictApiMocks.gitSaveConflict,
    gitResolveConflict: conflictApiMocks.gitResolveConflict,
  };
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function makeConflictDetailForPath(
  base: GitConflictFileDetail,
  path: string,
  sha = "sha-initial",
): GitConflictFileDetail {
  return {
    ...base,
    path,
    identity: {
      ...base.identity,
      path,
      worktreeSha256: sha,
    },
  };
}

afterEach(async () => {
  vi.clearAllMocks();
  vi.restoreAllMocks();
  localStorage.clear();
  try {
    await cleanup();
    await userEvent.cleanup();
  } finally {
    await page.viewport(1280, 900);
  }
});

test("desktop 3-way conflict resolver keeps Center Result editable while toggling Base comparison and enforces Draft -> Save Result -> Mark as Resolved -> Continue", async () => {
  await page.viewport(1280, 900);
  const initialState = buildGitWorkbenchFixture("conflict-three-way-text");
  await render(Harness, { initialState });

  await expect
    .element(page.getByTestId("git-in-progress-banner"))
    .toBeVisible();
  await expect.element(page.getByTestId("git-conflict-view")).toBeVisible();
  await expect
    .element(page.getByTestId("git-conflict-left-column"))
    .toBeVisible();
  await expect
    .element(page.getByTestId("git-conflict-center-result"))
    .toBeVisible();
  await expect
    .element(page.getByTestId("git-conflict-right-column"))
    .toBeVisible();

  // Left/Right rebase headers keep non-inverted semantics: Left = Current Branch, Right = Rebase Target Base
  expect(
    page.getByTestId("git-conflict-left-column").element().textContent,
  ).toContain("当前重放分支提交 (stage 3)");
  expect(
    page.getByTestId("git-conflict-right-column").element().textContent,
  ).toContain("变基目标基底 (stage 2)");

  // Toggle Base comparison on Left — Center Result MUST remain mounted & editable
  await page.getByRole("radio", { name: "左侧对照 Base" }).click();
  await expect
    .element(page.getByTestId("git-conflict-result-textarea"))
    .toBeVisible();
  expect(
    page.getByTestId("git-conflict-left-column").element().textContent,
  ).toContain("共同祖先 Base");

  const saveBtn = page.getByTestId("git-conflict-save-btn");
  const resolveBtn = page.getByTestId("git-conflict-resolve-btn");
  const continueBtn = page.getByTestId("git-conflict-continue-btn");

  await expect.element(resolveBtn).toBeDisabled();
  await expect.element(continueBtn).toBeDisabled();

  // Stage 1: Apply non-conflicting + Accept Left on conflict chunk
  await page.getByTestId("git-conflict-apply-nonconflicting-btn").click();
  await page.getByTestId("conflict-accept-left-seg-2").click();

  const textarea = page.getByTestId("git-conflict-result-textarea");
  await expect
    .element(textarea)
    .toHaveValue(
      "const defaultTimeout = 10 * time.Second\nconst maxDiffBytes = 2 << 20\nconst allowForcePush = false // V2 invariant\n",
    );

  // Still disabled until Save Result writes to worktree
  await expect.element(resolveBtn).toBeDisabled();
  await expect.element(continueBtn).toBeDisabled();

  // Stage 2: Save Result -> enables Mark as Resolved, Continue still disabled
  await saveBtn.click();
  await expect.element(resolveBtn).toBeEnabled();
  await expect.element(continueBtn).toBeDisabled();

  // Stage 3: Mark as Resolved -> transitions to all-resolved state & enables Continue
  await resolveBtn.click();
  await expect
    .element(page.getByTestId("git-conflict-all-resolved"))
    .toBeVisible();

  // Stage 4: Continue completes operation
  await page
    .getByTestId("git-conflict-all-resolved")
    .getByRole("button", { name: "继续操作 (Continue)" })
    .click();
  await expect
    .element(page.getByTestId("git-in-progress-banner"))
    .not.toBeInTheDocument();
});

test("renders binary conflict variant card with explicit left/right/delete resolution actions", async () => {
  await page.viewport(1280, 900);
  const initialState = buildGitWorkbenchFixture("conflict-binary-variant");
  await render(Harness, { initialState });

  await expect
    .element(page.getByTestId("git-conflict-variant-card"))
    .toBeVisible();
  await expect
    .element(page.getByTestId("git-conflict-choose-left-btn"))
    .toBeEnabled();
  await expect
    .element(page.getByTestId("git-conflict-choose-right-btn"))
    .toBeEnabled();
  await expect
    .element(page.getByTestId("git-conflict-delete-file-btn"))
    .toBeEnabled();

  await page.getByTestId("git-conflict-choose-left-btn").click();
  await expect
    .element(page.getByTestId("git-conflict-all-resolved"))
    .toBeVisible();
});

test("mobile conflict resolver fits 390px and 320px viewports without horizontal overflow and keeps Result editor continuously visible", async () => {
  await page.viewport(320, 568);
  const initialState = buildGitWorkbenchFixture("conflict-three-way-text");
  await render(Harness, { initialState });

  await expect
    .element(page.getByTestId("git-conflict-mobile-pane"))
    .toBeVisible();
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );

  // Switch read-only source to Common Ancestor Base while Result editor stays visible
  await page.getByRole("radio", { name: "共同祖先 Base" }).click();
  const mobileResult = page.getByTestId("git-conflict-mobile-textarea");
  await expect.element(mobileResult).toBeVisible();
  expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
    window.innerWidth,
  );
});

test("retains local conflict edit and selection through mobile and back", async () => {
  await page.viewport(1280, 900);
  await render(Harness, {
    initialState: buildGitWorkbenchFixture("conflict-three-way-text"),
  });
  const desktop = page.getByTestId("git-conflict-result-textarea");
  const mobile = page.getByTestId("git-conflict-mobile-textarea");
  await userEvent.fill(desktop, "draft adopted from both sides");
  (desktop.element() as HTMLTextAreaElement).setSelectionRange(6, 13);
  const report = [];
  for (const [width, height, field] of [
    [390, 844, mobile],
    [667, 375, mobile],
    [1280, 900, desktop],
  ] as const) {
    await page.viewport(width, height);
    await expect.element(field).toBeVisible();
    await expect.element(field).toHaveValue("draft adopted from both sides");
    const node = field.element() as HTMLTextAreaElement;
    report.push([
      width,
      node.selectionStart,
      node.selectionEnd,
      node === document.activeElement,
    ]);
  }
  expect(
    report.every(
      ([, start, end, focused]) => start === 6 && end === 13 && focused,
    ),
  ).toBe(true);
});

test("conflict controller caches unsaved drafts across file switches and close/reopen, and clears cache on reloadCurrent or identity change", async () => {
  await page.viewport(1280, 900);
  const fixture = buildGitWorkbenchFixture("conflict-three-way-text");
  const seedDetail = fixture.conflictDetail!;
  let shaByPath: Record<string, string> = {
    "a.go": "sha-a-1",
    "b.go": "sha-b-1",
  };

  conflictApiMocks.gitConflictDetail.mockImplementation(
    async (_repoId: string, _wtId: string, path: string) =>
      makeConflictDetailForPath(seedDetail, path, shaByPath[path] ?? "sha-x"),
  );

  const inProgress: GitInProgressOperation = {
    ...fixture.inProgress!,
    conflictedCount: 2,
    conflictedFiles: [
      { path: "a.go", xy: "UU" },
      { path: "b.go", xy: "UU" },
    ],
  };

  const ctrl = createGitConflictController({
    getRepoId: () => "repo-1",
    getWorktreeId: () => "wt-1",
    getInProgress: () => inProgress,
    onInProgressUpdated: () => {},
    onRefreshStaging: () => {},
    onStatusBanner: () => {},
  });

  await ctrl.openResolver("a.go");
  await render(GitConflictView, {
    conflict: ctrl,
    inProgress,
    onClose: () => ctrl.closeResolver(),
  });

  const textarea = page.getByTestId("git-conflict-result-textarea");
  await page.getByTestId("conflict-accept-left-seg-2").click();
  await userEvent.fill(textarea, "// unsaved manual draft for a.go\n");

  // Switch to b.go and back to a.go: unsaved manual edits and chunk choice remain intact.
  await ctrl.selectFile("b.go");
  expect(ctrl.activePath).toBe("b.go");
  await expect.element(textarea).toHaveValue(seedDetail.initialResultText);

  await ctrl.selectFile("a.go");
  expect(ctrl.draft?.choices["seg-2"]).toBe("left");
  await expect
    .element(textarea)
    .toHaveValue("// unsaved manual draft for a.go\n");

  // Close and reopen resolver: cached draft still restores when snapshot identity matches.
  ctrl.closeResolver();
  expect(ctrl.open).toBe(false);
  await ctrl.openResolver("a.go");
  expect(ctrl.draft?.choices["seg-2"]).toBe("left");
  expect(ctrl.draft?.resultText).toBe("// unsaved manual draft for a.go\n");

  // Explicit reloadCurrent discards cached draft and reloads base snapshot.
  await ctrl.reloadCurrent();
  expect(ctrl.draft?.choices["seg-2"]).toBe("unresolved");
  expect(ctrl.draft?.resultText).toBe(seedDetail.initialResultText);

  // If snapshot identity changes externally while switched away, stale cache is evicted.
  ctrl.setResultText("// stale draft before external change\n");
  await ctrl.selectFile("b.go");
  shaByPath = { ...shaByPath, "a.go": "sha-a-external" };
  await ctrl.selectFile("a.go");
  expect(ctrl.draft?.resultText).toBe(seedDetail.initialResultText);

  ctrl.dispose();
});

test("conflict controller blocks file switching during save/resolve and cancels in-flight actions on resetTarget", async () => {
  await page.viewport(1280, 900);
  const fixture = buildGitWorkbenchFixture("conflict-three-way-text");
  const seedDetail = fixture.conflictDetail!;

  conflictApiMocks.gitConflictDetail.mockImplementation(
    async (_repoId: string, _wtId: string, path: string) =>
      makeConflictDetailForPath(seedDetail, path, `sha-${path}`),
  );

  const pendingSave = deferred<GitSaveConflictResponse>();
  let capturedSaveSignal: AbortSignal | undefined;
  conflictApiMocks.gitSaveConflict.mockImplementation(
    (_r: string, _w: string, _req: unknown, signal?: AbortSignal) => {
      capturedSaveSignal = signal;
      return pendingSave.promise;
    },
  );

  const inProgress: GitInProgressOperation = {
    ...fixture.inProgress!,
    conflictedCount: 2,
    conflictedFiles: [
      { path: "a.go", xy: "UU" },
      { path: "b.go", xy: "UU" },
    ],
  };

  let refreshDeferred = deferred<void>();
  const banners: string[] = [];
  const ctrl = createGitConflictController({
    getRepoId: () => "repo-1",
    getWorktreeId: () => "wt-1",
    getInProgress: () => inProgress,
    onInProgressUpdated: () => {},
    onRefreshStaging: () => refreshDeferred.promise,
    onStatusBanner: (msg) => banners.push(msg),
  });

  await ctrl.openResolver("a.go");
  await render(GitConflictView, {
    conflict: ctrl,
    inProgress,
    onClose: () => ctrl.closeResolver(),
  });

  const fileSelect = page.getByLabelText("切换冲突文件");
  await expect.element(fileSelect).toBeEnabled();

  // Start saveResult: file switcher disables and programmatic selectFile is ignored.
  ctrl.chooseChunk("seg-2", "left");
  ctrl.applyAllNonConflicting();
  const saveTask = ctrl.saveResult();
  await expect.element(fileSelect).toBeDisabled();
  await ctrl.selectFile("b.go");
  expect(ctrl.activePath).toBe("a.go");

  // Reset target mid-save aborts signal, clears saving flag, and drops late response safely.
  ctrl.resetTarget();
  expect(capturedSaveSignal?.aborted).toBe(true);
  expect(ctrl.saving).toBe(false);
  pendingSave.resolve({
    path: "a.go",
    identity: {
      ...seedDetail.identity,
      path: "a.go",
      worktreeSha256: "sha-saved",
    },
  });
  await saveTask;
  expect(ctrl.detail).toBeNull();
  expect(banners).toHaveLength(0);

  // Reopen, save cleanly, then start markAsResolved and resetTarget during onRefreshStaging.
  conflictApiMocks.gitSaveConflict.mockResolvedValueOnce({
    path: "a.go",
    identity: {
      ...seedDetail.identity,
      path: "a.go",
      worktreeSha256: "sha-saved-2",
    },
  });
  let capturedResolveSignal: AbortSignal | undefined;
  conflictApiMocks.gitResolveConflict.mockImplementationOnce(
    async (
      _r: string,
      _w: string,
      _req: unknown,
      signal?: AbortSignal,
    ): Promise<GitResolveConflictResponse> => {
      capturedResolveSignal = signal;
      return {
        path: "a.go",
        inProgress: {
          ...inProgress,
          conflictedCount: 1,
          conflictedFiles: [{ path: "b.go", xy: "UU" }],
        },
      };
    },
  );

  refreshDeferred = deferred<void>();
  await ctrl.openResolver("a.go");
  ctrl.chooseChunk("seg-2", "left");
  ctrl.applyAllNonConflicting();
  await ctrl.saveResult();
  expect(banners).toHaveLength(1);

  const resolveTask = ctrl.markAsResolved();
  await vi.waitFor(() => expect(ctrl.resolving).toBe(true));
  expect(capturedResolveSignal).toBeDefined();

  // Reset target while awaiting onRefreshStaging: late staging completion does not load b.go.
  ctrl.resetTarget();
  expect(ctrl.resolving).toBe(false);
  refreshDeferred.resolve();
  await resolveTask;
  expect(ctrl.activePath).toBeNull();
  expect(ctrl.detail).toBeNull();

  ctrl.dispose();
});

test("CRLF conflict file preserves CRLF across textarea manual edits, chunk choices, saveResult, and markAsResolved", async () => {
  await page.viewport(1280, 900);
  const fixture = buildGitWorkbenchFixture("conflict-three-way-text");
  const baseDetail = fixture.conflictDetail!;
  const crlfDetail: GitConflictFileDetail = {
    ...baseDetail,
    base: { ...baseDetail.base, lineEnding: "crlf" },
    left: { ...baseDetail.left, lineEnding: "crlf" },
    right: { ...baseDetail.right, lineEnding: "crlf" },
    initialResultText: baseDetail.initialResultText.replace(/\r?\n/g, "\r\n"),
  };

  conflictApiMocks.gitConflictDetail.mockResolvedValue(crlfDetail);
  const savedPayloads: string[] = [];
  conflictApiMocks.gitSaveConflict.mockImplementation(
    async (_r: string, _w: string, req: { resultText: string }) => {
      savedPayloads.push(req.resultText);
      return {
        path: crlfDetail.path,
        identity: { ...crlfDetail.identity, worktreeSha256: "sha-crlf-saved" },
      };
    },
  );
  const resolvedPayloads: Array<string | undefined> = [];
  conflictApiMocks.gitResolveConflict.mockImplementation(
    async (_r: string, _w: string, req: { resultText?: string }) => {
      resolvedPayloads.push(req.resultText);
      return {
        path: crlfDetail.path,
        inProgress: {
          ...fixture.inProgress!,
          conflictedCount: 0,
          conflictedFiles: [],
          canContinue: true,
        },
      };
    },
  );

  const inProgress = fixture.inProgress!;
  const ctrl = createGitConflictController({
    getRepoId: () => "repo-1",
    getWorktreeId: () => "wt-1",
    getInProgress: () => inProgress,
    onInProgressUpdated: () => {},
    onRefreshStaging: () => {},
    onStatusBanner: () => {},
  });

  await ctrl.openResolver(crlfDetail.path);
  await render(GitConflictView, {
    conflict: ctrl,
    inProgress,
    onClose: () => ctrl.closeResolver(),
  });

  const textarea = page.getByTestId("git-conflict-result-textarea");
  // Browser textarea normalizes \r\n to \n on user edits.
  await userEvent.fill(
    textarea,
    [
      "// manual header",
      "const defaultTimeout = 5 * time.Second",
      "const maxDiffBytes = 1 << 20",
      "const allowForcePush = false",
      "",
    ].join("\n"),
  );
  expect(ctrl.draft?.resultText.includes("\r\n")).toBe(false);

  // Clicking chunk choice restores CRLF in draft.resultText.
  await page.getByTestId("conflict-accept-left-seg-2").click();
  expect(ctrl.draft?.resultText).toBe(
    [
      "// manual header",
      "const defaultTimeout = 5 * time.Second",
      "const maxDiffBytes = 2 << 20",
      "const allowForcePush = false",
      "",
    ].join("\r\n"),
  );

  // Edit in textarea again right before saving (re-normalizing value to \n).
  await userEvent.fill(
    textarea,
    [
      "// manual header updated",
      "const defaultTimeout = 5 * time.Second",
      "const maxDiffBytes = 2 << 20",
      "const allowForcePush = false",
      "",
    ].join("\n"),
  );
  expect(ctrl.draft?.resultText.includes("\r\n")).toBe(false);

  await page.getByTestId("git-conflict-save-btn").click();
  const expectedCrlf = [
    "// manual header updated",
    "const defaultTimeout = 5 * time.Second",
    "const maxDiffBytes = 2 << 20",
    "const allowForcePush = false",
    "",
  ].join("\r\n");
  expect(savedPayloads).toEqual([expectedCrlf]);

  const resolveBtn = page.getByTestId("git-conflict-resolve-btn");
  await expect.element(resolveBtn).toBeEnabled();
  await resolveBtn.click();
  expect(resolvedPayloads).toEqual([expectedCrlf]);

  ctrl.dispose();
});
