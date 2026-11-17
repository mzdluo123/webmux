import type { z } from "zod";
import { api } from "./api";
import type { UpstreamChoice } from "../contracts/worktree-upstream-schemas";
import {
  SessionInfoSchema,
  HandoffPreviewRequestSchema,
  HandoffPreviewSchema,
  HandoffOperationSchema,
  HandoffSendRequestSchema,
  NativeResultSchema,
} from "../contracts/schemas";
export const threadAPI = {
  source: (id: string, signal?: AbortSignal) =>
    api(
      `/sessions/${encodeURIComponent(id)}`,
      SessionInfoSchema,
      "GET",
      undefined,
      signal,
    ),
  preview: (
    source: string,
    body: z.infer<typeof HandoffPreviewRequestSchema>,
    signal?: AbortSignal,
  ) =>
    api(
      `/sessions/${encodeURIComponent(source)}/handoff-previews`,
      HandoffPreviewSchema,
      "POST",
      body,
      signal,
      HandoffPreviewRequestSchema,
    ),
  get: (id: string, signal?: AbortSignal) =>
    api(
      `/handoff-previews/${encodeURIComponent(id)}`,
      HandoffPreviewSchema,
      "GET",
      undefined,
      signal,
    ),
  send: (
    id: string,
    operationId: string,
    signal?: AbortSignal,
    upstreamChoice?: UpstreamChoice,
  ) =>
    api(
      `/handoff-previews/${encodeURIComponent(id)}/send`,
      HandoffOperationSchema,
      "POST",
      { operationId, ...(upstreamChoice ? { upstreamChoice } : {}) },
      signal,
      HandoffSendRequestSchema,
    ),
  delivery: (id: string, signal?: AbortSignal) =>
    api(
      `/handoff-operations/${encodeURIComponent(id)}`,
      HandoffOperationSchema,
      "GET",
      undefined,
      signal,
    ),
  cancel: (id: string, signal?: AbortSignal) =>
    api(
      `/handoff-operations/${encodeURIComponent(id)}/cancel`,
      HandoffOperationSchema,
      "POST",
      undefined,
      signal,
    ),
  history: (
    source: string,
    snapshotId: string,
    after = "0",
    offset = "0",
    signal?: AbortSignal,
  ) =>
    api(
      `/sessions/${encodeURIComponent(source)}/native-history?snapshotId=${encodeURIComponent(snapshotId)}&afterPosition=${after}&textOffset=${offset}`,
      NativeResultSchema,
      "GET",
      undefined,
      signal,
    ),
};
