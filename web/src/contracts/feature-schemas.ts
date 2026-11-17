import { z } from "zod";

export const FeatureIDSchema = z.enum([
  "launch",
  "workspace",
  "code",
  "files",
  "chat",
  "notifications",
  "quick-commands",
  "voice",
  "forward",
  "cloudflare",
  "tailscale",
  "p2p",
  "mcp",
  "immersive",
]);
export type FeatureID = z.infer<typeof FeatureIDSchema>;

const CapabilityReasonSchema = z.enum([
  "feature_disabled",
  "configuration_off",
  "unsupported",
  "storage_unavailable",
  "command_unavailable",
  "service_unavailable",
]);

const FeatureStatusSchema = z
  .object({
    id: FeatureIDSchema,
    enabled: z.boolean(),
    status: z.enum(["disabled", "ready", "unavailable"]),
    reason: CapabilityReasonSchema.optional(),
  })
  .superRefine((feature, ctx) => {
    if (feature.enabled === (feature.status === "disabled")) {
      ctx.addIssue({
        code: "custom",
        path: ["status"],
        message: "Feature status must agree with its enabled selection",
      });
    }
    if (feature.status === "ready" && feature.reason !== undefined) {
      ctx.addIssue({
        code: "custom",
        path: ["reason"],
        message: "Ready features cannot have an unavailable reason",
      });
    }
    if (feature.status === "unavailable" && feature.reason === undefined) {
      ctx.addIssue({
        code: "custom",
        path: ["reason"],
        message: "Unavailable features require a controlled reason",
      });
    }
    if (
      feature.status === "disabled" &&
      feature.reason !== undefined &&
      feature.reason !== "feature_disabled"
    ) {
      ctx.addIssue({
        code: "custom",
        path: ["reason"],
        message: "Disabled features can only report feature_disabled",
      });
    }
  });

const CapabilitySchema = z
  .object({
    id: z.string().min(1),
    available: z.boolean(),
    reason: CapabilityReasonSchema.optional(),
  })
  .superRefine((capability, ctx) => {
    if (capability.available && capability.reason !== undefined) {
      ctx.addIssue({
        code: "custom",
        path: ["reason"],
        message: "Available capabilities cannot have an unavailable reason",
      });
    }
    if (!capability.available && capability.reason === undefined) {
      ctx.addIssue({
        code: "custom",
        path: ["reason"],
        message: "Unavailable capabilities require a controlled reason",
      });
    }
  });

export const CapabilitiesSchema = z
  .object({
    version: z.literal(2),
    features: z
      .array(FeatureStatusSchema)
      .length(FeatureIDSchema.options.length),
    capabilities: z.array(CapabilitySchema),
  })
  .superRefine((snapshot, ctx) => {
    const features = new Set<string>();
    for (const [index, feature] of snapshot.features.entries()) {
      if (features.has(feature.id)) {
        ctx.addIssue({
          code: "custom",
          path: ["features", index, "id"],
          message: "Duplicate feature ID",
        });
      }
      features.add(feature.id);
    }
    const capabilities = new Set<string>();
    for (const [index, capability] of snapshot.capabilities.entries()) {
      if (capabilities.has(capability.id)) {
        ctx.addIssue({
          code: "custom",
          path: ["capabilities", index, "id"],
          message: "Duplicate capability ID",
        });
      }
      capabilities.add(capability.id);
    }
    if (!capabilities.has("terminal.pty")) {
      ctx.addIssue({
        code: "custom",
        path: ["capabilities"],
        message: "The terminal.pty core capability is required",
      });
    }
  });
export type Capabilities = z.infer<typeof CapabilitiesSchema>;
