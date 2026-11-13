import { z } from "zod";
import { CommitListSchema, IssueSchema } from "./branch-schemas";
import { GitWorktreeConfirmationEnvelopeSchema } from "./git-api-schemas";
import { SessionInfoSchema } from "./session-schemas";
import { WorktreeSchema } from "./workspace-schemas";

// ---- Git workbench V2 divergence, coordinator recovery & 3-way conflict resolver ----

const GitConflictKindSchema = z.enum([
  "text",
  "binary",
  "delete_modify",
  "submodule",
  "symlink",
  "too_large",
]);
export type GitConflictKind = z.infer<typeof GitConflictKindSchema>;

const GitSequencerKindSchema = z.enum([
  "merge",
  "rebase",
  "cherry-pick",
  "cherry_pick",
  "revert",
  "bisect",
]);
export type GitSequencerKind = z.infer<typeof GitSequencerKindSchema>;

const GitDivergenceKindSchema = z.enum([
  "merge",
  "rebase",
  "cherry_pick",
  "revert",
]);
export type GitDivergenceKind = z.infer<typeof GitDivergenceKindSchema>;

const GitDivergenceOutcomeSchema = z.enum([
  "completed",
  "already_up_to_date",
  "conflicted",
  "aborted",
]);

const ConflictStagesShape = {
  stage1Mode: z.string().optional(),
  stage1Oid: z.string().optional(),
  stage2Mode: z.string().optional(),
  stage2Oid: z.string().optional(),
  stage3Mode: z.string().optional(),
  stage3Oid: z.string().optional(),
};

const GitConflictFileSummarySchema = z.object({
  path: z.string(),
  xy: z.string(),
  kind: GitConflictKindSchema,
  ...ConflictStagesShape,
});

export const GitInProgressOperationSchema = z.object({
  active: z.boolean(),
  kind: GitSequencerKindSchema.optional(),
  worktreeId: z.string(),
  worktreePath: z.string(),
  currentBranch: z.string().optional(),
  detached: z.boolean(),
  headOid: z.string().optional(),
  headSubject: z.string().optional(),
  origHeadOid: z.string().optional(),
  origHeadSubject: z.string().optional(),
  sourceLabel: z.string().optional(),
  sourceOid: z.string().optional(),
  sourceSubject: z.string().optional(),
  targetLabel: z.string().optional(),
  targetOid: z.string().optional(),
  targetSubject: z.string().optional(),
  stepCurrent: z.number().int().nonnegative().optional(),
  stepTotal: z.number().int().nonnegative().optional(),
  stoppedCommitOid: z.string().optional(),
  stoppedSubject: z.string().optional(),
  conflictedCount: z.number().int().nonnegative(),
  conflictedFiles: z.array(GitConflictFileSummarySchema),
  canContinue: z.boolean(),
  canAbort: z.boolean(),
  continueBlockers: z.array(IssueSchema),
});
export type GitInProgressOperation = z.infer<
  typeof GitInProgressOperationSchema
>;

export const GitBranchDivergencePreviewRequestSchema = z
  .object({
    refKind: z.enum(["local", "remote"]),
    refName: z.string().min(1),
  })
  .strict();
export type GitBranchDivergencePreviewRequest = z.infer<
  typeof GitBranchDivergencePreviewRequestSchema
>;

export const GitCommitDivergencePreviewRequestSchema = z
  .object({
    commitOid: z.string().min(1),
  })
  .strict();
export type GitCommitDivergencePreviewRequest = z.infer<
  typeof GitCommitDivergencePreviewRequestSchema
>;

export const GitDivergencePreviewSchema =
  GitWorktreeConfirmationEnvelopeSchema.extend({
    kind: GitDivergenceKindSchema,
    directionSummary: z.string(),
    recoveryHint: z.string(),
    currentBranch: z.string().optional(),
    headOid: z.string().optional(),
    headSubject: z.string().optional(),
    targetRef: z.string(),
    targetKind: z.string(),
    targetOid: z.string(),
    targetSubject: z.string(),
    mergeBaseOid: z.string().optional(),
    fastForward: z.boolean(),
    alreadyUpToDate: z.boolean(),
    commits: CommitListSchema,
    sessions: z.array(SessionInfoSchema),
    blockers: z.array(IssueSchema),
    warnings: z.array(IssueSchema),
  });
export type GitDivergencePreview = z.infer<typeof GitDivergencePreviewSchema>;

const ConfirmedWorktreeActionShape = {
  confirmationId: z.string().min(1),
  acknowledgeSessions: z.boolean(),
  clientRequestId: z.string().min(1).max(128).optional(),
};

export const GitDivergenceExecuteRequestSchema = z
  .object(ConfirmedWorktreeActionShape)
  .strict();
export type GitDivergenceExecuteRequest = z.infer<
  typeof GitDivergenceExecuteRequestSchema
>;

export const GitContinueOperationRequestSchema = z
  .object({
    kind: z.string().min(1),
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitContinueOperationRequest = z.infer<
  typeof GitContinueOperationRequestSchema
>;

export const GitAbortOperationPreviewSchema =
  GitWorktreeConfirmationEnvelopeSchema.extend({
    kind: GitSequencerKindSchema,
    currentBranch: z.string().optional(),
    headOid: z.string().optional(),
    restoreOid: z.string().optional(),
    restoreSubject: z.string().optional(),
    directionSummary: z.string(),
    affectedPaths: z.array(z.string()),
    sessions: z.array(SessionInfoSchema),
    blockers: z.array(IssueSchema),
    warnings: z.array(IssueSchema),
  });
export type GitAbortOperationPreview = z.infer<
  typeof GitAbortOperationPreviewSchema
>;

export const GitAbortOperationRequestSchema = z
  .object(ConfirmedWorktreeActionShape)
  .strict();
export type GitAbortOperationRequest = z.infer<
  typeof GitAbortOperationRequestSchema
>;

export const GitDivergenceResultSchema = z.object({
  operationId: z.string().optional(),
  kind: z.string(),
  outcome: GitDivergenceOutcomeSchema,
  summary: z.string(),
  headOid: z.string().optional(),
  subject: z.string().optional(),
  recoveryRef: z.string().optional(),
  inProgress: GitInProgressOperationSchema,
  worktree: WorktreeSchema,
});
export type GitDivergenceResult = z.infer<typeof GitDivergenceResultSchema>;

const GitConflictSideSchema = z.object({
  side: z.enum(["base", "left", "right"]),
  label: z.string(),
  role: z.string(),
  refName: z.string().optional(),
  commitOid: z.string().optional(),
  subject: z.string().optional(),
  stage: z.number().int().min(1).max(3),
  present: z.boolean(),
  mode: z.string().optional(),
  blobOid: z.string().optional(),
  sizeBytes: z.number().int().nonnegative(),
  binary: z.boolean(),
  text: z.string().optional(),
  lineEnding: z.string().optional(),
});
export type GitConflictSide = z.infer<typeof GitConflictSideSchema>;

const GitConflictSegmentKindSchema = z.enum([
  "common",
  "change_left",
  "change_right",
  "change_both_same",
  "conflict",
]);

const GitConflictSegmentSchema = z.object({
  id: z.string(),
  kind: GitConflictSegmentKindSchema,
  baseStart: z.number().int().nonnegative(),
  baseLines: z.array(z.string()),
  leftStart: z.number().int().nonnegative(),
  leftLines: z.array(z.string()),
  rightStart: z.number().int().nonnegative(),
  rightLines: z.array(z.string()),
});
export type GitConflictSegment = z.infer<typeof GitConflictSegmentSchema>;

const GitConflictSnapshotIdentitySchema = z.object({
  path: z.string(),
  operationKind: z.string().optional(),
  ...ConflictStagesShape,
  leftStage: z.number().int().min(1).max(3),
  rightStage: z.number().int().min(1).max(3),
  worktreeMode: z.string().optional(),
  worktreeSha256: z.string().optional(),
});

export const GitConflictFileDetailSchema = z.object({
  repositoryId: z.string(),
  worktreeId: z.string(),
  worktreePath: z.string(),
  path: z.string(),
  xy: z.string(),
  kind: GitConflictKindSchema,
  textMergeable: z.boolean(),
  degradeReason: z.string().optional(),
  operationKind: z.string().optional(),
  identity: GitConflictSnapshotIdentitySchema,
  base: GitConflictSideSchema,
  left: GitConflictSideSchema,
  right: GitConflictSideSchema,
  segments: z.array(GitConflictSegmentSchema),
  initialResultText: z.string(),
  diskText: z.string().optional(),
  diskHasConflictMarker: z.boolean(),
  diskModifiedFromBase: z.boolean(),
});
export type GitConflictFileDetail = z.infer<typeof GitConflictFileDetailSchema>;

export const GitSaveConflictRequestSchema = z
  .object({
    identity: GitConflictSnapshotIdentitySchema,
    resultText: z.string(),
  })
  .strict();
export type GitSaveConflictRequest = z.infer<
  typeof GitSaveConflictRequestSchema
>;

export const GitSaveConflictResponseSchema = z.object({
  path: z.string(),
  identity: GitConflictSnapshotIdentitySchema,
});

const GitConflictResolutionModeSchema = z.enum([
  "text",
  "choose_left",
  "choose_right",
  "delete",
]);

export const GitResolveConflictRequestSchema = z
  .object({
    identity: GitConflictSnapshotIdentitySchema,
    resolution: GitConflictResolutionModeSchema,
    resultText: z.string().optional(),
  })
  .strict();
export type GitResolveConflictRequest = z.infer<
  typeof GitResolveConflictRequestSchema
>;

export const GitResolveConflictResponseSchema = z.object({
  path: z.string(),
  inProgress: GitInProgressOperationSchema,
});
