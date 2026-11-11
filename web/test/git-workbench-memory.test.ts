import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  applyGitQueryOverrides,
  clearGitWorkbenchMemoryCache,
  readGitDeviceLayout,
  readGitTargetMemory,
  resetGitDeviceLayout,
  resolveGitDeviceClass,
  writeGitDeviceLayout,
  writeGitTargetMemory,
} from "../src/domains/workspace/git-workbench-memory";
import { clearHostStorage, setActiveHostId } from "../src/platform/storage";

class MemoryStorage implements Storage {
  data = new Map<string, string>();
  get length() {
    return this.data.size;
  }
  clear() {
    this.data.clear();
  }
  getItem(key: string) {
    return this.data.get(key) ?? null;
  }
  key(index: number) {
    return [...this.data.keys()][index] ?? null;
  }
  removeItem(key: string) {
    this.data.delete(key);
  }
  setItem(key: string, value: string) {
    this.data.set(key, value);
  }
}

describe("git workbench state isolation and device-class layout memory", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", new MemoryStorage());
    setActiveHostId(null);
    clearGitWorkbenchMemoryCache();
  });

  afterEach(() => {
    setActiveHostId(null);
    vi.unstubAllGlobals();
  });

  test("classifies viewport widths into four device classes", () => {
    expect(resolveGitDeviceClass(1440)).toBe("desktop");
    expect(resolveGitDeviceClass(1280)).toBe("desktop");
    expect(resolveGitDeviceClass(1279)).toBe("compact");
    expect(resolveGitDeviceClass(1024)).toBe("compact");
    expect(resolveGitDeviceClass(1023)).toBe("tablet");
    expect(resolveGitDeviceClass(640)).toBe("tablet");
    expect(resolveGitDeviceClass(639)).toBe("phone");
    expect(resolveGitDeviceClass(390)).toBe("phone");
  });

  test("isolates splitter ratios and diff mode across phone and desktop", () => {
    writeGitDeviceLayout("desktop", {
      logThreePaneSizes: [18, 50, 32],
      diffMode: "split",
      showBranchesPanel: true,
    });
    writeGitDeviceLayout("phone", {
      diffMode: "unified",
      showBranchesPanel: false,
    });

    expect(readGitDeviceLayout("desktop")).toMatchObject({
      logThreePaneSizes: [18, 50, 32],
      diffMode: "split",
      showBranchesPanel: true,
    });
    expect(readGitDeviceLayout("phone")).toMatchObject({
      diffMode: "unified",
      showBranchesPanel: false,
    });

    // Switching active host keeps device-scoped layout preferences intact.
    setActiveHostId("host-alpha");
    expect(readGitDeviceLayout("desktop").diffMode).toBe("split");

    resetGitDeviceLayout("desktop");
    expect(readGitDeviceLayout("desktop").diffMode).toBe("unified");
  });

  test("isolates target memory by (hostId, repositoryId, worktreeId) and clears per host", () => {
    setActiveHostId("host-a");
    writeGitTargetMemory("repo-1", "wt-1", {
      mode: "commit",
      commitMessage: "feat: wt-1 draft",
      reviewedPaths: ["src/a.ts"],
      logDiffScrollTop: 180,
    });
    writeGitTargetMemory("repo-1", "wt-2", {
      mode: "log",
      commitMessage: "fix: wt-2 draft",
      branchFilter: "feat/two",
    });
    // Repo-only mode never stores a commit draft.
    writeGitTargetMemory("repo-1", undefined, {
      commitMessage: "should not persist without worktree",
      branchFilter: "main",
    });

    expect(readGitTargetMemory("repo-1", "wt-1")).toMatchObject({
      mode: "commit",
      commitMessage: "feat: wt-1 draft",
      reviewedPaths: ["src/a.ts"],
      logDiffScrollTop: 180,
    });
    expect(readGitTargetMemory("repo-1", "wt-2")).toMatchObject({
      mode: "log",
      commitMessage: "fix: wt-2 draft",
      branchFilter: "feat/two",
    });
    expect(readGitTargetMemory("repo-1", undefined).commitMessage).toBe("");

    setActiveHostId("host-b");
    expect(readGitTargetMemory("repo-1", "wt-1").commitMessage).toBe("");

    clearHostStorage("host-a");
    clearGitWorkbenchMemoryCache();
    setActiveHostId("host-a");
    expect(readGitTargetMemory("repo-1", "wt-1").commitMessage).toBe("");
  });

  test("prioritizes explicit URL deep-link query params over stored memory", () => {
    const stored = writeGitTargetMemory("repo-1", "wt-1", {
      mode: "branches",
      scope: "to-push",
      branchFilter: "old-branch",
      selectedCommitOid: "aaaa111",
      searchQuery: "old",
    });

    const resolved = applyGitQueryOverrides(stored, {
      view: "commit",
      scope: "branch",
      branch: "feat/new",
      commit: "bbbb222",
      q: "fresh",
    });

    expect(resolved).toMatchObject({
      mode: "commit",
      scope: "branch",
      branchFilter: "feat/new",
      selectedCommitOid: "bbbb222",
      searchQuery: "fresh",
    });
  });
});
