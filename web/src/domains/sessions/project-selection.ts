import type { Project, Repository } from "../../contracts/schemas";

// A project may have just been registered and not yet reached the host's
// repository list. Carry its validated snapshot through session setup.
export interface ProjectSelection {
  project: Project;
  repository?: Repository;
  openedPath?: string;
}

// Context entries and catalog rows can use different ids for the same folder.
export function sameProject(
  project: Project,
  current: Project | undefined,
): boolean {
  return (
    !!current &&
    project.repositoryId === current.repositoryId &&
    project.path === current.path
  );
}

export function projectTarget(selection: ProjectSelection) {
  const repository = selection.repository;
  const trees = repository?.worktrees ?? [];
  const tree =
    [...trees]
      .filter(
        (item) =>
          item.available &&
          item.kind !== "bare" &&
          selection.openedPath &&
          (selection.openedPath === item.path ||
            selection.openedPath.startsWith(
              item.path.replace(/\/+$/, "") + "/",
            )),
      )
      .sort((a, b) => b.path.length - a.path.length)[0] ??
    trees.find((item) => item.kind === "main" && item.available) ??
    trees.find((item) => item.available && item.kind !== "bare");
  return {
    kind: selection.project.repositoryId
      ? ("worktree" as const)
      : ("directory" as const),
    cwd: selection.project.path,
    repositoryId: selection.project.repositoryId,
    worktreeId: tree?.id ?? "",
  };
}

export function projectRepositories(
  host: readonly Repository[],
  selection: ProjectSelection | null,
): readonly Repository[] {
  const added = selection?.repository;
  return added && !host.some((repository) => repository.id === added.id)
    ? [...host, added]
    : host;
}

export function contextProject(
  repository: Repository | undefined,
  cwd?: string,
): Project | undefined {
  if (repository)
    return {
      id: "repository:" + repository.id,
      name: repository.name,
      path: repository.anchorPath,
      repositoryId: repository.id,
      availability: repository.availability,
      error: repository.error,
    };
  if (cwd)
    return {
      id: "context:" + cwd,
      name: cwd.split("/").pop() || cwd,
      path: cwd,
      repositoryId: "",
      availability: "available",
      error: "",
    };
  return undefined;
}

export function scopeForProject(
  scope: { repositoryId: string; worktreeId?: string } | undefined,
  project: Project | undefined,
) {
  return (
    scope ??
    (project?.repositoryId ? { repositoryId: project.repositoryId } : undefined)
  );
}
