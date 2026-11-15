import { APIError, api, apiBlob } from "./api";
import {
  CodeListingSchema,
  GitChangesSchema,
  GitCommitDetailResponseSchema,
  GitCommitPreviewRequestSchema,
  GitCommitPreviewSchema,
  GitCommitRequestSchema,
  GitCommitResultSchema,
  GitCompareSchema,
  GitConflictReasonSchema,
  GitContextSchema,
  GitDiffSchema,
  GitFileSchema,
  GitHistoryPageSchema,
  GitOperationSchema,
  GitOperationsSchema,
  GitStageRequestSchema,
  GitStagingFileDiffSchema,
  GitStagingOverviewSchema,
  NoContentSchema,
  type GitCommitPreviewRequest,
  type GitCommitRequest,
  type GitConflictReason,
  type GitHistoryScope,
  type GitSnapshotSide,
  type GitSource,
  type GitStageRequest,
} from "../contracts/schemas";

export interface GitTreeParams {
  contextId: string;
  worktreeId?: string;
  nodeId?: string;
  includeIgnored?: boolean;
  cursor?: string;
}

export interface GitFilesParams {
  contextId: string;
  worktreeId?: string;
  q?: string;
  includeIgnored?: boolean;
}

export interface GitFileParams {
  contextId: string;
  nodeId: string;
  source?: GitSource;
  worktreeId?: string;
}

export interface GitChangesParams {
  contextId: string;
  worktreeId?: string;
  group?: "unstaged" | "staged" | "untracked";
  q?: string;
  nodeId?: string;
  cursor?: string;
}

export interface GitDiffParams {
  contextId: string;
  changeId: string;
  worktreeId?: string;
}

export interface GitCompareParams {
  contextId: string;
  baseOid?: string;
  targetOid: string;
  path?: string;
  changeId?: string;
  q?: string;
  cursor?: string;
  worktreeId?: string;
}

export interface GitSnapshotParams {
  side?: GitSnapshotSide;
  contextId?: string;
  worktreeId?: string;
  download?: boolean;
}

export interface GitHistoryParams {
  worktreeId?: string;
  scope?: GitHistoryScope;
  branch?: string;
  q?: string;
  author?: string;
  since?: string;
  until?: string;
  path?: string;
  limit?: number;
  cursor?: string;
}

export interface GitStagingDiffParams {
  area: "unstaged" | "staged";
  path: string;
  oldPath?: string;
}

export function withGitQuery(
  path: string,
  entries: Record<string, string | number | boolean | undefined>,
): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(entries)) {
    if (value === undefined || value === "") continue;
    query.set(key, String(value));
  }
  const encoded = query.toString();
  return encoded ? `${path}?${encoded}` : path;
}

export function repoGitBase(repositoryId: string): string {
  return `/repositories/${encodeURIComponent(repositoryId)}/git`;
}

export function worktreeGitBase(
  repositoryId: string,
  worktreeId: string,
): string {
  return `/repositories/${encodeURIComponent(repositoryId)}/worktrees/${encodeURIComponent(worktreeId)}/git`;
}

/** Extracts a validated machine-readable 409 conflict reason from `APIError`. */
export function gitConflictReason(err: unknown): GitConflictReason | undefined {
  if (!(err instanceof APIError) || err.status !== 409 || !err.reason) {
    return undefined;
  }
  const parsed = GitConflictReasonSchema.safeParse(err.reason);
  return parsed.success ? parsed.data : undefined;
}

export function gitContext(
  repositoryId: string,
  worktreeId?: string,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/context`, { worktreeId }),
    GitContextSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitTree(
  repositoryId: string,
  params: GitTreeParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/tree`, {
      contextId: params.contextId,
      worktreeId: params.worktreeId,
      nodeId: params.nodeId,
      includeIgnored: params.includeIgnored,
      cursor: params.cursor,
    }),
    CodeListingSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitFiles(
  repositoryId: string,
  params: GitFilesParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/files`, {
      contextId: params.contextId,
      worktreeId: params.worktreeId,
      q: params.q,
      includeIgnored: params.includeIgnored,
    }),
    CodeListingSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitFile(
  repositoryId: string,
  params: GitFileParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/file`, {
      contextId: params.contextId,
      nodeId: params.nodeId,
      source: params.source,
      worktreeId: params.worktreeId,
    }),
    GitFileSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitChanges(
  repositoryId: string,
  params: GitChangesParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/changes`, {
      contextId: params.contextId,
      worktreeId: params.worktreeId,
      group: params.group,
      q: params.q,
      nodeId: params.nodeId,
      cursor: params.cursor,
    }),
    GitChangesSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitDiff(
  repositoryId: string,
  params: GitDiffParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/diff`, {
      contextId: params.contextId,
      changeId: params.changeId,
      worktreeId: params.worktreeId,
    }),
    GitDiffSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitCompare(
  repositoryId: string,
  params: GitCompareParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/compare`, {
      contextId: params.contextId,
      baseOid: params.baseOid,
      targetOid: params.targetOid,
      path: params.path,
      changeId: params.changeId,
      q: params.q,
      cursor: params.cursor,
      worktreeId: params.worktreeId,
    }),
    GitCompareSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitSnapshotFile(
  repositoryId: string,
  snapshotId: string,
  params: GitSnapshotParams = {},
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(
      `${repoGitBase(repositoryId)}/snapshots/${encodeURIComponent(snapshotId)}/file`,
      {
        side: params.side,
        contextId: params.contextId,
        worktreeId: params.worktreeId,
      },
    ),
    GitFileSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitSnapshotRaw(
  repositoryId: string,
  snapshotId: string,
  params: GitSnapshotParams = {},
  signal?: AbortSignal,
  received?: (bytes: number) => void,
) {
  return apiBlob(
    withGitQuery(
      `${repoGitBase(repositoryId)}/snapshots/${encodeURIComponent(snapshotId)}/raw`,
      {
        side: params.side,
        contextId: params.contextId,
        worktreeId: params.worktreeId,
        download: params.download ? 1 : undefined,
      },
    ),
    signal,
    received,
  );
}

export function gitHistory(
  repositoryId: string,
  params: GitHistoryParams = {},
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/history`, {
      worktreeId: params.worktreeId,
      scope: params.scope,
      branch: params.branch,
      q: params.q,
      author: params.author,
      since: params.since,
      until: params.until,
      path: params.path,
      limit: params.limit,
      cursor: params.cursor,
    }),
    GitHistoryPageSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitCommitDetail(
  repositoryId: string,
  oid: string,
  parentOid?: string,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(
      `${repoGitBase(repositoryId)}/commits/${encodeURIComponent(oid)}`,
      {
        parentOid,
      },
    ),
    GitCommitDetailResponseSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitStaging(
  repositoryId: string,
  worktreeId: string,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/staging`,
    GitStagingOverviewSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitStagingDiff(
  repositoryId: string,
  worktreeId: string,
  params: GitStagingDiffParams,
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${worktreeGitBase(repositoryId, worktreeId)}/staging/diff`, {
      area: params.area,
      path: params.path,
      oldPath: params.oldPath,
    }),
    GitStagingFileDiffSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitStage(
  repositoryId: string,
  worktreeId: string,
  request: GitStageRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/stage`,
    NoContentSchema,
    "POST",
    request,
    signal,
    GitStageRequestSchema,
  );
}

export function gitUnstage(
  repositoryId: string,
  worktreeId: string,
  request: GitStageRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/unstage`,
    NoContentSchema,
    "POST",
    request,
    signal,
    GitStageRequestSchema,
  );
}

export function gitCommitPreview(
  repositoryId: string,
  worktreeId: string,
  requestOrSignal?: GitCommitPreviewRequest | AbortSignal,
  signal?: AbortSignal,
) {
  const isSignal =
    requestOrSignal !== undefined && "aborted" in requestOrSignal;
  const request: GitCommitPreviewRequest = isSignal
    ? {}
    : (requestOrSignal ?? {});
  const abortSignal = isSignal ? requestOrSignal : signal;
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/commit-preview`,
    GitCommitPreviewSchema,
    "POST",
    request,
    abortSignal,
    GitCommitPreviewRequestSchema,
  );
}

export interface GitOperationsParams {
  worktreeId?: string;
  limit?: number;
}

export function gitCommit(
  repositoryId: string,
  worktreeId: string,
  request: GitCommitRequest,
  signal?: AbortSignal,
) {
  return api(
    `${worktreeGitBase(repositoryId, worktreeId)}/commit`,
    GitCommitResultSchema,
    "POST",
    request,
    signal,
    GitCommitRequestSchema,
  );
}

export function gitOperations(
  repositoryId: string,
  params: GitOperationsParams = {},
  signal?: AbortSignal,
) {
  return api(
    withGitQuery(`${repoGitBase(repositoryId)}/operations`, {
      worktreeId: params.worktreeId,
      limit: params.limit,
    }),
    GitOperationsSchema,
    "GET",
    undefined,
    signal,
  );
}

export function gitOperation(
  repositoryId: string,
  operationId: string,
  signal?: AbortSignal,
) {
  return api(
    `${repoGitBase(repositoryId)}/operations/${encodeURIComponent(operationId)}`,
    GitOperationSchema,
    "GET",
    undefined,
    signal,
  );
}
