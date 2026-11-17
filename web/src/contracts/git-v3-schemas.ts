import { z } from "zod";
import {
  IssueSchema,
  SwitchPreviewSchema,
  SwitchTargetSchema,
} from "./branch-schemas";
import {
  GitLineSelectionSchema,
  GitStageFileIdentitySchema,
  GitWorktreeConfirmationEnvelopeSchema,
} from "./git-api-schemas";
import { GitInProgressOperationSchema } from "./git-conflict-schemas";
import { SessionInfoSchema } from "./session-schemas";
import { WorktreeSchema } from "./workspace-schemas";

// ---- Git workbench V3: Scoped Discard Changes & Tracked-Stash Smart Checkout ----

const GitDiscardScopeSchema = z.enum(["files", "hunks", "lines"]);

const GitDiscardPreviewItemSchema = z.object({
  path: z.string(),
  status: z.string(),
  action: z.enum(["overwrite_tracked", "restore_deleted", "delete_untracked"]),
});

export const GitDiscardPreviewRequestSchema = z
  .object({
    scope: GitDiscardScopeSchema,
    paths: z.array(z.string().min(1)).optional(),
    identities: z.array(GitStageFileIdentitySchema).optional(),
    identity: GitStageFileIdentitySchema.optional(),
    hunkIds: z.array(z.string().min(1)).optional(),
    lineSelections: z.array(GitLineSelectionSchema).optional(),
  })
  .strict();
export type GitDiscardPreviewRequest = z.infer<
  typeof GitDiscardPreviewRequestSchema
>;

export const GitDiscardPreviewSchema =
  GitWorktreeConfirmationEnvelopeSchema.extend({
    scope: GitDiscardScopeSchema,
    trackedOverwritePaths: z.array(z.string()),
    untrackedDeletePaths: z.array(z.string()),
    items: z.array(GitDiscardPreviewItemSchema),
    hunkCount: z.number().int().nonnegative(),
    lineCount: z.number().int().nonnegative(),
    summary: z.string(),
    sessions: z.array(SessionInfoSchema),
    blockers: z.array(IssueSchema),
    warnings: z.array(IssueSchema),
  });
export type GitDiscardPreview = z.infer<typeof GitDiscardPreviewSchema>;

const GitConfirmedV3RequestSchema = z
  .object({
    confirmationId: z.string().min(1),
    clientRequestId: z.string().min(1).max(128).optional(),
    acknowledgeSessions: z.boolean().optional(),
  })
  .strict();

export const GitDiscardRequestSchema = GitConfirmedV3RequestSchema;
export type GitDiscardRequest = z.infer<typeof GitDiscardRequestSchema>;

export const GitDiscardResultSchema = z.object({
  operationId: z.string().optional(),
  scope: GitDiscardScopeSchema,
  summary: z.string(),
  recoveryOid: z.string().optional(),
  recoveryRef: z.string().optional(),
  recoveryCommand: z.string().optional(),
  discardedPaths: z.array(z.string()),
  worktree: WorktreeSchema,
});
export type GitDiscardResult = z.infer<typeof GitDiscardResultSchema>;

export const GitSmartCheckoutPreviewRequestSchema = z
  .object({
    target: SwitchTargetSchema,
    includeUntracked: z.boolean().optional(),
  })
  .strict();
export type GitSmartCheckoutPreviewRequest = z.infer<
  typeof GitSmartCheckoutPreviewRequestSchema
>;

export const GitSmartCheckoutPreviewSchema = z.object({
  confirmationId: z.string(),
  expiresAt: z.iso.datetime({ offset: true }).nullable(),
  repositoryId: z.string(),
  worktreeId: z.string(),
  worktree: WorktreeSchema,
  target: SwitchPreviewSchema.shape.target,
  includeUntracked: z.boolean(),
  stagedCount: z.number().int().nonnegative(),
  unstagedCount: z.number().int().nonnegative(),
  untrackedCount: z.number().int().nonnegative(),
  dirtyPaths: z.array(z.string()),
  hasActiveAgent: z.boolean(),
  sessions: z.array(SessionInfoSchema),
  blockers: z.array(IssueSchema),
  warnings: z.array(IssueSchema),
});
export type GitSmartCheckoutPreview = z.infer<
  typeof GitSmartCheckoutPreviewSchema
>;

export const GitSmartCheckoutRequestSchema = GitConfirmedV3RequestSchema;
export type GitSmartCheckoutRequest = z.infer<
  typeof GitSmartCheckoutRequestSchema
>;

const GitSmartCheckoutOutcomeSchema = z.enum([
  "completed",
  "rolled_back",
  "conflict_needs_resolution",
  "failed",
]);

export const GitSmartCheckoutResultSchema = z.object({
  operationId: z.string().optional(),
  outcome: GitSmartCheckoutOutcomeSchema,
  phase: z.string(),
  summary: z.string(),
  errorMessage: z.string().optional(),
  stashOid: z.string().optional(),
  stashRef: z.string().optional(),
  stashDropped: z.boolean(),
  stashPreserved: z.boolean(),
  stashRestored: z.boolean(),
  recoveryCommand: z.string().optional(),
  inProgress: GitInProgressOperationSchema,
  worktree: WorktreeSchema,
});
export type GitSmartCheckoutResult = z.infer<
  typeof GitSmartCheckoutResultSchema
>;
