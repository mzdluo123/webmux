import { describe, expect, it } from "vitest";
import {
  dockActivate,
  dockDropEdge,
  dockGeometry,
  dockGroup,
  dockMove,
  dockRemove,
  dockResize,
  dockSplit,
  restoreDock,
} from "../src/lib/dock-layout";

const available = new Set(["chat", "files", "diff", "tests"]);
const geometry = (
  node: Parameters<typeof dockGeometry>[0],
  hidden: string[] = [],
) => dockGeometry(node, available, new Set(hidden));

const makeTree = () =>
  dockSplit(
    "horizontal",
    dockGroup("chat"),
    dockSplit("vertical", dockGroup("files", "diff"), dockGroup("tests"), 0.65),
    0.4,
  );

describe("dock layout", () => {
  it("releases a minimized branch without losing its nested size or active tab", () => {
    const tree = dockActivate(makeTree(), "diff");
    const before = geometry(tree);
    const minimized = geometry(tree, ["tests"]);
    expect(minimized.groups.get("diff")?.rect).toEqual({
      x: 0.4,
      y: 0,
      width: 0.6,
      height: 1,
    });
    expect(minimized.dividers.map((divider) => divider.direction)).toEqual([
      "horizontal",
    ]);
    expect(geometry(tree)).toEqual(before);
    expect(before.groups.get("diff")?.rect.height).toBe(0.65);
  });

  it("moves an active tab out of its group then merges it without copying the instance", () => {
    const original = dockActivate(makeTree(), "diff");
    const split = dockMove(original, "diff", "tests", "right");
    expect(geometry(split).groups.get("tests")?.rect.width).toBeCloseTo(0.3);
    expect(geometry(split).groups.get("files")?.ids).toEqual(["files"]);

    const merged = dockMove(split, "diff", "chat", "center");
    const result = geometry(merged).groups;
    expect(result.has("chat")).toBe(false);
    expect(result.get("diff")?.ids).toEqual(["chat", "diff"]);
    expect(result.get("tests")?.rect.width).toBeCloseTo(0.6);
  });

  it("resizes the remaining split at its original path when its sibling is hidden", () => {
    const tree = makeTree();
    const [visible] = geometry(tree, ["chat"]).dividers;
    expect(visible?.path).toBe("1");
    const result = dockResize(tree, visible!.path, 0.35);
    expect(geometry(result).groups.get("files")?.rect.height).toBe(0.35);
    expect(geometry(result).groups.get("chat")?.rect.width).toBe(0.4);
  });

  it("restores only valid unique layouts and ignores minimized IDs removed from the tree", () => {
    const node = dockRemove(makeTree(), "tests");
    const saved = JSON.stringify({
      version: 1,
      node,
      hidden: ["tests", "files"],
    });
    expect(restoreDock(saved)).toEqual({ node, hidden: ["files"] });
    expect(
      restoreDock(
        JSON.stringify({
          version: 1,
          node: dockGroup("chat", "chat"),
          hidden: [],
        }),
      ),
    ).toBeUndefined();
    expect(restoreDock("{broken")).toBeUndefined();
  });

  it("keeps the center for tabs and routes each edge to the matching split", () => {
    expect(dockDropEdge(0.5, 0.5)).toBe("center");
    expect(dockDropEdge(0.9, 0.5)).toBe("right");
    expect(dockDropEdge(0.1, 0.5)).toBe("left");
    expect(dockDropEdge(0.5, 0.1)).toBe("top");
    expect(dockDropEdge(0.5, 0.9)).toBe("bottom");
  });
});
