import { z } from "zod";
export const HistoryCursorSchema = z
  .string()
  .regex(/^(0|[1-9][0-9]{0,18})$/)
  .refine(
    (value) =>
      /^(0|[1-9][0-9]{0,18})$/.test(value) &&
      BigInt(value) <= 9223372036854775807n,
  );
const nativeFileSchema = z.object({
  agent: z.string(),
  nativeId: z.string(),
  branch: z.string(),
  path: z.string(),
});

const nativeRecordSchema = z.object({
  id: z.string().min(1),
  nativeId: z.string(),
  parentId: z.string(),
  hash: z.string().min(1),
  kind: z.string(),
  role: z.string(),
  status: z.string(),
  text: z.string(),
  callId: z.string(),
  nativeCallId: z.string(),
  toolName: z.string(),
  position: HistoryCursorSchema,
  offset: HistoryCursorSchema,
  end: HistoryCursorSchema,
  byteLength: HistoryCursorSchema,
  nextTextOffset: HistoryCursorSchema,
  truncated: z.boolean(),
  attachments: z.array(
    z.object({
      name: z.string(),
      mimeType: z.string(),
      reference: z.string(),
      size: z.number().int(),
    }),
  ),
});

const nativeRangeSchema = z.object({
  sourceId: z.string(),
  field: z.string(),
  start: z.number().int().nonnegative(),
  end: z.number().int().nonnegative(),
});

export const NativePreviewSchema = z.object({
  gaps: z.array(z.string()),
  id: z.string(),
  version: z.string(),
  currentTask: z.string(),
  segments: z.array(
    z.object({
      sourceId: z.string(),
      text: z.string(),
      role: z.string(),
      callId: z.string(),
      toolName: z.string(),
      status: z.string(),
      sources: z.array(nativeRangeSchema),
    }),
  ),
  omissions: z.array(
    z.object({
      range: nativeRangeSchema,
      reason: z.string(),
      originalRunes: z.number().int(),
    }),
  ),
  pages: z.array(
    z.object({ png: z.string(), ranges: z.array(nativeRangeSchema) }),
  ),
  uncovered: z.array(nativeRangeSchema),
  missingGlyphCount: z.number().int().nonnegative(),
  problem: z.string(),
});

export const NativeResultSchema = z.object({
  expiresAt: z.string(),
  agent: z.string(),
  nativeId: z.string(),
  branch: z.string(),
  format: z.string(),
  boundary: z.string(),
  contentHash: z.string(),
  truncated: z.boolean(),
  more: z.boolean(),
  nextPosition: HistoryCursorSchema,
  nextTextOffset: HistoryCursorSchema,
  recordCount: z.number().int().nonnegative(),
  files: z.array(nativeFileSchema),
  snapshotId: z.string(),
  parser: z.string(),
  pending: z.boolean(),
  gaps: z.array(z.string()),
  records: z.array(nativeRecordSchema),
});
