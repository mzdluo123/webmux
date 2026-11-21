import { expect, test } from "vitest";
import { dockGroup, dockSplit, type DockNode } from "../src/lib/dock-layout";
import { migrateSessionLayout } from "../src/domains/terminal/session-layout-migration";

test("migration prefers the visible active side and preserves unrelated splits", () => {
  const other = dockSplit(
    "horizontal",
    dockGroup("b"),
    dockGroup("reader:b"),
    0.4,
  );
  const node = dockSplit(
    "vertical",
    dockGroup("a"),
    dockSplit("horizontal", dockGroup("chat:a"), other, 0.6),
  );
  const restore = (hidden: string[]) =>
    migrateSessionLayout(
      JSON.stringify({ version: 1, node, hidden }),
      ["a", "b"],
      (id) => id === "a",
    );
  expect(restore([])?.node).toEqual(
    dockSplit("horizontal", dockGroup("a"), other, 0.6),
  );
  expect(restore(["chat:a"])?.node).toEqual(
    dockSplit("vertical", dockGroup("a"), other),
  );
  expect(restore(["chat:a", "a"])?.hidden).toEqual(["a"]);
  expect(migrateSessionLayout("invalid", ["a"], () => false)).toBeUndefined();
});

test("migration drops retired sessions and is stable when the result is restored again", () => {
  const live = dockSplit(
    "vertical",
    dockGroup("chat:a", "a"),
    dockGroup("files:a"),
    0.3,
  );
  const node = dockSplit(
    "horizontal",
    dockGroup("retired", "chat:retired"),
    live,
  );
  const restore = (layout: { node: DockNode; hidden: string[] }) =>
    migrateSessionLayout(
      JSON.stringify({ version: 1, ...layout }),
      ["a"],
      () => true,
    );
  const first = restore({ node, hidden: ["retired", "files:a"] });
  expect(first).toEqual({
    node: dockSplit("vertical", dockGroup("a"), dockGroup("files:a"), 0.3),
    hidden: ["files:a"],
  });
  expect(restore(first!)).toEqual(first);
});
