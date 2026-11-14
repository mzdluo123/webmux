import { z } from "zod";
import { IssueSchema } from "./branch-schemas";
import {
  GitCommitFileChangeSchema,
  GitWorktreeConfirmationEnvelopeSchema,
} from "./git-api-schemas";
import { GitInProgressOperationSchema } from "./git-conflict-schemas";
import { SessionInfoSchema } from "./session-schemas";
import { WorktreeSchema } from "./workspace-schemas";

const GitStashEntrySchema = z.object({
  stashRef: z.string(),
  index: z.number().int().nonnegative(),
  oid: z.string(),
  shortOid: z.string(),
  baseOid: z.string().optional(),
  indexOid: z.string().optional(),
  untrackedOid: z.string().optional(),
  parents: z.array(z.string()),
  hasUntracked: z.boolean(),
  branch: z.string().optional(),
  summary: z.string(),
  createdAt: z.string(),
});
export type GitStashEntry = z.infer<typeof GitStashEntrySchema>;

export const GitStashListSchema = z.object({
  stashes: z.array(GitStashEntrySchema),
});

const GitStashFileChangeSchema = GitCommitFileChangeSchema.extend({
  untracked: z.boolean(),
});
export type GitStashFileChange = z.infer<typeof GitStashFileChangeSchema>;

export const GitStashDetailSchema = z.object({
  entry: GitStashEntrySchema,
  authorName: z.string(),
  authorEmail: z.string(),
  trackedCount: z.number().int().nonnegative(),
  untrackedCount: z.number().int().nonnegative(),
  files: z.array(GitStashFileChangeSchema),
});
export type GitStashDetail = z.infer<typeof GitStashDetailSchema>;

export const GitStashCreatePreviewRequestSchema = z
  .object({
    message: z.string().optional(),
    includeUntracked: z.boolean().optional(),
    keepIndex: z.boolean().optional(),
  })
  .strict();
export type GitStashCreatePreviewRequest = z.infer<
  typeof GitStashCreatePreviewRequestSchema
>;

export const GitStashCreatePreviewSchema =
  GitWorktreeConfirmationEnvelopeSchema.extend({
    currentBranch: z.string().optional(),
    detached: z.boolean(),
    headOid: z.string().optional(),
    headSubject: z.string().optional(),
    message: z.string().optional(),
    includeUntracked: z.boolean(),
    keepIndex: z.boolean(),
    stagedCount: z.number().int().nonnegative(),
    unstagedCount: z.number().int().nonnegative(),
    untrackedCount: z.number().int().nonnegative(),
    affectedPaths: z.array(z.string()),
    sessions: z.array(SessionInfoSchema),
    blockers: z.array(IssueSchema),
    warnings: z.array(IssueSchema),
  });
export type GitStashCreatePreview = z.infer<typeof GitStashCreatePreviewSchema>;

export const GitStashExecuteRequestSchema = z
  .object({
    confirmationId: z.string().min(1),
    acknowledgeSessions: z.boolean().optional(),
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitStashExecuteRequest = z.infer<
  typeof GitStashExecuteRequestSchema
>;

export const GitStashCreateResultSchema = z.object({
  operationId: z.string().optional(),
  entry: GitStashEntrySchema,
  worktree: WorktreeSchema,
});

const GitStashApplyModeSchema = z.enum(["apply", "pop"]);

export const GitStashApplyPreviewRequestSchema = z
  .object({
    mode: GitStashApplyModeSchema,
    stashRef: z.string().min(1),
    stashOid: z.string().min(1),
    reinstateIndex: z.boolean().optional(),
  })
  .strict();
export type GitStashApplyPreviewRequest = z.infer<
  typeof GitStashApplyPreviewRequestSchema
>;

export const GitStashApplyPreviewSchema =
  GitWorktreeConfirmationEnvelopeSchema.extend({
    mode: GitStashApplyModeSchema,
    entry: GitStashEntrySchema,
    reinstateIndex: z.boolean(),
    currentBranch: z.string().optional(),
    detached: z.boolean(),
    headOid: z.string().optional(),
    headSubject: z.string().optional(),
    stagedCount: z.number().int().nonnegative(),
    unstagedCount: z.number().int().nonnegative(),
    untrackedCount: z.number().int().nonnegative(),
    overlappingPaths: z.array(z.string()),
    sessions: z.array(SessionInfoSchema),
    blockers: z.array(IssueSchema),
    warnings: z.array(IssueSchema),
  });
export type GitStashApplyPreview = z.infer<typeof GitStashApplyPreviewSchema>;

export const GitStashApplyResultSchema = z.object({
  operationId: z.string().optional(),
  kind: z.string(),
  outcome: z.enum(["completed", "conflicted"]),
  stashRef: z.string(),
  stashOid: z.string(),
  stashDropped: z.boolean(),
  stashPreserved: z.boolean(),
  summary: z.string(),
  inProgress: GitInProgressOperationSchema,
  worktree: WorktreeSchema,
});

export const GitStashDropPreviewRequestSchema = z
  .object({
    stashRef: z.string().min(1),
    stashOid: z.string().min(1),
  })
  .strict();
export type GitStashDropPreviewRequest = z.infer<
  typeof GitStashDropPreviewRequestSchema
>;

export const GitStashDropPreviewSchema = z.object({
  confirmationId: z.string(),
  expiresAt: z.iso.datetime({ offset: true }).nullable(),
  repositoryId: z.string(),
  entry: GitStashEntrySchema,
  fileCount: z.number().int().nonnegative(),
  recoveryCommand: z.string(),
  blockers: z.array(IssueSchema),
  warnings: z.array(IssueSchema),
});
export type GitStashDropPreview = z.infer<typeof GitStashDropPreviewSchema>;

export const GitStashDropRequestSchema = z
  .object({
    confirmationId: z.string().min(1),
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitStashDropRequest = z.infer<typeof GitStashDropRequestSchema>;

export const GitStashDropResultSchema = z.object({
  operationId: z.string().optional(),
  stashRef: z.string(),
  stashOid: z.string(),
  recoveryCommand: z.string(),
  stashes: z.array(GitStashEntrySchema),
});
