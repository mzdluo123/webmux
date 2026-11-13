export interface GitConflictHandoff {
  repositoryId: string;
  worktreeId: string;
  path?: string;
}

export const GIT_CONFLICT_HANDOFF_EVENT = "webmux:open-git-conflict";

let pendingHandoff: GitConflictHandoff | null = null;

export function requestConflictResolverHandoff(
  repositoryId: string,
  worktreeId: string,
  path?: string,
) {
  pendingHandoff = { repositoryId, worktreeId, path };
  if (typeof window !== "undefined") {
    window.dispatchEvent(
      new CustomEvent<GitConflictHandoff>(GIT_CONFLICT_HANDOFF_EVENT, {
        detail: pendingHandoff,
      }),
    );
  }
}

export function consumeConflictResolverHandoff(
  repositoryId: string,
  worktreeId: string | undefined,
): { matched: boolean; path?: string } {
  if (!pendingHandoff || pendingHandoff.repositoryId !== repositoryId) {
    return { matched: false };
  }
  if (worktreeId && pendingHandoff.worktreeId !== worktreeId) {
    return { matched: false };
  }
  const path = pendingHandoff.path;
  pendingHandoff = null;
  return { matched: true, path };
}
