import { describe, expect, it } from "vitest";
import {
  NativePreviewSchema,
  NativeResultSchema,
} from "../src/contracts/schemas";

describe("native history boundary", () => {
  it("keeps page positions exact and source boundaries opaque", () => {
    const value = {
      agent: "synthetic",
      nativeId: "native",
      branch: "leaf",
      format: "format-v1",
      boundary: "opaque-native-boundary",
      contentHash: "content-version",
      expiresAt: "",
      truncated: false,
      more: true,
      recordCount: 1,
      files: [],
      snapshotId: "snapshot",
      parser: "reader-v1",
      nextPosition: "9007199254740993",
      nextTextOffset: "0",
      pending: true,
      gaps: [],
      records: [],
    };
    const parsed = NativeResultSchema.parse(value);
    expect(parsed.nextPosition).toBe("9007199254740993");
    expect(parsed.boundary).toBe("opaque-native-boundary");
    expect(
      NativeResultSchema.safeParse({
        ...value,
        nextPosition: Number("9007199254740993"),
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
