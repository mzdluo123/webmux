import { api, APIError } from "./api";
import { newRequestId } from "./ids";
import {
  EmptyRequestSchema,
  HybridConflictSchema,
  HybridRecordSchema,
  HybridRecordsSchema,
  NoContentSchema,
  RebuildPreviewSchema,
  RebuildRequestSchema,
  ReleasePreviewSchema,
  ReleaseResultSchema,
  ReleaseWorktreeRequestSchema,
  SessionInfoSchema,
  WorktreeSchema,
  type AgentConnector,
  type HybridCandidate,
  type HybridConflict,
  type HybridRecord,
  type RebuildPreview,
  type SessionInfo,
  type Worktree,
} from "./schemas";

/** The hybrid conflict a 409 carries, or undefined for any other failure. */
export function hybridConflict(err: unknown): HybridConflict | undefined {
  if (!(err instanceof APIError) || err.status !== 409) return undefined;
  const conflict = HybridConflictSchema.safeParse(err.body);
  return conflict.success ? conflict.data : undefined;
}

// Hybrid session records that can be resumed: closed or interrupted sessions
// whose agent conversation lives on in the agent's own store.

export function loadRecoverable(signal?: AbortSignal) {
  return api(
    "/hybrid-sessions?state=recoverable",
    HybridRecordsSchema,
    "GET",
    undefined,
    signal,
  );
}

/**
 * The record of session `id`, the session page's fallback when the session
 * itself is gone. Undefined when there is none: no such record (404) or no
 * readable store (503). Other failures prove nothing and are thrown.
 */
export async function loadRecord(
  id: string,
  signal?: AbortSignal,
): Promise<HybridRecord | undefined> {
  try {
    return await api(
      `/hybrid-sessions/${id}`,
      HybridRecordSchema,
      "GET",
      undefined,
      signal,
    );
  } catch (err) {
    if (err instanceof APIError && (err.status === 404 || err.status === 503))
      return undefined;
    throw err;
  }
}

export type ResumeChoice = {
  /** Picks another agent session after the recorded one went missing. */
  agentSessionId?: string;
  /** Accepts resuming after the launch config (and its env) was deleted. */
  confirmWithoutEnv?: boolean;
};

export type ResumeOutcome =
  | { status: "resumed"; session: SessionInfo }
  | { status: "candidates"; message: string; candidates: HybridCandidate[] }
  | { status: "config_missing"; message: string }
  | { status: "connector_missing"; message: string; connector: AgentConnector };

/** Resumes a record on one side; 409s that need a user choice are returned. */
export async function resumeHybrid(
  id: string,
  side: "tui" | "acp",
  choice: ResumeChoice = {},
): Promise<ResumeOutcome> {
  try {
    const session = await api(
      `/sessions/${id}/resume`,
      SessionInfoSchema,
      "POST",
      { clientRequestId: newRequestId(), side, ...choice },
    );
    return { status: "resumed", session };
  } catch (err) {
    const conflict = hybridConflict(err);
    if (conflict?.reason === "session_missing")
      return {
        status: "candidates",
        message: conflict.error,
        candidates: conflict.candidates ?? [],
      };
    if (conflict?.reason === "config_missing")
      return { status: "config_missing", message: conflict.error };
    if (conflict?.reason === "connector_missing" && conflict.connector)
      return {
        status: "connector_missing",
        message: conflict.error,
        connector: conflict.connector,
      };
    throw err;
  }
}

export function removeRecord(id: string) {
  return api(`/hybrid-sessions/${id}`, NoContentSchema, "DELETE");
}

function worktreeRoute(worktree: Worktree) {
  return `/repositories/${worktree.repositoryId}/worktrees/${worktree.id}`;
}

/**
 * Reads what releasing `worktree` would remove and keep: its blockers, the
 * archived records that stay, and the ignored files deleted with it.
 */
export function previewRelease(worktree: Worktree) {
  return api(
    worktreeRoute(worktree) + "/release-preview",
    ReleasePreviewSchema,
    "POST",
    {},
    undefined,
    EmptyRequestSchema,
  );
}

/** Releases the worktree a preview confirmed; its records stay archived. */
export function releaseWorktree(worktree: Worktree, confirmationId: string) {
  return api(
    worktreeRoute(worktree) + "/release",
    ReleaseResultSchema,
    "POST",
    { confirmationId },
    undefined,
    ReleaseWorktreeRequestSchema,
  );
}

/** Reads how the released worktree of record `id` would be rebuilt. */
export function previewRebuild(id: string) {
  return api(
    `/hybrid-sessions/${id}/rebuild-preview`,
    RebuildPreviewSchema,
    "POST",
    {},
    undefined,
    EmptyRequestSchema,
  );
}

/**
 * Rebuilds the released worktree of record `id` the way `preview` showed;
 * the server refuses when the repository has changed since.
 */
export function rebuildWorktree(id: string, preview: RebuildPreview) {
  return api(
    `/hybrid-sessions/${id}/rebuild`,
    WorktreeSchema,
    "POST",
    {
      releaseId: preview.releaseId,
      mode: preview.mode,
      commit: preview.commit,
    },
    undefined,
    RebuildRequestSchema,
  );
}
