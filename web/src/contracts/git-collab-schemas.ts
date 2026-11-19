import { z } from "zod";

// ---- Server-synced commit message drafts & content-bound review marks (#548) ----

export const GitCommitDraftSchema = z.object({
  repositoryId: z.string(),
  worktreeId: z.string(),
  subject: z.string(),
  body: z.string(),
  revision: z.number().int().nonnegative(),
  updatedAt: z.iso.datetime({ offset: true }).nullable(),
});
export type GitCommitDraft = z.infer<typeof GitCommitDraftSchema>;

export const GitPutCommitDraftRequestSchema = z
  .object({
    subject: z.string(),
    body: z.string(),
    baseRevision: z.number().int().nonnegative(),
  })
  .strict();
export type GitPutCommitDraftRequest = z.infer<
  typeof GitPutCommitDraftRequestSchema
>;

const GitReviewScopeParamSchema = z.enum(["uncommitted", "task"]);
export type GitReviewScopeParam = z.infer<typeof GitReviewScopeParamSchema>;

const GitReviewAreaSchema = z.enum([
  "staged",
  "unstaged",
  "untracked",
  "conflicted",
  "task",
]);
export type GitReviewArea = z.infer<typeof GitReviewAreaSchema>;

const GitReviewVersionFields = {
  oldPath: z.string().optional(),
  baseMode: z.string().optional(),
  baseIdentity: z.string().optional(),
  targetMode: z.string().optional(),
  targetIdentity: z.string().optional(),
};

const GitReviewMarkIdentitySchema = z
  .object({
    scope: GitReviewScopeParamSchema,
    area: GitReviewAreaSchema,
    path: z.string().min(1),
    ...GitReviewVersionFields,
  })
  .strict();
export type GitReviewMarkIdentity = z.infer<typeof GitReviewMarkIdentitySchema>;

const GitReviewFileStateSchema = z.object({
  scope: GitReviewScopeParamSchema,
  area: GitReviewAreaSchema,
  path: z.string(),
  status: z.string(),
  ...GitReviewVersionFields,
  reviewed: z.boolean(),
  reviewedAt: z.iso.datetime({ offset: true }).nullable(),
});
export type GitReviewFileState = z.infer<typeof GitReviewFileStateSchema>;

export const GitReviewsResponseSchema = z.object({
  scope: GitReviewScopeParamSchema,
  status: z.enum(["ready", "no_task_baseline"]),
  reason: z.string().optional(),
  totalFiles: z.number().int().nonnegative(),
  reviewedFiles: z.number().int().nonnegative(),
  files: z.array(GitReviewFileStateSchema),
  applied: z.boolean(),
  staleRejected: z.boolean().optional(),
  rejectedReason: z.string().optional(),
});
export type GitReviewsResponse = z.infer<typeof GitReviewsResponseSchema>;

export const GitPutReviewsRequestSchema = z
  .object({
    scope: GitReviewScopeParamSchema,
    action: z.enum(["mark", "unmark", "clear"]),
    item: GitReviewMarkIdentitySchema.optional(),
  })
  .strict();
export type GitPutReviewsRequest = z.infer<typeof GitPutReviewsRequestSchema>;
