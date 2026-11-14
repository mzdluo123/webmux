import { expect, test } from "vitest";
import { computeDiff } from "../src/domains/code/code-diff";
import { xrCodePanel } from "../src/domains/code/xr-code-panel";
import type { CodeDiff, CodeDocument } from "../src/contracts/schemas";

function document(text: string, source: "head" | "worktree"): CodeDocument {
  return {
    nodeId: "file",
    path: "src/main.ts",
    absolutePath: "/repo/src/main.ts",
    source,
    oid: source,
    contentVersion: source,
    kind: "text",
    text,
    lineCount: text.split("\n").length,
    referenceable: true,
    reason: "",
    mediaType: "text/plain",
    size: text.length,
    downloadable: true,
  };
}

function panel(old: string, next: string) {
  const metadata = computeDiff(old, next);
  if (metadata.kind !== "ready") throw new Error(metadata.reason);
  const diff: CodeDiff = {
    snapshotId: "snapshot",
    old: document(old, "head"),
    new: document(next, "worktree"),
    kind: "text",
    reason: "",
    checkedAt: "2026-11-14T00:00:00Z",
    change: {
      nodeId: "file",
      changeId: "change",
      group: "unstaged",
      oldPath: "src/main.ts",
      newPath: "src/main.ts",
      status: "M",
      oldOid: "old",
      newOid: "new",
      oldMode: "100644",
      newMode: "100644",
    },
  };
  return xrCodePanel("file", diff.new, diff, metadata.fileDiff, "");
}

test("XR hunks retain real line numbers across unequal replacements", () => {
  const result = panel("same\nold\nend\n", "same\nnew\nextra\nend\n");
  expect(
    result.lines
      .filter((line) => line.tone === "removed")
      .map((line) => line.text.trimEnd()),
  ).toEqual(["-2 old"]);
  expect(
    result.lines
      .filter((line) => line.tone === "added")
      .map((line) => line.text.trimEnd()),
  ).toEqual(["+2 new", "+3 extra"]);
  expect(
    result.lines
      .filter((line) => !line.tone)
      .map((line) => line.text.trimEnd()),
  ).toEqual([" 1 same", " 4 end"]);
});

test("XR added and deleted files never invent an absent-side line", () => {
  expect(
    panel("", "界\n").lines.filter((line) => line.tone === "removed"),
  ).toEqual([]);
  expect(
    panel("界\n", "").lines.filter((line) => line.tone === "added"),
  ).toEqual([]);
  expect(
    panel("", "界\n")
      .lines.find((line) => line.tone === "added")
      ?.text.trimEnd(),
  ).toBe("+1 界");
});
