import { suggestProjectClonePath } from "../src/domains/workspace/project-clone-path";
import { describe, expect, test } from "vite-plus/test";
import {
  contextProject,
  projectRepositories,
  projectTarget,
  scopeForProject,
} from "../src/domains/sessions/project-selection";
import {
  resolveFallbackAfterWorktreeRemoved,
  resolveInitialTarget,
} from "../src/domains/sessions/new-session-defaults";
import type { Project } from "../src/contracts/schemas";
import { repository, worktree } from "./browser/shell-fixture";

const project: Project = {
  id: "repo1",
  name: "webmux",
  path: "/repos/webmux",
  repositoryId: "repo1",
  availability: "available",
  error: "",
};

describe("project session boundaries", () => {
  test("a Git project with no selectable tree stays unset rather than using an unrelated cwd", () => {
    const selection = {
      project,
      repository: repository({
        worktrees: [
          worktree({ kind: "bare" }),
          worktree({ id: "missing", available: false }),
        ],
      }),
    };
    expect(projectTarget(selection)).toEqual({
      kind: "worktree",
      cwd: project.path,
      repositoryId: "repo1",
      worktreeId: "",
    });
  });
  test("fallback uses an available linked tree when the main tree is unavailable", () => {
    const result = projectTarget({
      project,
      repository: repository({
        worktrees: [
          worktree({ available: false }),
          worktree({ id: "linked", kind: "linked" }),
        ],
      }),
    });
    expect(result.worktreeId).toBe("linked");
  });
  test("opening a linked worktree subfolder selects that tree rather than main", () => {
    const repo = repository({
      worktrees: [
        worktree({ path: "/repos/webmux" }),
        worktree({
          id: "linked",
          kind: "linked",
          path: "/repos/webmux-linked",
        }),
      ],
    });
    expect(
      projectTarget({
        project,
        repository: repo,
        openedPath: "/repos/webmux-linked/src",
      }).worktreeId,
    ).toBe("linked");
    expect(
      projectTarget({
        project,
        repository: repo,
        openedPath: "/repos/webmux-linked-other/src",
      }).worktreeId,
    ).toBe("main");
  });
  test("new repository snapshots bridge host refresh and then yield to the host's newer snapshot", () => {
    const captured = repository();
    expect(projectRepositories([], { project, repository: captured })).toEqual([
      captured,
    ]);
    const refreshed = { ...captured, name: "updated name" };
    expect(
      projectRepositories([refreshed], { project, repository: captured }),
    ).toEqual([refreshed]);
  });
  test("explicit worktree scope stays narrow while ordinary project scope excludes other repositories", () => {
    const scope = { repositoryId: "repo1", worktreeId: "main" };
    expect(scopeForProject(scope, project)).toBe(scope);
    expect(scopeForProject(undefined, project)).toEqual({
      repositoryId: "repo1",
    });
    expect(
      scopeForProject(undefined, { ...project, repositoryId: "" }),
    ).toBeUndefined();
  });
  test("removing the scoped tree cannot silently fall back to a different cwd or worktree", () => {
    const removed = worktree({ id: "removed", kind: "linked" });
    expect(
      resolveFallbackAfterWorktreeRemoved(
        [repository()],
        "worktree",
        removed.id,
        removed,
        { repositoryId: "repo1", worktreeId: removed.id },
      ),
    ).toEqual({ kind: "worktree", worktreeId: "" });
    expect(
      resolveFallbackAfterWorktreeRemoved(
        [repository({ worktrees: [] })],
        "worktree",
        removed.id,
        removed,
        { repositoryId: "repo1" },
      ),
    ).toEqual({ kind: "worktree", worktreeId: "" });
  });
  test("an explicit directory context keeps its actual pane directory", () => {
    expect(contextProject(undefined, "/projects/notes/subdir")).toMatchObject({
      path: "/projects/notes/subdir",
      repositoryId: "",
    });
  });
});

test("core-only creation ignores repository identity but keeps an explicit pane cwd", () => {
  expect(
    resolveInitialTarget({
      workspaceAvailable: false,
      entry: {
        kind: "directory",
        cwd: "/work/subdir",
        workspace: {
          repositoryId: "repo1",
          worktreeId: "main",
          worktreePath: "/work",
        },
      },
      scope: { repositoryId: "repo1" },
      selectedRepositoryId: "repo1",
      repositories: [repository()],
      recent: ["/other"],
      defaultCwd: "/work",
    }),
  ).toEqual({
    kind: "directory",
    cwd: "/work/subdir",
    repositoryId: "",
    worktreeId: "",
  });
});

test.each([
  "https://github.com/demo/repo.git",
  "ssh://git@example.invalid/demo/repo.git",
  "git@example.invalid:demo/repo.git",
])("clone suggestion takes a repository name from %s", (remote) => {
  expect(suggestProjectClonePath("~/projects/", remote)).toBe(
    "~/projects/repo",
  );
});
test.each([
  "git@host:..",
  "https://host/repo?token=secret",
  "https://host/repo%2fchild",
  "git@host:repo\nother",
])("ambiguous clone names remain explicit: %s", (remote) => {
  expect(suggestProjectClonePath("/projects", remote)).toBe("");
});
