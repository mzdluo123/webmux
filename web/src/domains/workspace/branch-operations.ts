// The server calls of the branch flows: a preview, then the confirmed
// execution. Flows take them as one object so stories can pass fixtures.
import { api, APIError, repositoryRemotes } from "../../platform/api";
import {
  BranchDeletePreviewRequestSchema,
  BranchDeletePreviewSchema,
  BranchDeleteRequestSchema,
  BranchDeleteResultSchema,
  FastForwardPreviewRequestSchema,
  FastForwardPreviewSchema,
  FastForwardRequestSchema,
  FastForwardResultSchema,
  PushPreviewRequestSchema,
  PushPreviewSchema,
  PushRequestSchema,
  PushResultSchema,
  type BranchDeletePreview,
  type BranchDeleteResult,
  type FastForwardPreview,
  type FastForwardResult,
  type PushPreview,
  type PushPreviewRequest,
  type PushResult,
  type Remotes,
} from "../../contracts/schemas";

interface BranchOperationExecOptions {
  clientRequestId?: string;
  worktreeId?: string;
  signal?: AbortSignal;
}

export interface BranchOperations {
  fastForwardPreview(
    repositoryId: string,
    branch: string,
    signal: AbortSignal,
  ): Promise<FastForwardPreview>;
  fastForward(
    repositoryId: string,
    confirmationId: string,
    acknowledgeSessions: boolean,
    options?: BranchOperationExecOptions,
  ): Promise<FastForwardResult>;
  pushPreview(
    repositoryId: string,
    request: PushPreviewRequest,
    signal: AbortSignal,
  ): Promise<PushPreview>;
  push(
    repositoryId: string,
    confirmationId: string,
    options?: BranchOperationExecOptions,
  ): Promise<PushResult>;
  deletePreview(
    repositoryId: string,
    branches: readonly string[],
    signal?: AbortSignal,
  ): Promise<BranchDeletePreview>;
  deleteBranches(
    repositoryId: string,
    confirmationId: string,
    acknowledgeUnmerged: boolean,
  ): Promise<BranchDeleteResult>;
  remotes(repositoryId: string, signal: AbortSignal): Promise<Remotes>;
}

/** What a flow component takes besides the branches it acts on. */
export interface BranchFlowProps {
  repositoryId: string;
  /** A request runs; the host keeps itself open. */
  busy?: boolean;
  source?: BranchOperations;
  /** The repository changed (or may have): read it again. */
  onchange: () => void;
  onclose: () => void;
}

const base = (id: string) => `/repositories/${id}/branches`;

export const branchOperations: BranchOperations = {
  fastForwardPreview: (id, branch, signal) =>
    api(
      `${base(id)}/fast-forward-preview`,
      FastForwardPreviewSchema,
      "POST",
      { branch },
      signal,
      FastForwardPreviewRequestSchema,
    ),
  fastForward: (id, confirmationId, acknowledgeSessions, options) =>
    api(
      `${base(id)}/fast-forward`,
      FastForwardResultSchema,
      "POST",
      {
        confirmationId,
        acknowledgeSessions,
        ...(options?.clientRequestId
          ? { clientRequestId: options.clientRequestId }
          : {}),
        ...(options?.worktreeId ? { worktreeId: options.worktreeId } : {}),
      },
      options?.signal,
      FastForwardRequestSchema,
    ),
  pushPreview: (id, request, signal) =>
    api(
      `${base(id)}/push-preview`,
      PushPreviewSchema,
      "POST",
      request,
      signal,
      PushPreviewRequestSchema,
    ),
  push: (id, confirmationId, options) =>
    api(
      `${base(id)}/push`,
      PushResultSchema,
      "POST",
      {
        confirmationId,
        ...(options?.clientRequestId
          ? { clientRequestId: options.clientRequestId }
          : {}),
        ...(options?.worktreeId ? { worktreeId: options.worktreeId } : {}),
      },
      options?.signal,
      PushRequestSchema,
    ),
  deletePreview: (id, branches, signal) =>
    api(
      `${base(id)}/delete-preview`,
      BranchDeletePreviewSchema,
      "POST",
      { branches: [...branches] },
      signal,
      BranchDeletePreviewRequestSchema,
    ),
  deleteBranches: (id, confirmationId, acknowledgeUnmerged) =>
    api(
      `${base(id)}/delete`,
      BranchDeleteResultSchema,
      "POST",
      { confirmationId, acknowledgeUnmerged },
      undefined,
      BranchDeleteRequestSchema,
    ),
  remotes: (id, signal) => repositoryRemotes(id, signal),
};

/** The previewed state changed before the execution: preview again. */
export const isStale = (error: unknown) =>
  error instanceof APIError && error.status === 409;
