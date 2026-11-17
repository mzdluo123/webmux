import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vite-plus/test";
import {
  readFavoriteBranches,
  toggleFavoriteBranch,
} from "../src/domains/workspace/git-favorites";
import {
  buildGitRouteHref,
  parseGitRouteQuery,
} from "../src/hooks/shared/route";
import { setActiveHostId } from "../src/platform/storage";

const OID_HEAD = "1111111111111111111111111111111111111111";

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

describe("Git Log favorites and route query parameters", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", new MemoryStorage());
    setActiveHostId(null);
  });

  afterEach(() => {
    setActiveHostId(null);
    vi.unstubAllGlobals();
  });

  it("round-trips author, since, until, and path query parameters in route.ts", () => {
    const href = buildGitRouteHref({
      repositoryId: "repo-1",
      worktreeId: "wt-main",
      view: "log",
      scope: "branch",
      branch: "v1.0.0",
      commit: OID_HEAD,
      q: "fix",
      author: "alice",
      since: "2026-04-01",
      until: "2026-05-01",
      path: "internal/workspace",
    });
    const url = new URL(href, "http://127.0.0.1:7681");
    expect(parseGitRouteQuery(url.search)).toEqual({
      view: "log",
      scope: "branch",
      branch: "v1.0.0",
      commit: OID_HEAD,
      q: "fix",
      author: "alice",
      since: "2026-04-01",
      until: "2026-05-01",
      path: "internal/workspace",
      returnSession: undefined,
    });
  });

  it("persists favorite branches per repository and seeds defaultBranch", () => {
    const localSet = new Set(["main", "feat/log"]);
    expect(readFavoriteBranches("repo-1", "main", localSet)).toEqual(["main"]);

    const added = toggleFavoriteBranch("repo-1", "feat/log", ["main"]);
    expect(added).toEqual(["main", "feat/log"]);
    expect(readFavoriteBranches("repo-1", "main", localSet)).toEqual([
      "main",
      "feat/log",
    ]);

    const removed = toggleFavoriteBranch("repo-1", "main", added);
    expect(removed).toEqual(["feat/log"]);
    expect(readFavoriteBranches("repo-1", "main", localSet)).toEqual([
      "feat/log",
    ]);
  });
});
