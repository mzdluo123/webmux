import { z } from "zod";
import { m } from "$lib/paraglide/messages.js";

const CounterSchema = z
  .string()
  .max(20)
  .regex(/^(0|[1-9][0-9]*)$/);
const PublicURLSchema = z.string().refine(
  (value) => {
    if (!value) return true;
    try {
      const url = new URL(value);
      return (
        url.protocol === "https:" &&
        !url.username &&
        !url.password &&
        !url.search &&
        !url.hash &&
        url.pathname === "/" &&
        !url.hostname.includes(":") &&
        !/^\d+(\.\d+){3}$/.test(url.hostname)
      );
    } catch {
      return false;
    }
  },
  { error: () => m.relay_invalid_public_origin() },
);

export const RelayStatusSchema = z.object({
  operation: z.object({
    kind: z.enum(["idle", "enable", "disable", "reconnect", "delete"]),
    phase: z.enum(["idle", "submitting", "completed", "failed", "uncertain"]),
    error: z.string(),
    deleted: z.boolean(),
    accessClosed: z.boolean(),
  }),
  enabled: z.boolean(),
  paired: z.boolean(),
  ownerId: z
    .string()
    .max(20)
    .regex(/^(|[1-9][0-9]*)$/),
  ownerLogin: z
    .string()
    .max(39)
    .regex(/^[A-Za-z0-9-]*$/),
  appliedRevision: CounterSchema,
  readinessError: z.string(),
  pairing: z.boolean(),
  state: z.enum(["off", "connecting", "ready", "failed"]),
  transport: z.literal("wss"),
  controlReady: z.boolean(),
  dataReady: z.boolean(),
  publicUrl: PublicURLSchema.optional(),
  boundPublicUrl: PublicURLSchema,
  hybridSupported: z.boolean(),
  hybridUnavailableReason: z.enum([
    "",
    "p2p-feature-unavailable",
    "p2p-build-unavailable",
  ]),
  error: z.string(),
  reconnectReason: z.string(),
  tcpAlgorithm: z.string(),
  storageAvailable: z.boolean(),
  tokenStrong: z.boolean(),
  controlEndpoint: z.string(),
  controlSource: z.enum(["toml", "default", "web", "safety"]),
  controlReachability: z.enum(["unknown", "reachable", "unavailable"]),
  dataReachability: z.enum([
    "unassigned",
    "unknown",
    "reachable",
    "unavailable",
  ]),
  connectorUrl: z.string(),
  dataOrigin: z.string(),
  assignmentRevision: CounterSchema,
  configRevision: CounterSchema,
  nodeId: z
    .string()
    .max(64)
    .regex(/^[A-Za-z0-9_-]*$/),
  capabilities: z.array(z.enum(["wss", "turn"])),
  assignmentSource: z.enum(["unassigned", "authenticated-control"]),
  sessions: z.array(
    z.object({
      id: z.string(),
      path: z.enum(["wss", "webrtc-direct", "webrtc-turn"]),
      candidateType: z.string(),
      transport: z.string(),
      reason: z.string(),
    }),
  ),
});
export type RelayStatus = z.infer<typeof RelayStatusSchema>;

export const RelayPairingSchema = z.object({
  phase: z.enum([
    "idle",
    "preparing",
    "waiting",
    "retrying",
    "uncertain",
    "cancelling",
    "cancelled",
    "paired",
    "denied",
    "expired",
    "interrupted",
    "failed",
  ]),
  userCode: z.string().max(64),
  verificationUrl: z.string().refine((value) => {
    if (!value) return true;
    try {
      const url = new URL(value);
      return (
        url.protocol === "https:" &&
        url.hostname === "github.com" &&
        !url.username &&
        !url.password &&
        !url.search &&
        !url.hash
      );
    } catch {
      return false;
    }
  }),
  expiresAt: z.number().int().nonnegative(),
  pollIntervalSeconds: z.number().int().nonnegative(),
  retryAt: z.number().int().nonnegative(),
  error: z.string(),
});
export type RelayPairing = z.infer<typeof RelayPairingSchema>;
export const RelayUpdateSchema = z
  .object({
    enabled: z.boolean(),
    confirmPublic: z.boolean().optional(),
  })
  .strict();
export const RelayEnrollmentRequestSchema = z
  .object({
    confirmPublic: z.boolean(),
  })
  .strict();
export const RelayAuthReasonSchema = z.enum([
  "reconnecting",
  "expired",
  "revoked",
  "unauthenticated",
]);
export type RelayAuthReason = z.infer<typeof RelayAuthReasonSchema>;
export const RelayAuthSessionSchema = z
  .object({
    authenticated: z.boolean(),
    csrfToken: z.string(),
    expiresAt: z.number().int().min(0),
    relay: z.boolean().optional(),
    reason: RelayAuthReasonSchema.optional(),
  })
  .refine(
    (value) =>
      !value.authenticated ||
      (value.csrfToken.length > 0 && value.expiresAt > 0),
  );
