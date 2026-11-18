import { describe, expect, it } from "vite-plus/test";
import { GitWorktreeContextSchema } from "../src/contracts/git-workbench-schemas";
import type { GitConflictFileDetail } from "../src/contracts/schemas";
import { resolveGitBranchActions } from "../src/domains/workspace/git-branches-model";
import {
  applyAllNonConflictingSegments,
  applyConflictChunkChoice,
  createInitialConflictDraftState,
  detectTextConflictMarkers,
  markAllUnresolvedAsManual,
  normalizeConflictLineEndings,
  summarizeConflictDraftProgress,
  toggleNonConflictingSegment,
} from "../src/domains/workspace/git-conflict-model";
import {
  formatInProgressHeadline,
  formatPushRemoteTrackingRef,
  gitDivergenceConfirmLabel,
  gitDivergenceTitleLabel,
  gitSequencerKindLabel,
} from "../src/domains/workspace/git-divergence-model";

function makeSampleConflictDetail(
  overrides: Partial<GitConflictFileDetail> = {},
): GitConflictFileDetail {
  return {
    repositoryId: "repo-1",
    worktreeId: "wt-1",
    worktreePath: "/tmp/repo",
    path: "src/config.ts",
    xy: "UU",
    kind: "text",
    textMergeable: true,
    operationKind: "rebase",
    identity: {
      path: "src/config.ts",
      operationKind: "rebase",
      stage1Mode: "100644",
      stage1Oid: "1111111",
      stage2Mode: "100644",
      stage2Oid: "2222222",
      stage3Mode: "100644",
      stage3Oid: "3333333",
      leftStage: 3,
      rightStage: 2,
      worktreeMode: "100644",
      worktreeSha256: "abc",
    },
    base: {
      side: "base",
      label: "共同祖先 (Base)",
      role: "共同祖先 (stage 1)",
      stage: 1,
      present: true,
      mode: "100644",
      blobOid: "1111111",
      sizeBytes: 42,
      binary: false,
      text: "const host = 'localhost';\nconst port = 7681;\nconst debug = false;\n",
      lineEnding: "lf",
    },
    left: {
      side: "left",
      label: "feat/auth · 3333333",
      role: "当前重放分支提交 (stage 3)",
      refName: "feat/auth",
      commitOid: "3333333333333333333333333333333333333333",
      subject: "feat: customize port",
      stage: 3,
      present: true,
      mode: "100644",
      blobOid: "3333333",
      sizeBytes: 50,
      binary: false,
      text: "const host = '127.0.0.1';\nconst port = 8080;\nconst debug = false;\n",
      lineEnding: "lf",
    },
    right: {
      side: "right",
      label: "origin/main · 2222222",
      role: "变基目标基底 (stage 2)",
      refName: "origin/main",
      commitOid: "2222222222222222222222222222222222222222",
      subject: "chore: production port",
      stage: 2,
      present: true,
      mode: "100644",
      blobOid: "2222222",
      sizeBytes: 52,
      binary: false,
      text: "const host = 'localhost';\nconst port = 9000;\nconst debug = true;\n",
      lineEnding: "lf",
    },
    segments: [
      {
        id: "seg-1",
        kind: "change_left",
        baseStart: 1,
        baseLines: ["const host = 'localhost';"],
        leftStart: 1,
        leftLines: ["const host = '127.0.0.1';"],
        rightStart: 1,
        rightLines: ["const host = 'localhost';"],
      },
      {
        id: "seg-2",
        kind: "conflict",
        baseStart: 2,
        baseLines: ["const port = 7681;"],
        leftStart: 2,
        leftLines: ["const port = 8080;"],
        rightStart: 2,
        rightLines: ["const port = 9000;"],
      },
      {
        id: "seg-3",
        kind: "change_right",
        baseStart: 3,
        baseLines: ["const debug = false;"],
        leftStart: 3,
        leftLines: ["const debug = false;"],
        rightStart: 3,
        rightLines: ["const debug = true;"],
      },
    ],
    initialResultText:
      "const host = 'localhost';\nconst port = 7681;\nconst debug = false;\n",
    diskText:
      "const host = '127.0.0.1';\n<<<<<<< HEAD\nconst port = 9000;\n=======\nconst port = 8080;\n>>>>>>> feat\nconst debug = true;\n",
    diskHasConflictMarker: true,
    diskModifiedFromBase: true,
    ...overrides,
  };
}

describe("git-conflict-model progressive 4-stage workflow", () => {
  it("starts from clean Base text when disk still has raw conflict markers", () => {
    const detail = makeSampleConflictDetail();
    const draft = createInitialConflictDraftState(detail);
    expect(draft.resultText).toBe(detail.initialResultText);
    expect(draft.savedResultText).toBeNull();

    const p0 = summarizeConflictDraftProgress(detail, draft);
    expect(p0.totalConflicts).toBe(1);
    expect(p0.unresolvedConflicts).toBe(1);
    expect(p0.pendingNonConflicting).toBe(2);
    expect(p0.canSaveResult).toBe(true);
    expect(p0.canMarkResolved).toBe(false);
  });

  it("applies non-conflicting segments and conflict choices, requiring Save Result before Mark as Resolved", () => {
    const detail = makeSampleConflictDetail();
    let draft = createInitialConflictDraftState(detail);

    draft = applyAllNonConflictingSegments(detail, draft);
    expect(draft.resultText).toBe(
      "const host = '127.0.0.1';\nconst port = 7681;\nconst debug = true;\n",
    );
    expect(
      summarizeConflictDraftProgress(detail, draft).pendingNonConflicting,
    ).toBe(0);

    // Toggle one non-conflicting segment off and back on
    draft = toggleNonConflictingSegment(detail, draft, "seg-3");
    expect(draft.resultText).toContain("const debug = false;");
    draft = toggleNonConflictingSegment(detail, draft, "seg-3");
    expect(draft.resultText).toContain("const debug = true;");

    // Choose left for conflict chunk
    draft = applyConflictChunkChoice(detail, draft, "seg-2", "left");
    expect(draft.resultText).toBe(
      "const host = '127.0.0.1';\nconst port = 8080;\nconst debug = true;\n",
    );

    // Even with 0 unresolved conflicts, Mark as Resolved MUST stay disabled until Save Result runs!
    const beforeSave = summarizeConflictDraftProgress(detail, draft);
    expect(beforeSave.unresolvedConflicts).toBe(0);
    expect(beforeSave.hasSavedToDisk).toBe(false);
    expect(beforeSave.canSaveResult).toBe(true);
    expect(beforeSave.canMarkResolved).toBe(false);

    // Stage 2: Save Result to worktree disk
    draft = { ...draft, savedResultText: draft.resultText };
    const afterSave = summarizeConflictDraftProgress(detail, draft);
    expect(afterSave.hasSavedToDisk).toBe(true);
    expect(afterSave.canMarkResolved).toBe(true);

    // Subsequent manual edit invalidates saved state until saved again
    draft = {
      ...draft,
      manualEdited: true,
      resultText:
        "const host = '127.0.0.1';\nconst port = 8081;\nconst debug = true;\n",
    };
    expect(summarizeConflictDraftProgress(detail, draft).canMarkResolved).toBe(
      false,
    );
  });

  it("supports both_lr / both_rl choices, manual edit confirmation, conflict marker blocking, and CRLF preservation", () => {
    const crlfDetail = makeSampleConflictDetail({
      left: {
        ...makeSampleConflictDetail().left,
        lineEnding: "crlf",
      },
    });
    let draft = createInitialConflictDraftState(crlfDetail);
    draft = applyConflictChunkChoice(crlfDetail, draft, "seg-2", "both_rl");
    expect(draft.resultText).toBe(
      "const host = 'localhost';\r\nconst port = 9000;\r\nconst port = 8080;\r\nconst debug = false;\r\n",
    );

    // Manual edit while chunk was still unresolved
    let manualDraft = {
      ...createInitialConflictDraftState(crlfDetail),
      manualEdited: true,
      resultText: "const host = '0.0.0.0';\r\nconst port = 7777;\r\n",
    };
    expect(
      summarizeConflictDraftProgress(crlfDetail, manualDraft)
        .unresolvedConflicts,
    ).toBe(1);
    manualDraft = markAllUnresolvedAsManual(manualDraft);
    manualDraft = { ...manualDraft, savedResultText: manualDraft.resultText };
    expect(
      summarizeConflictDraftProgress(crlfDetail, manualDraft).canMarkResolved,
    ).toBe(true);

    // Conflict markers block both Save Result and Mark as Resolved
    expect(
      detectTextConflictMarkers(
        "<<<<<<< HEAD\nconst a = 1;\n=======\nconst a = 2;\n>>>>>>> branch\n",
      ),
    ).toBe(true);
    expect(detectTextConflictMarkers("const clean = true;\n")).toBe(false);
    manualDraft = {
      ...manualDraft,
      resultText:
        "<<<<<<< HEAD\nconst a = 1;\n=======\nconst a = 2;\n>>>>>>> branch\n",
    };
    const blocked = summarizeConflictDraftProgress(crlfDetail, manualDraft);
    expect(blocked.hasConflictMarkers).toBe(true);
    expect(blocked.canSaveResult).toBe(false);
    expect(blocked.canMarkResolved).toBe(false);
  });

  it("restores previously saved clean draft from worktree disk on re-entry", () => {
    const savedOnDisk =
      "const host = '127.0.0.1';\nconst port = 8080;\nconst debug = true;\n";
    const detail = makeSampleConflictDetail({
      diskText: savedOnDisk,
      diskHasConflictMarker: false,
      diskModifiedFromBase: true,
    });
    const draft = createInitialConflictDraftState(detail);
    expect(draft.resultText).toBe(savedOnDisk);
    expect(draft.savedResultText).toBe(savedOnDisk);
    expect(draft.manualEdited).toBe(true);
    const progress = summarizeConflictDraftProgress(detail, draft);
    expect(progress.unresolvedConflicts).toBe(0);
    expect(progress.hasSavedToDisk).toBe(true);
    expect(progress.canMarkResolved).toBe(true);
  });

  it("replaces only the target segment slice when manual edits add or remove lines in other segments", () => {
    const detail = makeSampleConflictDetail();
    let draft = createInitialConflictDraftState(detail);

    // User manually inserts header lines in seg-1 and footer lines in seg-3.
    draft = {
      ...draft,
      manualEdited: true,
      resultText: [
        "// custom header 1",
        "// custom header 2",
        "const host = '0.0.0.0';",
        "const port = 7681;",
        "const debug = false;",
        "export const extraFooter = true;",
        "",
      ].join("\n"),
    };

    // Choosing Left on seg-2 updates only seg-2 and preserves custom header/footer.
    draft = applyConflictChunkChoice(detail, draft, "seg-2", "left");
    expect(draft.resultText).toBe(
      [
        "// custom header 1",
        "// custom header 2",
        "const host = '0.0.0.0';",
        "const port = 8080;",
        "const debug = false;",
        "export const extraFooter = true;",
        "",
      ].join("\n"),
    );

    // Switching seg-2 choice from Left to both_rl tracks previous segment lines and updates seg-2 in place.
    draft = applyConflictChunkChoice(detail, draft, "seg-2", "both_rl");
    expect(draft.resultText).toBe(
      [
        "// custom header 1",
        "// custom header 2",
        "const host = '0.0.0.0';",
        "const port = 9000;",
        "const port = 8080;",
        "const debug = false;",
        "export const extraFooter = true;",
        "",
      ].join("\n"),
    );

    // Toggling non-conflicting seg-3 updates only seg-3 while keeping header and seg-2 intact.
    draft = toggleNonConflictingSegment(detail, draft, "seg-3", true);
    expect(draft.resultText).toBe(
      [
        "// custom header 1",
        "// custom header 2",
        "const host = '0.0.0.0';",
        "const port = 9000;",
        "const port = 8080;",
        "const debug = true;",
        "export const extraFooter = true;",
        "",
      ].join("\n"),
    );
  });

  it("disambiguates duplicate conflict slices via context anchors and preserves wholesale manual rewrites", () => {
    const detail = makeSampleConflictDetail({
      segments: [
        {
          id: "seg-1",
          kind: "common",
          baseStart: 1,
          baseLines: ["function alpha() {"],
          leftStart: 1,
          leftLines: ["function alpha() {"],
          rightStart: 1,
          rightLines: ["function alpha() {"],
        },
        {
          id: "seg-2",
          kind: "conflict",
          baseStart: 2,
          baseLines: ["  return false;"],
          leftStart: 2,
          leftLines: ["  return true; // alpha-left"],
          rightStart: 2,
          rightLines: ["  return true; // alpha-right"],
        },
        {
          id: "seg-3",
          kind: "common",
          baseStart: 3,
          baseLines: ["}", "function beta() {"],
          leftStart: 3,
          leftLines: ["}", "function beta() {"],
          rightStart: 3,
          rightLines: ["}", "function beta() {"],
        },
        {
          id: "seg-4",
          kind: "conflict",
          baseStart: 5,
          baseLines: ["  return false;"],
          leftStart: 5,
          leftLines: ["  return true; // beta-left"],
          rightStart: 5,
          rightLines: ["  return true; // beta-right"],
        },
        {
          id: "seg-5",
          kind: "common",
          baseStart: 6,
          baseLines: ["}"],
          leftStart: 6,
          leftLines: ["}"],
          rightStart: 6,
          rightLines: ["}"],
        },
      ],
      initialResultText: [
        "function alpha() {",
        "  return false;",
        "}",
        "function beta() {",
        "  return false;",
        "}",
        "",
      ].join("\n"),
    });

    let draft = createInitialConflictDraftState(detail);
    draft = {
      ...draft,
      manualEdited: true,
      resultText: [
        "// manual banner",
        "function alpha() {",
        "  return false;",
        "}",
        "function beta() {",
        "  return false;",
        "}",
        "",
      ].join("\n"),
    };

    // Choosing right on seg-4 uses seg-3/seg-5 context anchors so seg-2 stays untouched.
    draft = applyConflictChunkChoice(detail, draft, "seg-4", "right");
    expect(draft.resultText).toBe(
      [
        "// manual banner",
        "function alpha() {",
        "  return false;",
        "}",
        "function beta() {",
        "  return true; // beta-right",
        "}",
        "",
      ].join("\n"),
    );

    // If user edits inside seg-2 directly while seg-1 and seg-3 anchors remain intact, choosing left replaces seg-2.
    draft = {
      ...draft,
      manualEdited: true,
      resultText: [
        "function alpha() {",
        "  const temp = 1;",
        "  return Boolean(temp);",
        "}",
        "function beta() {",
        "  return true; // beta-right",
        "}",
        "",
      ].join("\n"),
    };
    draft = applyConflictChunkChoice(detail, draft, "seg-2", "left");
    expect(draft.resultText).toBe(
      [
        "function alpha() {",
        "  return true; // alpha-left",
        "}",
        "function beta() {",
        "  return true; // beta-right",
        "}",
        "",
      ].join("\n"),
    );

    // Wholesale rewrite with no matching target slice or anchors preserves custom resultText.
    const customRewrite = "export const customConfig = { port: 443 };\n";
    draft = {
      ...draft,
      manualEdited: true,
      resultText: customRewrite,
    };
    draft = applyConflictChunkChoice(detail, draft, "seg-2", "right");
    expect(draft.resultText).toBe(customRewrite);
  });

  it("marks hasUnsavedDraftChanges when non-conflicting segments are applied or manualEdited is set", () => {
    const noOpNonConflictDetail = makeSampleConflictDetail({
      segments: [
        {
          id: "seg-same",
          kind: "change_both_same",
          baseStart: 1,
          baseLines: ["const host = 'localhost';"],
          leftStart: 1,
          leftLines: ["const host = 'localhost';"],
          rightStart: 1,
          rightLines: ["const host = 'localhost';"],
        },
      ],
      initialResultText: "const host = 'localhost';\n",
    });

    const initial = createInitialConflictDraftState(noOpNonConflictDetail);
    expect(
      summarizeConflictDraftProgress(noOpNonConflictDetail, initial)
        .hasUnsavedDraftChanges,
    ).toBe(false);

    const afterNonConflict = toggleNonConflictingSegment(
      noOpNonConflictDetail,
      initial,
      "seg-same",
      true,
    );
    expect(
      summarizeConflictDraftProgress(noOpNonConflictDetail, afterNonConflict)
        .hasUnsavedDraftChanges,
    ).toBe(true);

    const afterManualFlag: typeof initial = {
      ...initial,
      manualEdited: true,
    };
    expect(
      summarizeConflictDraftProgress(noOpNonConflictDetail, afterManualFlag)
        .hasUnsavedDraftChanges,
    ).toBe(true);
  });

  it("preserves CRLF line endings across LF-normalized textarea manual edits, chunk choices, and save normalization", () => {
    const crlfDetail = makeSampleConflictDetail({
      base: {
        ...makeSampleConflictDetail().base,
        text: "const host = 'localhost';\r\nconst port = 7681;\r\nconst debug = false;\r\n",
        lineEnding: "crlf",
      },
      left: {
        ...makeSampleConflictDetail().left,
        text: "const host = '127.0.0.1';\r\nconst port = 8080;\r\nconst debug = false;\r\n",
        lineEnding: "crlf",
      },
      right: {
        ...makeSampleConflictDetail().right,
        text: "const host = 'localhost';\r\nconst port = 9000;\r\nconst debug = true;\r\n",
        lineEnding: "crlf",
      },
      initialResultText:
        "const host = 'localhost';\r\nconst port = 7681;\r\nconst debug = false;\r\n",
    });

    let draft = createInitialConflictDraftState(crlfDetail);
    // Browser HTMLTextAreaElement.value normalizes \r\n to \n on manual edits.
    draft = {
      ...draft,
      manualEdited: true,
      resultText:
        "// custom header\nconst host = 'localhost';\nconst port = 7681;\nconst debug = false;\n",
    };

    // Subsequent chunk click restores CRLF across both manual lines and replaced segment lines.
    draft = applyConflictChunkChoice(crlfDetail, draft, "seg-2", "left");
    expect(draft.resultText).toBe(
      "// custom header\r\nconst host = 'localhost';\r\nconst port = 8080;\r\nconst debug = false;\r\n",
    );

    // Another manual textarea edit right before saving normalizes to \n again;
    // normalizeConflictLineEndings restores \r\n while leaving LF files untouched.
    const editedAgainLf =
      "// custom header\nconst host = 'localhost';\nconst port = 8080;\nconst debug = true;\n";
    expect(normalizeConflictLineEndings(editedAgainLf, crlfDetail)).toBe(
      "// custom header\r\nconst host = 'localhost';\r\nconst port = 8080;\r\nconst debug = true;\r\n",
    );
    const lfDetail = makeSampleConflictDetail();
    expect(normalizeConflictLineEndings(editedAgainLf, lfDetail)).toBe(
      editedAgainLf,
    );
  });
});

describe("git-divergence-model & shared branch actions", () => {
  it("formats sequencer labels, divergence titles, confirm labels, and push remote tracking refs", () => {
    expect(gitSequencerKindLabel("rebase")).toBe("变基 (Rebase)");
    expect(gitSequencerKindLabel("cherry-pick")).toBe("遴选 (Cherry-pick)");
    expect(gitSequencerKindLabel("cherry_pick")).toBe("遴选 (Cherry-pick)");
    expect(gitDivergenceTitleLabel("merge")).toContain("Merge");
    expect(gitDivergenceConfirmLabel("merge", true)).toContain("快进合入");
    expect(gitDivergenceConfirmLabel("revert")).toContain("还原提交");
    expect(formatPushRemoteTrackingRef("origin", "feat/x")).toBe(
      "origin/feat/x",
    );
    expect(formatPushRemoteTrackingRef("origin", "origin/feat/x")).toBe(
      "origin/feat/x",
    );

    expect(
      formatInProgressHeadline({
        active: true,
        kind: "rebase",
        worktreeId: "wt-1",
        worktreePath: "/tmp/repo",
        detached: true,
        sourceLabel: "feat/x",
        targetLabel: "origin/main",
        conflictedCount: 1,
        conflictedFiles: [],
        canContinue: false,
        canAbort: true,
        continueBlockers: [],
      }),
    ).toContain("feat/x 变基到 origin/main");
  });

  it("exposes merge-into-current and rebase-onto actions on non-current branches and diverged upstream", () => {
    const context = GitWorktreeContextSchema.parse({
      repositoryId: "repo-1",
      repositoryName: "webmux",
      worktreeId: "wt-1",
      worktreePath: "/work/webmux",
      worktreeKind: "main",
      checkedOutBranch: "main",
      headOid: "bbbb",
      headState: "branch",
      ahead: 2,
      behind: 1,
      upstream: "origin/main",
    });

    const otherLocalActions = resolveGitBranchActions({
      item: {
        name: "feat/other",
        kind: "local",
        head: "aaaa",
        shortHead: "aaaa",
        subject: "feat",
        committedAt: "2026-11-12T10:00:00Z",
        ahead: 1,
        behind: 0,
        upstreamGone: false,
        mergedIntoDefault: false,
        favorite: false,
      },
      context,
      compareBaseOid: "bbbb",
      branchFilter: null,
    });
    expect(otherLocalActions.some((a) => a.kind === "merge-into-current")).toBe(
      true,
    );
    expect(otherLocalActions.some((a) => a.kind === "rebase-onto")).toBe(true);

    const currentDivergedActions = resolveGitBranchActions({
      item: {
        name: "main",
        kind: "local",
        head: "bbbb",
        shortHead: "bbbb",
        subject: "main",
        committedAt: "2026-11-12T10:00:00Z",
        upstream: "origin/main",
        ahead: 2,
        behind: 1,
        upstreamGone: false,
        mergedIntoDefault: false,
        favorite: false,
      },
      context,
      compareBaseOid: "bbbb",
      branchFilter: null,
    });
    expect(
      currentDivergedActions.some((a) => a.kind === "merge-into-current"),
    ).toBe(true);
    expect(currentDivergedActions.some((a) => a.kind === "rebase-onto")).toBe(
      true,
    );
  });
});
