import { api } from "./api";
import {
  GitDiffSchema,
  GitPutTaskBaselineRequestSchema,
  GitTaskBaselineResponseSchema,
  GitTaskChangesSchema,
  NoContentSchema,
  type GitPutTaskBaselineRequest,
} from "../contracts/schemas";

export interface GitTaskChangesParams {
  q?: string;
  nodeId?: string;
  cursor?: string;
}

export interface GitTaskDiffParams {
  path?: string;
  changeId?: string;
}

function worktreeTaskBase(repositoryId: string, worktreeId: string): string {
  return `/repositories/${encodeURIComponent(repositoryId)}/worktrees/${encodeURIComponent(worktreeId)}/git`;
}

function withTaskQuery(
  path: string,
  entries: Record<string, string | undefined>,
): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(entries)) {
    if (!value) continue;
    query.set(key, value);
  }
  const encoded = query.toString();
  return encoded ? `${path}?${encoded}` : path;
}

export function gitTaskBaseline(
  repositoryId: string,
  worktreeId: string,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeTaskBase(repositoryId, worktreeId)}/task-baseline`,
    GitTaskBaselineResponseSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitPutTaskBaseline(
  repositoryId: string,
  worktreeId: string,
  request: GitPutTaskBaselineRequest = {},
  signal?: AbortSignal,
) {
  return api(
    `${worktreeTaskBase(repositoryId, worktreeId)}/task-baseline`,
    GitTaskBaselineResponseSchema,
    "PUT",
    request,
    signal,
    GitPutTaskBaselineRequestSchema,
  );
}

export function gitDeleteTaskBaseline(
  repositoryId: string,
  worktreeId: string,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeTaskBase(repositoryId, worktreeId)}/task-baseline`,
    NoContentSchema,
    "DELETE",
    undefined,
    signal,
  );
}

export function gitTaskChanges(
  repositoryId: string,
  worktreeId: string,
  params: GitTaskChangesParams = {},
  signal?: AbortSignal,
) {
  return api(
    withTaskQuery(
      `${worktreeTaskBase(repositoryId, worktreeId)}/task-changes`,
      {
        q: params.q,
        nodeId: params.nodeId,
        cursor: params.cursor,
      },
    ),
    GitTaskChangesSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitTaskDiff(
  repositoryId: string,
  worktreeId: string,
  params: GitTaskDiffParams,
  signal?: AbortSignal,
) {
  return api(
    withTaskQuery(`${worktreeTaskBase(repositoryId, worktreeId)}/task-diff`, {
      path: params.path,
      changeId: params.changeId,
    }),
    GitDiffSchema,
    "GET",
    undefined,
    signal,
  );
}
