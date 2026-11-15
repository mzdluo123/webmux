import { z } from "zod";
import { IssueSchema } from "./branch-schemas";
import { GitStashExecuteRequestSchema } from "./git-stash-schemas";
import { SessionInfoSchema } from "./session-schemas";
import {
  BranchesSchema,
  RemoteSchema,
  RepositorySchema,
  TagSchema,
  WorktreeSchema,
} from "./workspace-schemas";

export const GitBranchRenamePreviewRequestSchema = z
  .object({
    oldName: z.string().min(1),
    newName: z.string().min(1),
  })
  .strict();
export type GitBranchRenamePreviewRequest = z.infer<
  typeof GitBranchRenamePreviewRequestSchema
>;

export const GitBranchRenamePreviewSchema = z.object({
  confirmationId: z.string(),
  expiresAt: z.iso.datetime({ offset: true }).nullable(),
  repositoryId: z.string(),
  oldName: z.string(),
  newName: z.string(),
  headOid: z.string(),
  subject: z.string(),
  upstream: z.string().optional(),
  affectedWorktrees: z.array(WorktreeSchema),
  sessions: z.array(SessionInfoSchema),
  blockers: z.array(IssueSchema),
  warnings: z.array(IssueSchema),
});
export type GitBranchRenamePreview = z.infer<
  typeof GitBranchRenamePreviewSchema
>;

export const GitBranchRenameRequestSchema = GitStashExecuteRequestSchema;
export type GitBranchRenameRequest = z.infer<
  typeof GitBranchRenameRequestSchema
>;

export const GitBranchUpstreamRequestSchema = z
  .object({
    branch: z.string().min(1),
    upstream: z.string().optional(),
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitBranchUpstreamRequest = z.infer<
  typeof GitBranchUpstreamRequestSchema
>;

export const GitBranchMutationResultSchema = z.object({
  operationId: z.string().optional(),
  branch: z.string(),
  upstream: z.string().optional(),
  repository: RepositorySchema,
  branches: BranchesSchema,
});

export const GitTagCreateRequestSchema = z
  .object({
    name: z.string().min(1),
    targetOid: z.string().min(1),
    annotated: z.boolean(),
    message: z.string().optional(),
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitTagCreateRequest = z.infer<typeof GitTagCreateRequestSchema>;

export const GitTagCreateResultSchema = z.object({
  operationId: z.string().optional(),
  tag: TagSchema,
  branches: BranchesSchema,
});

export const GitTagDeleteRequestSchema = z
  .object({
    name: z.string().min(1),
    expectedTargetOid: z.string().optional(),
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitTagDeleteRequest = z.infer<typeof GitTagDeleteRequestSchema>;

export const GitTagDeleteResultSchema = z.object({
  operationId: z.string().optional(),
  name: z.string(),
  targetOid: z.string(),
  recoveryCommand: z.string(),
  branches: BranchesSchema,
});

export const GitRemoteAddRequestSchema = z
  .object({
    name: z.string().min(1),
    fetchUrl: z.string().min(1),
    pushUrl: z.string().optional(),
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitRemoteAddRequest = z.infer<typeof GitRemoteAddRequestSchema>;

export const GitRemoteUpdateRequestSchema = z
  .object({
    fetchUrl: z.string().optional(),
    pushUrl: z.string().optional(),
    clearPushUrl: z.boolean().optional(),
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitRemoteUpdateRequest = z.infer<
  typeof GitRemoteUpdateRequestSchema
>;

export const GitRemoteRemoveRequestSchema = z
  .object({
    clientRequestId: z.string().min(1).max(128).optional(),
  })
  .strict();
export type GitRemoteRemoveRequest = z.infer<
  typeof GitRemoteRemoveRequestSchema
>;

export const GitRemoteMutationResultSchema = z.object({
  operationId: z.string().optional(),
  remotes: z.array(RemoteSchema),
  repository: RepositorySchema,
});

const GitFileHistoryRowSchema = z.object({
  oid: z.string(),
  shortOid: z.string(),
  parents: z.array(z.string()),
  comparedParentOid: z.string().optional(),
  subject: z.string(),
  authorName: z.string(),
  authorEmail: z.string(),
  authorAt: z.string(),
  status: z.string(),
  path: z.string(),
  oldPath: z.string().optional(),
});
export type GitFileHistoryRow = z.infer<typeof GitFileHistoryRowSchema>;

export const GitFileHistoryPageSchema = z.object({
  status: z.enum(["ready", "unborn", "empty"]),
  path: z.string(),
  startOid: z.string().optional(),
  commits: z.array(GitFileHistoryRowSchema),
  nextCursor: z.string().optional(),
});

const GitBlameCommitSchema = z.object({
  oid: z.string(),
  shortOid: z.string(),
  summary: z.string(),
  authorName: z.string(),
  authorEmail: z.string(),
  authorAt: z.string(),
  previousOid: z.string().optional(),
  previousPath: z.string().optional(),
  boundary: z.boolean().optional(),
});
export type GitBlameCommit = z.infer<typeof GitBlameCommitSchema>;

const GitBlameLineSchema = z.object({
  lineNumber: z.number().int().positive(),
  origLine: z.number().int().positive(),
  commitOid: z.string(),
  shortOid: z.string(),
  origPath: z.string().optional(),
  previousOid: z.string().optional(),
  previousPath: z.string().optional(),
  text: z.string(),
});

export const GitBlameResultSchema = z.object({
  commitOid: z.string(),
  shortOid: z.string(),
  path: z.string(),
  blobOid: z.string().optional(),
  mode: z.string().optional(),
  kind: z.enum(["text", "binary", "too_large", "unsupported"]),
  degradeReason: z.string().optional(),
  totalLines: z.number().int().nonnegative(),
  commits: z.array(GitBlameCommitSchema),
  lines: z.array(GitBlameLineSchema),
});
export type GitBlameResult = z.infer<typeof GitBlameResultSchema>;
