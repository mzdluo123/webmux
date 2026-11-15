import type {
  CreateSessionRequest,
  LaunchConfig,
  LaunchConfigSummary,
  SessionInfo,
} from "../../contracts/schemas";

export type ChosenLaunchConfig = LaunchConfig | LaunchConfigSummary;

export type SessionBuildFields = {
  kind: "directory" | "worktree" | "new-worktree";
  repositoryId: string;
  worktreeId: string;
  branchMode: "new" | "existing";
  branch: string;
  base: string;
  newPath: string;
  cwd: string;
  entryDirectory: { cwd: string; workspace: SessionInfo["workspace"] } | null;
  prompt: string;
  hasInstruction: boolean;
  useInstruction: boolean;
  chosenConfig: ChosenLaunchConfig | undefined;
  name: string;
  startSide: "terminal" | "chat";
};

export function buildCreateSessionRequest(
  input: SessionBuildFields & { cmd: string[] | undefined },
): CreateSessionRequest {
  const workspace =
    input.entryDirectory?.workspace && input.cwd === input.entryDirectory.cwd
      ? input.entryDirectory.workspace
      : null;
  const target =
    input.kind === "new-worktree"
      ? {
          newWorktree: {
            repositoryId: input.repositoryId,
            mode: input.branchMode,
            branch: input.branch,
            ...(input.branchMode === "new" ? { base: input.base } : {}),
            ...(input.newPath ? { path: input.newPath } : {}),
          },
        }
      : input.kind === "directory"
        ? {
            cwd: input.cwd,
            ...(workspace
              ? {
                  workspace: {
                    repositoryId: workspace.repositoryId,
                    worktreeId: workspace.worktreeId,
                  },
                }
              : {}),
          }
        : {
            workspace: {
              repositoryId: input.repositoryId,
              worktreeId: input.worktreeId,
            },
          };
  const first = {
    ...(input.prompt.trim() ? { prompt: input.prompt.trim() } : {}),
    ...(input.hasInstruction && input.useInstruction
      ? { useInitInstruction: true }
      : {}),
  };
  return input.chosenConfig
    ? {
        ...target,
        configId: input.chosenConfig.id,
        name: input.name,
        ...(input.chosenConfig.chat ? { view: input.startSide } : {}),
        ...first,
      }
    : { ...target, cmd: input.cmd, name: input.name, ...first };
}
