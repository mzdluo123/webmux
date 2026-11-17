import { z } from "zod";
export const handoffMaxPages = 8;
const HandoffLocationSchema = z.object({
  cwd: z.string(),
  repositoryId: z.string(),
  worktreeId: z.string(),
  newWorktreeBase: z.string(),
  newWorktreeBranch: z.string(),
});
export const HandoffPreviewRequestSchema = z
  .object({
    format: z.enum(["image", "text"]).default("image"),
    maxPages: z
      .number()
      .int()
      .min(0)
      .max(handoffMaxPages)
      .default(handoffMaxPages),
    configId: z.string(),
    targetSessionId: z.string(),
    agentConfig: z.record(z.string(), z.string()),
    userText: z.string(),
    location: HandoffLocationSchema,
  })
  .strict();
const HandoffBudgetSchema = z.object({
  window: z.number(),
  nativeUsed: z.number(),
  current: z.number(),
  reserve: z.number(),
  history: z.number(),
  used: z.number(),
  source: z.string(),
  overBudget: z.boolean(),
});
export const HandoffPreviewSchema = z.object({
  id: z.string(),
  format: z.enum(["image", "text"]),
  pages: z.array(
    z.object({
      index: z.number().int().nonnegative(),
      mimeType: z.literal("image/png"),
      data: z.string(),
      sha256: z.string(),
      width: z.number().int().positive(),
      height: z.number().int().positive(),
      bytes: z.number().int().nonnegative(),
    }),
  ),
  imageBudget: z.object({
    maxPages: z.number().int(),
    pages: z.number().int(),
    bytes: z.number().int(),
    maxBytes: z.number().int(),
    maxRequestBytes: z.number().int(),
    projectedRunes: z.number().int().nonnegative(),
    retainedRunes: z.number().int().nonnegative(),
    estimatedTokens: z.number().int().nonnegative(),
    tokenEstimateKnown: z.boolean(),
    tokenBasis: z.string(),
    shape: z.string(),
  }),
  blockedReason: z.string(),
  capability: z.enum(["supported", "unsupported", "pending"]),
  historyText: z.string(),
  instanceId: z.string(),
  operationId: z.string().min(1),
  sourceSessionId: z.string(),
  snapshotId: z.string(),
  text: z.string(),
  userText: z.string(),
  request: HandoffPreviewRequestSchema,
  createdAt: z.iso.datetime({ offset: true }),
  expiresAt: z.iso.datetime({ offset: true }),
  selectedCount: z.number().int(),
  omittedCount: z.number().int(),
  gapCount: z.number().int(),
  budget: HandoffBudgetSchema,
});
export const HandoffOperationSchema = z.object({
  id: z.string(),
  previewId: z.string(),
  instanceId: z.string(),
  state: z.enum(["preparing", "sending", "succeeded", "failed", "unknown"]),
  targetSessionId: z.string(),
  error: z.string(),
  worktreeId: z.string(),
  worktreePath: z.string(),
  startedAt: z.iso.datetime({ offset: true }),
  finishedAt: z.iso.datetime({ offset: true }).nullable(),
});
export const HandoffSendRequestSchema = z.object({
  operationId: z.string().min(1),
});
export type HandoffLocation = z.infer<typeof HandoffLocationSchema>;
export type HandoffPreview = z.infer<typeof HandoffPreviewSchema>;
export type HandoffOperation = z.infer<typeof HandoffOperationSchema>;
