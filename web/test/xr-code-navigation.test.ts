import { expect, test } from "vitest";
import type { CodeListing } from "../src/contracts/schemas";
import { xrTreePanel } from "../src/domains/code/xr-code-navigation";

test("XR file navigation disables links, boundaries and special nodes", () => {
  const kinds = [
    "file",
    "directory",
    "symlink",
    "boundary",
    "special",
  ] as const;
  const listing: CodeListing = {
    children: kinds.map((kind) => ({
      nodeId: kind,
      path: kind,
      name: kind,
      kind,
      gitStatus: "",
      ignored: false,
      referenceable: kind === "file",
      mediaType: "",
    })),
    nextCursor: "",
    limited: false,
    complete: true,
    contentVersion: "v1",
    checkedAt: "",
  };
  const loading = new Set<string>();
  const options = {
    listing,
    expanded: new Set<string>(),
    branches: new Map<string, CodeListing>(),
    loading,
    errors: new Map<string, string>(),
    toggle() {},
    open() {},
    more() {},
  };
  expect(
    xrTreePanel(options)
      .actions!.filter((action) => !action.disabled)
      .map((action) => action.id),
  ).toEqual(["file", "directory"]);
});
