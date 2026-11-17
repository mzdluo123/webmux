import { api } from "./api";
import { withGitQuery } from "./git-api";
import {
  GitBlameResultSchema,
  GitBranchMutationResultSchema,
  GitBranchRenamePreviewRequestSchema,
  GitBranchRenamePreviewSchema,
  GitBranchRenameRequestSchema,
  GitBranchUpstreamRequestSchema,
  GitDiffSchema,
  GitFileHistoryPageSchema,
  GitRemoteAddRequestSchema,
  GitRemoteMutationResultSchema,
  GitRemoteRemoveRequestSchema,
  GitRemoteUpdateRequestSchema,
  GitTagCreateRequestSchema,
  GitTagCreateResultSchema,
  GitTagDeleteRequestSchema,
  GitTagDeleteResultSchema,
  type GitBranchRenamePreviewRequest,
  type GitBranchRenameRequest,
  type GitBranchUpstreamRequest,
  type GitRemoteAddRequest,
  type GitRemoteRemoveRequest,
  type GitRemoteUpdateRequest,
  type GitTagCreateRequest,
  type GitTagDeleteRequest,
} from "../contracts/schemas";

export interface GitFileHistoryParams {
  path: string;
  worktreeId?: string;
  rev?: string;
  limit?: number;
  cursor?: string;
}

export interface GitBlameParams {
  path: string;
  worktreeId?: string;
  rev?: string;
}

export interface GitRevisionDiffParams {
  worktreeId: string;
  contextId: string;
  baseOid: string;
  path: string;
}

function repoBase(repositoryId: string): string {
  return `/repositories/${encodeURIComponent(repositoryId)}`;
}

export function gitBranchRenamePreview(
  repositoryId: string,
  request: GitBranchRenamePreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoBase(repositoryId)}/branches/rename-preview`,
    GitBranchRenamePreviewSchema,
    "POST",
    request,
    signal,
    GitBranchRenamePreviewRequestSchema,
  );
}

export function gitBranchRename(
  repositoryId: string,
  request: GitBranchRenameRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoBase(repositoryId)}/branches/rename`,
    GitBranchMutationResultSchema,
    "POST",
    request,
    signal,
    GitBranchRenameRequestSchema,
  );
}

export function gitBranchSetUpstream(
  repositoryId: string,
  request: GitBranchUpstreamRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoBase(repositoryId)}/branches/upstream`,
    GitBranchMutationResultSchema,
    "POST",
    request,
    signal,
    GitBranchUpstreamRequestSchema,
  );
}

export function gitTagCreate(
  repositoryId: string,
  request: GitTagCreateRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoBase(repositoryId)}/tags`,
    GitTagCreateResultSchema,
    "POST",
    request,
    signal,
    GitTagCreateRequestSchema,
  );
}

export function gitTagDelete(
  repositoryId: string,
  request: GitTagDeleteRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoBase(repositoryId)}/tags/delete`,
    GitTagDeleteResultSchema,
    "POST",
    request,
    signal,
    GitTagDeleteRequestSchema,
  );
}

export function gitRemoteAdd(
  repositoryId: string,
  request: GitRemoteAddRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoBase(repositoryId)}/remotes`,
    GitRemoteMutationResultSchema,
    "POST",
    request,
    signal,
    GitRemoteAddRequestSchema,
  );
}

export function gitRemoteUpdate(
  repositoryId: string,
  name: string,
  request: GitRemoteUpdateRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoBase(repositoryId)}/remotes/${encodeURIComponent(name)}`,
    GitRemoteMutationResultSchema,
    "PATCH",
    request,
    signal,
    GitRemoteUpdateRequestSchema,
  );
}

export function gitRemoteRemove(
  repositoryId: string,
  name: string,
  request: GitRemoteRemoveRequest = {},
  signal?: AbortSignal,
) {
  return api(
    `${repoBase(repositoryId)}/remotes/${encodeURIComponent(name)}`,
    GitRemoteMutationResultSchema,
    "DELETE",
    request,
    signal,
    GitRemoteRemoveRequestSchema,
  );
}

export function gitFileHistory(
  repositoryId: string,
  params: GitFileHistoryParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoBase(repositoryId)}/git/file-history`, {
      worktreeId: params.worktreeId,
      rev: params.rev,
      path: params.path,
      limit: params.limit,
      cursor: params.cursor,
    }),
    GitFileHistoryPageSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitBlame(
  repositoryId: string,
  params: GitBlameParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoBase(repositoryId)}/git/blame`, {
      worktreeId: params.worktreeId,
      rev: params.rev,
      path: params.path,
    }),
    GitBlameResultSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitRevisionDiff(
  repositoryId: string,
  params: GitRevisionDiffParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoBase(repositoryId)}/git/revision-diff`, {
      worktreeId: params.worktreeId,
      contextId: params.contextId,
      baseOid: params.baseOid,
      path: params.path,
    }),
    GitDiffSchema,
    "GET",
    undefined,
    signal,
  );
}
