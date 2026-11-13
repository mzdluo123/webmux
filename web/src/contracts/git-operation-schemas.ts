import { z } from "zod";
import { FetchResultSchema } from "./workspace-schemas";

// ---- Persistent Git operation ledger ----

const GitOperationKindSchema = z.enum([
  "commit",
  "push",
  "fetch",
  "fast_forward",
  "switch",
  "merge",
  "rebase",
  "cherry_pick",
  "revert",
  "continue",
  "abort",
  "stash_push",
  "stash_apply",
  "stash_pop",
  "stash_drop",
  "branch_rename",
  "branch_upstream",
  "tag_create",
  "tag_delete",
  "remote_add",
  "remote_update",
  "remote_remove",
]);
export type GitOperationKind = z.infer<typeof GitOperationKindSchema>;

const GitOperationStatusSchema = z.enum([
  "pending",
  "running",
  "succeeded",
  "needs_review",
  "failed",
  "uncertain",
]);
export type GitOperationStatus = z.infer<typeof GitOperationStatusSchema>;

const GitOperationPreviewSchema = z.object({
  worktreePath: z.string().optional(),
  headState: z.string().optional(),
  branch: z.string().optional(),
  headOid: z.string().optional(),
  expectedTreeOid: z.string().optional(),
  expectedParents: z.array(z.string()).optional(),
  amend: z.boolean().optional(),
  authorOverride: z.string().optional(),
  signoff: z.boolean().optional(),
  stashRef: z.string().optional(),
  stashOid: z.string().optional(),
  remote: z.string().optional(),
  target: z.string().optional(),
  targetOid: z.string().optional(),
  publish: z.boolean().optional(),
  detached: z.boolean().optional(),
});

export const GitCommitOutcomeIdentityFields = {
  parents: z.array(z.string()),
  subject: z.string().optional(),
  authorName: z.string().optional(),
  authorEmail: z.string().optional(),
  committerName: z.string().optional(),
  committerEmail: z.string().optional(),
  reviewReasons: z.array(z.string()),
};

export const GitOperationSchema = z.object({
  id: z.string(),
  repositoryId: z.string(),
  worktreeId: z.string().optional(),
  clientRequestId: z.string().optional(),
  kind: GitOperationKindSchema,
  status: GitOperationStatusSchema,
  confirmationId: z.string().optional(),
  startedAt: z.iso.datetime({ offset: true }),
  finishedAt: z.iso.datetime({ offset: true }).nullable(),
  summary: z.string(),
  sanitizedOutput: z.string().optional(),
  remoteStatus: FetchResultSchema.shape.status.optional(),
  fetchResults: z.array(FetchResultSchema),
  preview: GitOperationPreviewSchema,
  actualOid: z.string().optional(),
  actualTreeOid: z.string().optional(),
  ...GitCommitOutcomeIdentityFields,
});
export type GitOperation = z.infer<typeof GitOperationSchema>;

export const GitOperationsSchema = z.object({
  operations: z.array(GitOperationSchema),
});
