import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, test } from "vite-plus/test";
import { GitWorkingChangeSchema } from "../src/contracts/git-workbench-schemas";
import {
  GitDiscardPreviewSchema,
  GitDiscardResultSchema,
  type GitDiscardResult,
  GitOperationsSchema,
  GitSmartCheckoutPreviewSchema,
  GitSmartCheckoutResultSchema,
  GitStageRequestSchema,
  SwitchPreviewSchema,
} from "../src/contracts/schemas";
import {
  consumeConflictResolverHandoff,
  requestConflictResolverHandoff,
} from "../src/domains/workspace/git-conflict-handoff";
import {
  applyOfflineDiscardChanges,
  applyOfflineLineMutation,
  countSelectedLines,
  mapDiffRangeToLineSelections,
  selectAllChangedInHunk,
  toggleHunkLineSelection,
  toGitLineSelections,
  type HunkLineSelectionMap,
} from "../src/domains/workspace/git-line-selection";
import { gitOperationKindLabel } from "../src/domains/workspace/git-operations-model";
import { shouldOfferSmartCheckout } from "../src/hooks/workspace/git-smart-checkout.svelte";

const wireDir = resolve(
  import.meta.dirname,
  "../../internal/api/testdata/wire",
);

function readWireJson(name: string): unknown {
  const raw = JSON.parse(readFileSync(resolve(wireDir, name), "utf8")) as {
    body: unknown;
  };
  return JSON.parse(
    JSON.stringify(raw.body)
      .replaceAll('"<time>"', '"2026-01-02T03:04:05Z"')
      .replaceAll(/<id#\d+>/g, "aaaaaaaa")
      .replaceAll(/<generation#\d+>/g, "0123456789abcdef"),
  );
}

const sampleHunk = {
  id: "hunk-0",
  header: "@@ -10,5 +10,6 @@",
  oldStart: 10,
  oldLines: 5,
  newStart: 10,
  newLines: 6,
  lines: [
    {
      kind: "context" as const,
      oldLine: 10,
      newLine: 10,
      text: "const a = 1;",
      lineEnding: "lf" as const,
    },
    {
      kind: "delete" as const,
      oldLine: 11,
      text: "const b = 2;",
      lineEnding: "lf" as const,
    },
    {
      kind: "add" as const,
      newLine: 11,
      text: "const b = 20;",
      lineEnding: "lf" as const,
    },
    {
      kind: "add" as const,
      newLine: 12,
      text: "const c = 30;",
      lineEnding: "lf" as const,
    },
    {
      kind: "context" as const,
      oldLine: 12,
      newLine: 13,
      text: "return a + b;",
      lineEnding: "lf" as const,
    },
  ],
};

describe("Git Workbench V3 (#551): Wire Contract Schemas", () => {
  test("parses backend wire snapshots for discard, smart-checkout, and operations", () => {
    const discardPreview = GitDiscardPreviewSchema.parse(
      readWireJson("git-discard-preview.json"),
    );
    expect(discardPreview.scope).toBe("files");
    expect(discardPreview.confirmationId).toBeTruthy();

    const discardResult: GitDiscardResult = GitDiscardResultSchema.parse(
      readWireJson("git-discard.json"),
    );
    expect(discardResult.recoveryOid).toBeTruthy();
    expect(discardResult.recoveryCommand).toBeTruthy();

    const smartPreview = GitSmartCheckoutPreviewSchema.parse(
      readWireJson("git-smart-checkout-preview.json"),
    );
    expect(smartPreview.confirmationId).toBeTruthy();
    expect(smartPreview.untrackedCount).toBeGreaterThanOrEqual(1);

    const smartResult = GitSmartCheckoutResultSchema.parse(
      readWireJson("git-smart-checkout.json"),
    );
    expect(smartResult.outcome).toBe("completed");

    const ops = GitOperationsSchema.parse(readWireJson("git-operations.json"));
    expect(ops.operations.length).toBeGreaterThan(0);
    expect(gitOperationKindLabel("discard")).toBe("丢弃更改");
    expect(gitOperationKindLabel("smart_checkout")).toBe("Smart Checkout");
  });

  test("validates GitStageRequestSchema with arbitrary sub-hunk lineSelections", () => {
    const req = GitStageRequestSchema.parse({
      identity: {
        path: "src/main.ts",
        status: "modified",
        area: "unstaged",
        indexOid: "1111111111111111111111111111111111111111",
        worktreeHash:
          "2222222222222222222222222222222222222222222222222222222222222222",
        mode: "100644",
      },
      lineSelections: [{ hunkId: "hunk-0", lineIndices: [1, 2] }],
    });
    expect(req.lineSelections).toEqual([
      { hunkId: "hunk-0", lineIndices: [1, 2] },
    ]);
  });
});

describe("Git Workbench V3 (#551): Sub-hunk Line Selection & Offline Splicing", () => {
  test("supports single-line toggle, Shift-click contiguous range selection, and hunk select-all", () => {
    let map: HunkLineSelectionMap = {};
    // Context line (index 0) is ignored.
    const ign = toggleHunkLineSelection({
      current: map,
      hunk: sampleHunk,
      lineIndex: 0,
      anchorIndex: null,
    });
    expect(ign.next).toEqual({});

    // Toggle delete line at index 1.
    const s1 = toggleHunkLineSelection({
      current: map,
      hunk: sampleHunk,
      lineIndex: 1,
      anchorIndex: null,
    });
    map = s1.next;
    expect(map["hunk-0"]).toEqual([1]);

    // Shift-click index 3 selects contiguous changed lines [1, 2, 3].
    const s2 = toggleHunkLineSelection({
      current: map,
      hunk: sampleHunk,
      lineIndex: 3,
      anchorIndex: s1.nextAnchor,
      shiftKey: true,
    });
    map = s2.next;
    expect(map["hunk-0"]).toEqual([1, 2, 3]);
    const selections = toGitLineSelections(map, [sampleHunk]);
    expect(countSelectedLines(selections)).toBe(3);

    // Select-all toggles clear when all changed lines are already selected.
    map = selectAllChangedInHunk(map, sampleHunk);
    expect(map["hunk-0"]).toBeUndefined();
    map = selectAllChangedInHunk(map, sampleHunk);
    expect(map["hunk-0"]).toEqual([1, 2, 3]);
  });

  test("maps CodeDiffReader range selections to hunk lineIndices", () => {
    expect(
      mapDiffRangeToLineSelections([sampleHunk], {
        side: "deletions",
        start: 11,
        end: 11,
      }),
    ).toEqual([{ hunkId: "hunk-0", lineIndices: [1] }]);

    expect(
      mapDiffRangeToLineSelections([sampleHunk], {
        side: "additions",
        start: 11,
        end: 12,
      }),
    ).toEqual([{ hunkId: "hunk-0", lineIndices: [2, 3] }]);
  });

  test("splices staged/unstaged/discarded lines while keeping staged entries safe from discard", () => {
    const unstaged = GitWorkingChangeSchema.parse({
      id: "unstaged:src/app.ts",
      group: "unstaged",
      status: "M",
      path: "src/app.ts",
      hunkStageAvailable: true,
      hunks: [sampleHunk],
    });
    const stagedExisting = GitWorkingChangeSchema.parse({
      id: "staged:src/app.ts-other",
      group: "staged",
      status: "M",
      path: "src/app.ts",
      hunkStageAvailable: true,
      hunks: [],
    });
    const untracked = GitWorkingChangeSchema.parse({
      id: "untracked:notes.txt",
      group: "untracked",
      status: "?",
      path: "notes.txt",
      hunkStageAvailable: false,
      hunks: [],
    });

    // Stage only line index 2 (`const b = 20;`), leaving index 1 & 3 in unstaged.
    const afterStage = applyOfflineLineMutation(
      [stagedExisting, unstaged, untracked],
      unstaged.id,
      [{ hunkId: "hunk-0", lineIndices: [2] }],
      "stage",
    );
    const nextStaged = afterStage.find(
      (c) => c.group === "staged" && c.path === "src/app.ts",
    );
    const nextUnstaged = afterStage.find(
      (c) => c.group === "unstaged" && c.path === "src/app.ts",
    );
    expect(nextStaged?.hunks.length).toBe(1);
    expect(
      nextStaged?.hunks[0]?.lines.filter((l) => l.kind === "add").length,
    ).toBe(1);
    expect(
      nextUnstaged?.hunks[0]?.lines.filter((l) => l.kind !== "context").length,
    ).toBe(2);

    // File-level discard NEVER discards staged entries (even for the same path)!
    const afterDiscard = applyOfflineDiscardChanges(afterStage, {
      scope: "files",
      paths: ["src/app.ts", "notes.txt"],
    });
    expect(afterDiscard.some((c) => c.group === "staged")).toBe(true);
    expect(afterDiscard.some((c) => c.group === "unstaged")).toBe(false);
    expect(afterDiscard.some((c) => c.group === "untracked")).toBe(false);
  });
});

describe("Git Workbench V3 (#551): Smart Checkout Offer & Conflict Handoff", () => {
  const baseSwitchPreview = SwitchPreviewSchema.parse(
    readWireJson("worktree-switch-preview.json"),
  );

  test("offers Smart Checkout only when changes block switch without hard structural blockers", () => {
    const dirtyOnly = {
      ...baseSwitchPreview,
      blockers: [{ code: "changes", message: "有未提交的修改" }],
    };
    expect(shouldOfferSmartCheckout(dirtyOnly, true)).toBe(true);
    expect(shouldOfferSmartCheckout(dirtyOnly, false)).toBe(false);

    const occupiedToo = {
      ...baseSwitchPreview,
      blockers: [
        { code: "changes", message: "有未提交的修改" },
        { code: "occupied", message: "已在另一工作树检出", path: "/tmp/wt" },
      ],
    };
    expect(shouldOfferSmartCheckout(occupiedToo, true)).toBe(false);
  });

  test("records and consumes one-shot 3-way conflict resolver handoff", () => {
    requestConflictResolverHandoff("repo-1", "wt-1", "src/conflict.ts");
    expect(consumeConflictResolverHandoff("repo-other", "wt-1")).toEqual({
      matched: false,
    });
    expect(consumeConflictResolverHandoff("repo-1", "wt-1")).toEqual({
      matched: true,
      path: "src/conflict.ts",
    });
    // Consumed once.
    expect(consumeConflictResolverHandoff("repo-1", "wt-1")).toEqual({
      matched: false,
    });
  });
});
