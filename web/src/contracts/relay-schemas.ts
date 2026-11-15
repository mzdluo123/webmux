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
  state: z.enum(["off", "connecting", "ready", "degraded", "failed"]),
  transport: z.literal("wss"),
  bulkReady: z.boolean(),
  controlReady: z.boolean(),
  readyConnections: z.number().int().min(0).max(2),
  publicUrl: PublicURLSchema,
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

export const RelayEnrollmentSchema = z.object({
  version: z.literal(1),
  handle: z.string().regex(/^[A-Za-z0-9_-]{43}$/),
  userCode: z.string().min(1).max(64),
  verificationUrl: z
    .string()
    .url()
    .refine((value) => {
      const url = new URL(value);
      return (
        url.protocol === "https:" &&
        url.hostname === "github.com" &&
        !url.username &&
        !url.password &&
        !url.search &&
        !url.hash
      );
    }),
  expiresAt: z.number().int().positive(),
  pollIntervalSeconds: z.number().int().min(1).max(60),
});
export type RelayEnrollment = z.infer<typeof RelayEnrollmentSchema>;
export const RelayUpdateSchema = z
  .object({
    enabled: z.boolean(),
    confirmPublic: z.boolean().optional(),
  })
  .strict();
export const RelayEnrollmentRequestSchema = z
  .object({
    rebind: z.boolean(),
    confirmPublic: z.literal(true),
  })
  .strict();
export const RelayPollRequestSchema = z
  .object({
    handle: z.string().regex(/^[A-Za-z0-9_-]{43}$/),
  })
  .strict();
const RelayPendingSchema = z.object({ error: z.literal("pending") });
export const RelayPollResultSchema = z.union([
  RelayStatusSchema,
  RelayPendingSchema,
]);
export const RelayAuthSessionSchema = z
  .object({
    authenticated: z.boolean(),
    csrfToken: z.string(),
    expiresAt: z.number().int().min(0),
  })
  .refine(
    (value) =>
      !value.authenticated ||
      (value.csrfToken.length > 0 && value.expiresAt > 0),
  );
