import { z } from "zod";

export const PluginIDSchema = z.enum([
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
export type PluginID = z.infer<typeof PluginIDSchema>;

const CapabilityReasonSchema = z.enum([
  "plugin_disabled",
  "configuration_off",
  "unsupported",
  "storage_unavailable",
  "command_unavailable",
  "service_unavailable",
]);

const PluginStatusSchema = z
  .object({
    id: PluginIDSchema,
    enabled: z.boolean(),
    status: z.enum(["disabled", "ready", "unavailable"]),
    reason: CapabilityReasonSchema.optional(),
  })
  .superRefine((plugin, ctx) => {
    if (plugin.enabled === (plugin.status === "disabled")) {
      ctx.addIssue({
        code: "custom",
        path: ["status"],
        message: "Plugin status must agree with its enabled selection",
      });
    }
    if (plugin.status === "ready" && plugin.reason !== undefined) {
      ctx.addIssue({
        code: "custom",
        path: ["reason"],
        message: "Ready plugins cannot have an unavailable reason",
      });
    }
    if (plugin.status === "unavailable" && plugin.reason === undefined) {
      ctx.addIssue({
        code: "custom",
        path: ["reason"],
        message: "Unavailable plugins require a controlled reason",
      });
    }
    if (
      plugin.status === "disabled" &&
      plugin.reason !== undefined &&
      plugin.reason !== "plugin_disabled"
    ) {
      ctx.addIssue({
        code: "custom",
        path: ["reason"],
        message: "Disabled plugins can only report plugin_disabled",
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
    version: z.literal(1),
    plugins: z.array(PluginStatusSchema).length(PluginIDSchema.options.length),
    capabilities: z.array(CapabilitySchema),
  })
  .superRefine((snapshot, ctx) => {
    const plugins = new Set<string>();
    for (const [index, plugin] of snapshot.plugins.entries()) {
      if (plugins.has(plugin.id)) {
        ctx.addIssue({
          code: "custom",
          path: ["plugins", index, "id"],
          message: "Duplicate plugin ID",
        });
      }
      plugins.add(plugin.id);
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
