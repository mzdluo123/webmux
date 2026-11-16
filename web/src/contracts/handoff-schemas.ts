import { z } from "zod";
const HandoffLocationSchema = z.object({
  cwd: z.string(),
  repositoryId: z.string(),
  worktreeId: z.string(),
  newWorktreeBase: z.string(),
  newWorktreeBranch: z.string(),
});
export const HandoffPreviewRequestSchema = z
  .object({
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
