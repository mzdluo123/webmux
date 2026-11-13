import { api } from "./api";
import { repoGitBase, withGitQuery, worktreeGitBase } from "./git-api";
import {
  GitDiffSchema,
  GitStashApplyPreviewRequestSchema,
  GitStashApplyPreviewSchema,
  GitStashApplyResultSchema,
  GitStashCreatePreviewRequestSchema,
  GitStashCreatePreviewSchema,
  GitStashCreateResultSchema,
  GitStashDetailSchema,
  GitStashDropPreviewRequestSchema,
  GitStashDropPreviewSchema,
  GitStashDropRequestSchema,
  GitStashDropResultSchema,
  GitStashExecuteRequestSchema,
  GitStashListSchema,
  type GitStashApplyPreviewRequest,
  type GitStashCreatePreviewRequest,
  type GitStashDropPreviewRequest,
  type GitStashDropRequest,
  type GitStashExecuteRequest,
} from "../contracts/schemas";

export interface GitStashDiffParams {
  contextId: string;
  path: string;
  untracked?: boolean;
  worktreeId?: string;
}

export function gitStashes(repositoryId: string, signal?: AbortSignal) {
  return api(
    `${repoGitBase(repositoryId)}/stashes`,
    GitStashListSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitStashDetail(
  repositoryId: string,
  stashOid: string,
  signal?: AbortSignal,
) {
  return api(
    `${repoGitBase(repositoryId)}/stashes/${encodeURIComponent(stashOid)}`,
    GitStashDetailSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitStashDiff(
  repositoryId: string,
  stashOid: string,
  params: GitStashDiffParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(
      `${repoGitBase(repositoryId)}/stashes/${encodeURIComponent(stashOid)}/diff`,
      {
        contextId: params.contextId,
        path: params.path,
        untracked: params.untracked ? "true" : undefined,
        worktreeId: params.worktreeId,
      },
    ),
    GitDiffSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitStashCreatePreview(
  repositoryId: string,
  worktreeId: string,
  request: GitStashCreatePreviewRequest = {},
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/stash-preview`,
    GitStashCreatePreviewSchema,
    "POST",
    request,
    signal,
    GitStashCreatePreviewRequestSchema,
  );
}

export function gitStashCreate(
  repositoryId: string,
  worktreeId: string,
  request: GitStashExecuteRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/stash`,
    GitStashCreateResultSchema,
    "POST",
    request,
    signal,
    GitStashExecuteRequestSchema,
  );
}

export function gitStashApplyPreview(
  repositoryId: string,
  worktreeId: string,
  request: GitStashApplyPreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/stash-apply-preview`,
    GitStashApplyPreviewSchema,
    "POST",
    request,
    signal,
    GitStashApplyPreviewRequestSchema,
  );
}

export function gitStashApply(
  repositoryId: string,
  worktreeId: string,
  request: GitStashExecuteRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/stash-apply`,
    GitStashApplyResultSchema,
    "POST",
    request,
    signal,
    GitStashExecuteRequestSchema,
  );
}

export function gitStashPop(
  repositoryId: string,
  worktreeId: string,
  request: GitStashExecuteRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/stash-pop`,
    GitStashApplyResultSchema,
    "POST",
    request,
    signal,
    GitStashExecuteRequestSchema,
  );
}

export function gitStashDropPreview(
  repositoryId: string,
  request: GitStashDropPreviewRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoGitBase(repositoryId)}/stashes/drop-preview`,
    GitStashDropPreviewSchema,
    "POST",
    request,
    signal,
    GitStashDropPreviewRequestSchema,
  );
}

export function gitStashDrop(
  repositoryId: string,
  request: GitStashDropRequest,
  signal?: AbortSignal,
) {
  return api(
    `${repoGitBase(repositoryId)}/stashes/drop`,
    GitStashDropResultSchema,
    "POST",
    request,
    signal,
    GitStashDropRequestSchema,
  );
}
