import { expect, test } from "vitest";
import { dockGroup, dockSplit } from "../src/lib/dock-layout";
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
