import { api } from "./api";
import {
  EmptyRequestSchema,
  GitAbortOperationPreviewSchema,
  GitAbortOperationRequestSchema,
  GitBranchDivergencePreviewRequestSchema,
  GitCommitDivergencePreviewRequestSchema,
  GitConflictFileDetailSchema,
  GitContinueOperationRequestSchema,
  GitDivergenceExecuteRequestSchema,
  GitDivergencePreviewSchema,
  GitDivergenceResultSchema,
  GitInProgressOperationSchema,
  GitResolveConflictRequestSchema,
  GitResolveConflictResponseSchema,
  GitSaveConflictRequestSchema,
  GitSaveConflictResponseSchema,
  type GitAbortOperationRequest,
  type GitBranchDivergencePreviewRequest,
  type GitCommitDivergencePreviewRequest,
  type GitContinueOperationRequest,
  type GitDivergenceExecuteRequest,
  type GitDivergenceKind,
  type GitResolveConflictRequest,
  type GitSaveConflictRequest,
} from "../contracts/schemas";

function worktreeConflictBase(
  repositoryId: string,
  worktreeId: string,
): string {
  return `/repositories/${encodeURIComponent(repositoryId)}/worktrees/${encodeURIComponent(worktreeId)}/git`;
}

export function gitOperationState(
  repositoryId: string,
  worktreeId: string,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/operation-state`,
    GitInProgressOperationSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitConflictDetail(
  repositoryId: string,
  worktreeId: string,
  path: string,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/conflict?path=${encodeURIComponent(path)}`,
    GitConflictFileDetailSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitSaveConflict(
  repositoryId: string,
  worktreeId: string,
  request: GitSaveConflictRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/conflict/save`,
    GitSaveConflictResponseSchema,
    "POST",
    request,
    signal,
    GitSaveConflictRequestSchema,
  );
}

export function gitResolveConflict(
  repositoryId: string,
  worktreeId: string,
  request: GitResolveConflictRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/conflict/resolve`,
    GitResolveConflictResponseSchema,
    "POST",
    request,
    signal,
    GitResolveConflictRequestSchema,
  );
}

export function gitMergePreview(
  repositoryId: string,
  worktreeId: string,
  request: GitBranchDivergencePreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/merge-preview`,
    GitDivergencePreviewSchema,
    "POST",
    request,
    signal,
    GitBranchDivergencePreviewRequestSchema,
  );
}

export function gitRebasePreview(
  repositoryId: string,
  worktreeId: string,
  request: GitBranchDivergencePreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/rebase-preview`,
    GitDivergencePreviewSchema,
    "POST",
    request,
    signal,
    GitBranchDivergencePreviewRequestSchema,
  );
}

export function gitCherryPickPreview(
  repositoryId: string,
  worktreeId: string,
  request: GitCommitDivergencePreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/cherry-pick-preview`,
    GitDivergencePreviewSchema,
    "POST",
    request,
    signal,
    GitCommitDivergencePreviewRequestSchema,
  );
}

export function gitRevertPreview(
  repositoryId: string,
  worktreeId: string,
  request: GitCommitDivergencePreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/revert-preview`,
    GitDivergencePreviewSchema,
    "POST",
    request,
    signal,
    GitCommitDivergencePreviewRequestSchema,
  );
}

const divergenceExecutePath: Record<GitDivergenceKind, string> = {
  merge: "merge",
  rebase: "rebase",
  cherry_pick: "cherry-pick",
  revert: "revert",
};

export function gitExecuteDivergence(
  repositoryId: string,
  worktreeId: string,
  kind: GitDivergenceKind,
  request: GitDivergenceExecuteRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/${divergenceExecutePath[kind]}`,
    GitDivergenceResultSchema,
    "POST",
    request,
    signal,
    GitDivergenceExecuteRequestSchema,
  );
}

export function gitContinueOperation(
  repositoryId: string,
  worktreeId: string,
  request: GitContinueOperationRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/operation-continue`,
    GitDivergenceResultSchema,
    "POST",
    request,
    signal,
    GitContinueOperationRequestSchema,
  );
}

export function gitAbortOperationPreview(
  repositoryId: string,
  worktreeId: string,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/operation-abort-preview`,
    GitAbortOperationPreviewSchema,
    "POST",
    {},
    signal,
    EmptyRequestSchema,
  );
}

export function gitAbortOperation(
  repositoryId: string,
  worktreeId: string,
  request: GitAbortOperationRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeConflictBase(repositoryId, worktreeId)}/operation-abort`,
    GitDivergenceResultSchema,
    "POST",
    request,
    signal,
    GitAbortOperationRequestSchema,
  );
}
