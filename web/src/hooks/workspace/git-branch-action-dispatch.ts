import type {
  GitBranchItem,
  GitPrecedingCommit,
  GitWorkbenchMode,
} from "../../contracts/git-workbench-schemas";
import type {
  Repository,
  SessionInfo,
  Worktree,
} from "../../contracts/schemas";
import type { GitSharedBranchActionKind } from "../../domains/workspace/git-branches-model";
import type { Shell } from "../shared/shell.svelte";
import type { createGitWorkbenchController } from "./git-workbench-controller.svelte";

export function dispatchGitPageBranchAction(options: {
  kind: GitSharedBranchActionKind;
  item: GitBranchItem;
  repository: Repository | undefined;
  worktree: Worktree | undefined;
  writableWorktree: Worktree | undefined;
  sessions: readonly SessionInfo[];
  shell: Shell;
  controller: ReturnType<typeof createGitWorkbenchController>;
  switchWorktree: (
    nextWorktreeId?: string,
    forceMode?: GitWorkbenchMode,
  ) => void;
  openPush: (
    branchOverride?: string,
    preceding?: GitPrecedingCommit | null,
  ) => void;
}) {
  const {
    kind,
    item,
    repository,
    worktree,
    writableWorktree,
    sessions,
    shell,
    controller,
    switchWorktree,
    openPush,
  } = options;
  if (!repository) return;

  switch (kind) {
    case "filter-log":
      controller.setBranchFilter(
        item.name,
        repository,
        worktree,
        sessions,
        true,
      );
      break;
    case "clear-filter":
      controller.setBranchFilter(null, repository, worktree, sessions);
      break;
    case "toggle-favorite":
      controller.toggleFavorite(item.name);
      break;
    case "compare": {
      const base =
        controller.view.selectedCommitOid ??
        controller.view.context.headOid ??
        null;
      if (base && base !== item.head) {
        controller.setMode("log");
        void controller.startCompare(item.head, base);
      }
      break;
    }
    case "open-occupied-worktree":
      if (item.occupiedBy) switchWorktree(item.occupiedBy.worktreeId);
      break;
    case "checkout":
      if (writableWorktree) {
        shell.switchBranch(writableWorktree, item.name);
      } else {
        shell.start({
          kind: "new-worktree",
          repositoryId: repository.id,
          preset: { mode: "existing", branch: item.name },
        });
      }
      break;
    case "create-branch-from":
      if (writableWorktree) {
        shell.switchBranch(writableWorktree, {
          mode: "new",
          startPoint: item.name,
        });
      } else {
        shell.start({
          kind: "new-worktree",
          repositoryId: repository.id,
          preset: { mode: "new", startBranch: item.name },
        });
      }
      break;
    case "open-new-worktree":
      shell.start({
        kind: "new-worktree",
        repositoryId: repository.id,
        preset:
          item.occupiedBy || item.kind === "remote"
            ? { mode: "new", startBranch: item.name }
            : { mode: "existing", branch: item.name },
      });
      break;
    case "fetch":
      void controller.runFetch(worktree, sessions);
      break;
    case "fast-forward":
      shell.branchOperation(repository.id, {
        kind: "fast-forward",
        branch: item.name,
      });
      break;
    case "push":
      openPush(item.name);
      break;
    case "delete":
      shell.branchOperation(repository.id, {
        kind: "delete",
        branches: [item.name],
        cleanup: false,
      });
      break;
  }
}
