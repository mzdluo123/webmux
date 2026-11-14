import { api, APIError } from "./api";
import {
  GitCommitDraftSchema,
  GitConflictSchema,
  GitPutCommitDraftRequestSchema,
  GitPutReviewsRequestSchema,
  GitReviewsResponseSchema,
  type GitCommitDraft,
  type GitPutCommitDraftRequest,
  type GitPutReviewsRequest,
  type GitReviewScopeParam,
} from "../contracts/schemas";

function worktreeCollabBase(repositoryId: string, worktreeId: string): string {
  return `/repositories/${encodeURIComponent(repositoryId)}/worktrees/${encodeURIComponent(worktreeId)}/git`;
}

export function gitCommitDraft(
  repositoryId: string,
  worktreeId: string,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeCollabBase(repositoryId, worktreeId)}/commit-draft`,
    GitCommitDraftSchema,
    "GET",
    undefined,
    signal,
  );
}

export type GitPutCommitDraftOutcome =
  | { ok: true; draft: GitCommitDraft }
  | { ok: false; conflict: GitCommitDraft; message: string };

export async function gitPutCommitDraft(
  repositoryId: string,
  worktreeId: string,
  request: GitPutCommitDraftRequest,
  signal?: AbortSignal,
): Promise<GitPutCommitDraftOutcome> {
  try {
    const draft = await api(
      `${worktreeCollabBase(repositoryId, worktreeId)}/commit-draft`,
      GitCommitDraftSchema,
      "PUT",
      request,
      signal,
      GitPutCommitDraftRequestSchema,
    );
    return { ok: true, draft };
  } catch (err) {
    if (err instanceof APIError && err.status === 409) {
      const parsed = GitConflictSchema.safeParse(err.body);
      if (
        parsed.success &&
        parsed.data.reason === "draft_conflict" &&
        parsed.data.currentDraft
      ) {
        return {
          ok: false,
          conflict: parsed.data.currentDraft,
          message: parsed.data.error,
        };
      }
    }
    throw err;
  }
}

export function gitReviews(
  repositoryId: string,
  worktreeId: string,
  scope: GitReviewScopeParam,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeCollabBase(repositoryId, worktreeId)}/reviews?scope=${encodeURIComponent(scope)}`,
    GitReviewsResponseSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitPutReviews(
  repositoryId: string,
  worktreeId: string,
  request: GitPutReviewsRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeCollabBase(repositoryId, worktreeId)}/reviews`,
    GitReviewsResponseSchema,
    "PUT",
    request,
    signal,
    GitPutReviewsRequestSchema,
  );
}
