import { describe, expect, it } from "vitest";
import {
  NativePreviewSchema,
  NativeResultSchema,
} from "../src/contracts/schemas";

describe("native history boundary", () => {
  it("keeps snapshot cursors as decimal strings", () => {
    const value = {
      nativeId: "",
      model: "",
      provenance: [],
      files: [],
      sourceId: "source",
      conversationId: "conversation",
      snapshotId: "snapshot",
      parser: "codex-0.77/v1",
      consumed: "9007199254740993",
      pending: true,
      gaps: [],
      activities: {
        items: [],
        gaps: [],
        nextPosition: "0",
        reconcileRequired: false,
        historyOrigin: "bounded_import",
      },
    };
    expect(NativeResultSchema.parse(value).consumed).toBe("9007199254740993");
    expect(
      NativeResultSchema.safeParse({
        ...value,
        consumed: Number("9007199254740993"),
      }).success,
    ).toBe(false);
  });
  it("requires render limitation fields", () => {
    const value = {
      gaps: [],
      id: "derived",
      version: "v1",
      currentTask: "  原样\n任务  ",
      segments: [],
      omissions: [],
      pages: [],
      uncovered: [],
      missingGlyphCount: 1,
      problem: "derivation_failed",
    };
    expect(NativePreviewSchema.parse(value).currentTask).toBe(
      value.currentTask,
    );
    expect(
      NativePreviewSchema.safeParse({ ...value, missingGlyphCount: -1 })
        .success,
    ).toBe(false);
  });
});
