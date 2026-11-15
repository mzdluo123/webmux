import { api } from "../../platform/api";
import { RepositoriesSchema } from "../../contracts/schemas";
import { homeWorktrees } from "../../domains/workspace/home-worktrees.svelte";
import { setPinned } from "../../domains/workspace/home-worktrees";
import type { Repository, Worktree } from "../../contracts/schemas";

export function pinCreatedWorktree(
  repositories: readonly Repository[],
  tree: Worktree,
) {
  const repository = repositories.find(
    (entry) => entry.id === tree.repositoryId,
  );
  if (repository)
    homeWorktrees.save(
      setPinned(homeWorktrees.value, repository, [tree.id], true),
    );
}
export function loadRepositories(signal: AbortSignal) {
  return api("/repositories", RepositoriesSchema, "GET", undefined, signal);
}
export { createRepositories } from "../../hooks/workspace/repositories.svelte";
export { unknownWorkspaces } from "../../hooks/sessions/session-list";
export { homeWorktrees } from "../../domains/workspace/home-worktrees.svelte";
export { default as Tree } from "../../components/shell/WorkbenchTree.svelte";
export { default as Dialogs } from "../../components/shell/AppShellDialogs.svelte";
export const pages = {
  git: () => import("../../pages/GitWorkbench.svelte"),
  sessions: () => import("../../pages/WorkspaceSessionList.svelte"),
};
export const navigation = [
  { id: "workspace.home", label: "工作区", href: "/" },
];
