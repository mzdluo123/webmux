import { z } from "zod";
import { IssueSchema } from "./branch-schemas";
import {
  CodeChangeSchema,
  CodeChangesSchema,
  CodeContextSchema,
  CodeDiffSchema,
  CodeDocumentSchema,
} from "./code-schemas";
import { GitCommitDraftSchema } from "./git-collab-schemas";
import { FetchResultSchema, WorktreeChangesSchema } from "./workspace-schemas";

// ---- Read-only repository/worktree Git context, snapshots & compare ----

export const GitContextSchema = CodeContextSchema.extend({
  rootKind: z.enum(["repository", "bare", "worktree"]),
});
export type GitContext = z.infer<typeof GitContextSchema>;

export const GitSourceSchema = z.union([
  z.enum(["worktree", "index", "head", "empty"]),
  z.string().regex(/^commit:[0-9a-f]{40,64}$/),
]);
export type GitSource = z.infer<typeof GitSourceSchema>;

export const GitDocumentSchema = CodeDocumentSchema.extend({
  source: GitSourceSchema,
});
export type GitDocument = z.infer<typeof GitDocumentSchema>;

export const GitFileSchema = z.object({
  snapshotId: z.string(),
  document: GitDocumentSchema,
  checkedAt: z.string(),
});
export type GitFile = z.infer<typeof GitFileSchema>;

export const GitChangeSchema = CodeChangeSchema.extend({
  group: z.enum(["unstaged", "staged", "untracked", "commit", "task"]),
});
export type GitChange = z.infer<typeof GitChangeSchema>;

export const GitChangesSchema = CodeChangesSchema.extend({
  entries: z.array(GitChangeSchema).max(200),
});
export type GitChanges = z.infer<typeof GitChangesSchema>;

export const GitDiffSchema = CodeDiffSchema.extend({
  old: GitDocumentSchema,
  new: GitDocumentSchema,
  change: GitChangeSchema,
});
export type GitDiff = z.infer<typeof GitDiffSchema>;

export const GitCompareSchema = z.object({
  baseOid: z.string(),
  targetOid: z.string(),
  changes: GitChangesSchema,
  diff: GitDiffSchema.optional(),
});
export type GitCompare = z.infer<typeof GitCompareSchema>;

export const GitSnapshotSideSchema = z.enum(["before", "after"]);
export type GitSnapshotSide = z.infer<typeof GitSnapshotSideSchema>;

// ---- Paginated commit history graph & commit detail ----

export const GitHistoryScopeSchema = z.enum([
  "all",
  "branch",
  "task",
  "to-push",
  "incoming",
]);
export type GitHistoryScope = z.infer<typeof GitHistoryScopeSchema>;

export const GitHistoryStatusSchema = z.enum([
  "ready",
  "empty",
  "unborn",
  "no_worktree",
  "no_task_baseline",
  "no_upstream",
  "upstream_gone",
  "detached_no_branch",
]);
export type GitHistoryStatus = z.infer<typeof GitHistoryStatusSchema>;

const GitTaskBaselineSourceSchema = z.enum([
  "worktree_create",
  "manual_select",
]);

const GitTaskBaselineSchema = z.object({
  baselineId: z.string(),
  repositoryId: z.string(),
  worktreeId: z.string(),
  worktreePath: z.string(),
  baseOid: z.string().optional(),
  unborn: z.boolean(),
  capturedAt: z.iso.datetime({ offset: true }),
  source: GitTaskBaselineSourceSchema,
  cleanAtStart: z.boolean(),
  preExistingChanges: WorktreeChangesSchema,
});
export type GitTaskBaseline = z.infer<typeof GitTaskBaselineSchema>;

export const GitRefBadgeSchema = z.object({
  name: z.string(),
  kind: z.enum(["head", "local", "remote", "tag"]),
  worktreeId: z.string().optional(),
});
export type GitRefBadge = z.infer<typeof GitRefBadgeSchema>;

export const GitHistoryEdgeSchema = z.object({
  fromLane: z.number().int().nonnegative(),
  toLane: z.number().int().nonnegative(),
  targetOid: z.string(),
  kind: z.enum(["direct", "indirect", "boundary"]),
});
export type GitHistoryEdge = z.infer<typeof GitHistoryEdgeSchema>;

export const GitHistoryItemSchema = z.object({
  oid: z.string(),
  shortOid: z.string(),
  parents: z.array(z.string()),
  subject: z.string(),
  authorName: z.string(),
  authorEmail: z.string(),
  authorAt: z.string(),
  committerName: z.string(),
  committerAt: z.string(),
  refs: z.array(GitRefBadgeSchema),
  lane: z.number().int().nonnegative(),
  activeLanes: z.array(z.number().int().nonnegative()),
  edges: z.array(GitHistoryEdgeSchema),
  directParentsMissing: z.boolean(),
});
export type GitHistoryItem = z.infer<typeof GitHistoryItemSchema>;

export const GitHistoryPageSchema = z.object({
  status: GitHistoryStatusSchema,
  reason: z.string().optional(),
  scope: GitHistoryScopeSchema,
  lastFetchedAt: z.iso.datetime({ offset: true }).optional(),
  upstream: z.string().optional(),
  taskBaseline: GitTaskBaselineSchema.optional(),
  refsChanged: z.boolean(),
  nextCursor: z.string(),
  rows: z.array(GitHistoryItemSchema),
});
export type GitHistoryPage = z.infer<typeof GitHistoryPageSchema>;

export const GitCommitParentSchema = z.object({
  oid: z.string(),
  shortOid: z.string(),
  subject: z.string(),
});
export type GitCommitParent = z.infer<typeof GitCommitParentSchema>;

export const GitCommitFileChangeSchema = z.object({
  oldPath: z.string(),
  newPath: z.string(),
  status: z.string(),
  oldMode: z.string(),
  newMode: z.string(),
  oldOid: z.string(),
  newOid: z.string(),
  modeChanged: z.boolean(),
  binary: z.boolean(),
});
export type GitCommitFileChange = z.infer<typeof GitCommitFileChangeSchema>;

export const GitCommitDetailResponseSchema = z.object({
  oid: z.string(),
  shortOid: z.string(),
  subject: z.string(),
  body: z.string(),
  message: z.string(),
  authorName: z.string(),
  authorEmail: z.string(),
  authorAt: z.string(),
  committerName: z.string(),
  committerEmail: z.string(),
  committerAt: z.string(),
  distinctCommitter: z.boolean(),
  parents: z.array(GitCommitParentSchema),
  comparedParentOid: z.string(),
  refs: z.array(GitRefBadgeSchema),
  files: z.array(GitCommitFileChangeSchema),
});
export type GitCommitDetailResponse = z.infer<
  typeof GitCommitDetailResponseSchema
>;

// ---- Worktree index staging, hunk diff, commit preview & execution ----

export const GitStageAreaSchema = z.enum(["unstaged", "staged", "conflicted"]);
export type GitStageArea = z.infer<typeof GitStageAreaSchema>;

export const GitStageFileIdentitySchema = z.object({
  area: GitStageAreaSchema,
  path: z.string(),
  oldPath: z.string().optional(),
  oldBaseOid: z.string().optional(),
  oldBaseMode: z.string().optional(),
  baseOid: z.string().optional(),
  baseMode: z.string().optional(),
  targetOid: z.string().optional(),
  targetSha256: z.string().optional(),
  targetMode: z.string().optional(),
});
export type GitStageFileIdentity = z.infer<typeof GitStageFileIdentitySchema>;

export const GitStagingEntrySchema = z.object({
  path: z.string(),
  oldPath: z.string().optional(),
  area: GitStageAreaSchema,
  status: z.string(),
  hunkStageAvailable: z.boolean(),
  degradeReason: z.string().optional(),
  identity: GitStageFileIdentitySchema,
});
export type GitStagingEntry = z.infer<typeof GitStagingEntrySchema>;

export const GitStagingOverviewSchema = z.object({
  unborn: z.boolean(),
  headOid: z.string().optional(),
  staged: z.array(GitStagingEntrySchema),
  unstaged: z.array(GitStagingEntrySchema),
  conflicted: z.array(GitStagingEntrySchema),
});
export type GitStagingOverview = z.infer<typeof GitStagingOverviewSchema>;

export const GitStagingHunkLineSchema = z.object({
  kind: z.enum(["context", "delete", "add"]),
  oldLine: z.number().int().positive().optional(),
  newLine: z.number().int().positive().optional(),
  text: z.string(),
  lineEnding: z.enum(["lf", "crlf", "none"]),
});
export type GitStagingHunkLine = z.infer<typeof GitStagingHunkLineSchema>;

export const GitStagingHunkSchema = z.object({
  id: z.string(),
  header: z.string(),
  oldStart: z.number().int().nonnegative(),
  oldLines: z.number().int().nonnegative(),
  newStart: z.number().int().nonnegative(),
  newLines: z.number().int().nonnegative(),
  lines: z.array(GitStagingHunkLineSchema),
});
export type GitStagingHunk = z.infer<typeof GitStagingHunkSchema>;

export const GitStagingFileDiffSchema = z.object({
  identity: GitStageFileIdentitySchema,
  conflicted: z.boolean(),
  binary: z.boolean(),
  hunkStageAvailable: z.boolean(),
  degradeReason: z.string().optional(),
  hunks: z.array(GitStagingHunkSchema),
});
export type GitStagingFileDiff = z.infer<typeof GitStagingFileDiffSchema>;

export const GitStageRequestSchema = z
  .object({
    identity: GitStageFileIdentitySchema,
    hunkIds: z.array(z.string().min(1)).optional(),
  })
  .strict();
export type GitStageRequest = z.infer<typeof GitStageRequestSchema>;

export const GitCommitIdentitySchema = z.object({
  authorName: z.string(),
  authorEmail: z.string(),
  committerName: z.string(),
  committerEmail: z.string(),
  signEnabled: z.boolean(),
  signFormat: z.string().optional(),
});
export type GitCommitIdentity = z.infer<typeof GitCommitIdentitySchema>;

export const GitWorktreeConfirmationEnvelopeSchema = z.object({
  confirmationId: z.string(),
  expiresAt: z.iso.datetime({ offset: true }).nullable(),
  repositoryId: z.string(),
  worktreeId: z.string(),
  worktreePath: z.string(),
});

export const GitCommitPreviewSchema =
  GitWorktreeConfirmationEnvelopeSchema.extend({
    headState: z.enum(["branch", "detached", "unborn"]),
    branch: z.string().optional(),
    headOid: z.string().optional(),
    expectedTreeOid: z.string().optional(),
    indexDigest: z.string(),
    stagedFiles: z.array(GitCommitFileChangeSchema),
    unstagedCount: z.number().int().nonnegative(),
    untrackedCount: z.number().int().nonnegative(),
    conflictedCount: z.number().int().nonnegative(),
    operation: z.string().optional(),
    identity: GitCommitIdentitySchema,
    executableHooks: z.array(z.string()),
    blockers: z.array(IssueSchema),
    warnings: z.array(IssueSchema),
  });
export type GitCommitPreview = z.infer<typeof GitCommitPreviewSchema>;

export const GitCommitRequestSchema = z
  .object({
    confirmationId: z.string().min(1),
    clientRequestId: z.string().min(1).max(128).optional(),
    message: z.string().min(1),
  })
  .strict();
export type GitCommitRequest = z.infer<typeof GitCommitRequestSchema>;

export const GitCommitOutcomeStatusSchema = z.enum([
  "matched",
  "needs_review",
  "uncertain",
]);
export type GitCommitOutcomeStatus = z.infer<
  typeof GitCommitOutcomeStatusSchema
>;

export const GitCommitResultSchema = z.object({
  operationId: z.string().optional(),
  outcome: GitCommitOutcomeStatusSchema,
  commitOid: z.string().optional(),
  parents: z.array(z.string()),
  treeOid: z.string().optional(),
  expectedTreeOid: z.string(),
  subject: z.string().optional(),
  reviewReasons: z.array(z.string()),
});
export type GitCommitResult = z.infer<typeof GitCommitResultSchema>;

export const GitConflictReasonSchema = z.enum([
  "readonly",
  "busy",
  "stale",
  "context_mismatch",
  "no_task_baseline",
  "conflicted",
  "blocked",
  "commit_failed",
  "idempotency_conflict",
  "uncertain",
  "draft_conflict",
]);
export type GitConflictReason = z.infer<typeof GitConflictReasonSchema>;

export const GitConflictSchema = z.object({
  error: z.string(),
  reason: GitConflictReasonSchema,
  currentDraft: GitCommitDraftSchema.optional(),
});
export type GitConflict = z.infer<typeof GitConflictSchema>;

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
  remote: z.string().optional(),
  target: z.string().optional(),
  targetOid: z.string().optional(),
  publish: z.boolean().optional(),
  detached: z.boolean().optional(),
});

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
  parents: z.array(z.string()),
  subject: z.string().optional(),
  reviewReasons: z.array(z.string()),
});
export type GitOperation = z.infer<typeof GitOperationSchema>;

export const GitOperationsSchema = z.object({
  operations: z.array(GitOperationSchema),
});

// ---- Fixed worktree task baseline & single-entry net task review ----

export const GitTaskBaselineResponseSchema = z.object({
  baseline: GitTaskBaselineSchema.nullable(),
});

export const GitPutTaskBaselineRequestSchema = z
  .object({
    baseOid: z.string().optional(),
  })
  .strict();
export type GitPutTaskBaselineRequest = z.infer<
  typeof GitPutTaskBaselineRequestSchema
>;

const GitTaskChangesStatusSchema = z.enum(["ready", "no_task_baseline"]);

export const GitTaskChangesSchema = z.object({
  status: GitTaskChangesStatusSchema,
  reason: z.string().optional(),
  baseline: GitTaskBaselineSchema.nullable(),
  headOid: z.string().optional(),
  headUnborn: z.boolean(),
  commitCount: z.number().int().nonnegative(),
  uncommittedCount: z.number().int().nonnegative(),
  changes: GitChangesSchema,
});
