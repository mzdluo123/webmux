import { z } from "zod";

export const UpstreamChoiceSchema = z
  .object({
    confirmationId: z.string().min(1),
    useUpstream: z.boolean(),
  })
  .strict();

export const WorktreeUpstreamSchema = z.object({
  confirmationId: z.string().min(1),
  branch: z.string(),
  localHead: z.string(),
  upstream: z.string(),
  upstreamHead: z.string(),
  warning: z.string(),
  canUseUpstream: z.boolean(),
});

export const WorktreeUpstreamErrorSchema = z.object({
  error: z.string(),
  reason: z.literal("worktree_upstream_confirmation"),
  worktreeUpstream: WorktreeUpstreamSchema,
});

export type UpstreamChoice = z.infer<typeof UpstreamChoiceSchema>;
export type WorktreeUpstream = z.infer<typeof WorktreeUpstreamSchema>;
