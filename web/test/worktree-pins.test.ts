import { describe, expect, test, vi } from "vite-plus/test";
import {
  MAX_WORKTREE_PINS_PER_REPO,
  type Repository,
  type Worktree,
} from "../src/contracts/schemas";
import {
  buildWorktreePinMenuProps,
  canMoveWorktreePin,
  isRepoWorktreePinsFull,
  isWorktreePinned,
  moveWorktreePin,
  pinnedWorktreeIds,
  pruneWorktreePins,
  removeRepositoryWorktreePins,
  removeWorktreePins,
  sortRepositoryWorktrees,
  toggleWorktreePin,
} from "../src/domains/workspace/worktree-pins";

const makeTree = (
  id: string,
  kind: Worktree["kind"] = "linked",
  repositoryId = "r1",
): Worktree => ({
  id,
  repositoryId,
  path: kind === "main" ? "/work/main" : `/work/wt-${id}`,
  kind,
  branch: kind === "main" ? "main" : `wt/${id}`,
  head: "a".repeat(40),
  detached: false,
  locked: false,
  lockReason: "",
  prunable: false,
  available: true,
  dirty: false,
  hasUntracked: false,
  hasIgnored: false,
  error: "",
  upstream: null,
  ahead: null,
  behind: null,
  upstreamGone: false,
  lastCommit: null,
  changes: { staged: 0, unstaged: 0, untracked: 0, conflicted: 0 },
  operation: null,
});

const makeRepo = (
  worktrees: Worktree[],
  overrides: Partial<Repository> = {},
): Repository => ({
  id: "r1",
  name: "webmux",
  commonDir: "/work/main/.git",
  anchorPath: "/work/main",
  availability: "available",
  error: "",
  checkedAt: "2026-11-19T00:00:00Z",
  lastFetchedAt: null,
  initInstruction: "",
  worktrees,
  ...overrides,
});

describe("worktreePins", () => {
  const main = makeTree("main", "main");
  const wtA = makeTree("01");
  const wtB = makeTree("02");
  const wtC = makeTree("03");

  test("sorts pinned worktrees ahead of main in saved order, then main, then by path", () => {
    const trees = [wtC, main, wtA, wtB];
    expect(sortRepositoryWorktrees(trees, {}).map((t) => t.id)).toEqual([
      "main",
      "01",
      "02",
      "03",
    ]);
    expect(
      sortRepositoryWorktrees(trees, { r1: ["03", "01"] }).map((t) => t.id),
    ).toEqual(["03", "01", "main", "02"]);
    expect(
      sortRepositoryWorktrees(trees, { r1: ["02", "main", "03"] }).map(
        (t) => t.id,
      ),
    ).toEqual(["02", "main", "03", "01"]);
  });

  test("toggles pins newest-first, cleans empty keys, and enforces per-repo limit after pruning", () => {
    const repo = makeRepo([main, wtA, wtB]);
    const first = toggleWorktreePin({}, repo, "02");
    expect(first).toEqual({ pins: { r1: ["02"] }, pinned: true });
    expect(isWorktreePinned(first.pins, "r1", "02")).toBe(true);

    const second = toggleWorktreePin(first.pins, repo, "01");
    expect(second).toEqual({ pins: { r1: ["01", "02"] }, pinned: true });

    const unpinnedFirst = toggleWorktreePin(second.pins, repo, "01");
    expect(unpinnedFirst).toEqual({ pins: { r1: ["02"] }, pinned: false });

    const cleared = toggleWorktreePin(unpinnedFirst.pins, repo, "02");
    expect(cleared).toEqual({ pins: {}, pinned: false });

    const manyTrees = Array.from(
      { length: MAX_WORKTREE_PINS_PER_REPO + 1 },
      (_, i) => makeTree(String(i).padStart(2, "0")),
    );
    const fullRepo = makeRepo(manyTrees);
    const fullPins = {
      r1: manyTrees.slice(0, MAX_WORKTREE_PINS_PER_REPO).map((t) => t.id),
    };
    expect(isRepoWorktreePinsFull(fullPins, fullRepo)).toBe(true);
    expect(toggleWorktreePin(fullPins, fullRepo, manyTrees.at(-1)!.id)).toEqual(
      {
        pins: fullPins,
        pinned: false,
      },
    );

    // Stale deleted IDs in a healthy snapshot do not block pinning a live worktree.
    const withStale = {
      r1: [
        "stale-id",
        ...manyTrees.slice(0, MAX_WORKTREE_PINS_PER_REPO - 1).map((t) => t.id),
      ],
    };
    expect(isRepoWorktreePinsFull(withStale, fullRepo)).toBe(false);
    const addedAfterPrune = toggleWorktreePin(
      withStale,
      fullRepo,
      manyTrees.at(-1)!.id,
    );
    expect(addedAfterPrune.pinned).toBe(true);
    expect(addedAfterPrune.pins.r1?.[0]).toBe(manyTrees.at(-1)!.id);
    expect(addedAfterPrune.pins.r1).not.toContain("stale-id");
  });

  test("moves pinned worktrees up and down within candidate lists", () => {
    const repo = makeRepo([main, wtA, wtB, wtC]);
    const pins = { r1: ["01", "02", "03"] };
    expect(canMoveWorktreePin(pins, repo, "01", -1)).toBe(false);
    expect(canMoveWorktreePin(pins, repo, "01", 1)).toBe(true);
    expect(canMoveWorktreePin(pins, repo, "03", 1)).toBe(false);

    const movedDown = moveWorktreePin(pins, repo, "01", 1);
    expect(movedDown.r1).toEqual(["02", "01", "03"]);

    // With candidates excluding "01", moving "03" up swaps directly with "02".
    const movedOverHidden = moveWorktreePin(movedDown, repo, "03", -1, [
      wtB,
      wtC,
    ]);
    expect(movedOverHidden.r1).toEqual(["03", "01", "02"]);
  });

  test("prunes missing repositories and deleted worktrees while keeping unhealthy snapshots", () => {
    const pins = { r1: ["01", "gone"], r2: ["other"] };
    expect(pruneWorktreePins(pins, [makeRepo([main, wtA])])).toEqual({
      r1: ["01"],
    });

    const unhealthy = makeRepo([], { availability: "unavailable" });
    const kept = { r1: ["01", "gone"] };
    expect(pruneWorktreePins(kept, [unhealthy])).toBe(kept);

    expect(removeWorktreePins({ r1: ["01", "02"] }, "r1", ["01"])).toEqual({
      r1: ["02"],
    });
    expect(removeWorktreePins({ r1: ["02"] }, "r1", ["02"])).toEqual({});
    expect(
      removeRepositoryWorktreePins({ r1: ["01"], r2: ["x"] }, "r1"),
    ).toEqual({
      r2: ["x"],
    });
    expect(pinnedWorktreeIds({ r1: ["02", "01"] }, "r1", [wtA])).toEqual([
      "01",
    ]);
  });

  test("builds menu action props with boundary disable states", () => {
    const repo = makeRepo([main, wtA, wtB]);
    const ontoggle = vi.fn();
    const onmove = vi.fn();
    const firstProps = buildWorktreePinMenuProps({
      pins: { r1: ["01", "02"] },
      repo,
      worktreeId: "01",
      ontoggle,
      onmove,
    });
    expect(firstProps.topPinned).toBe(true);
    expect(firstProps.canMoveTopPinUp).toBe(false);
    expect(firstProps.canMoveTopPinDown).toBe(true);
    firstProps.onmovetoppindown?.();
    expect(onmove).toHaveBeenCalledWith("r1", "01", 1);
    firstProps.ontoggletoppin?.();
    expect(ontoggle).toHaveBeenCalledWith("r1", "01");
  });
});
