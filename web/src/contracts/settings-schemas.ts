import { z } from "zod";
import { m } from "$lib/paraglide/messages.js";
import {
  PreviewSettingsSchema,
  SessionIdSchema,
  WorkspaceCapabilitySchema,
} from "./session-schemas";
import { HomeWorktreesSchema, WorktreePinsSchema } from "./workspace-schemas";

const TunnelInfoSchema = z.object({
  enabled: z.boolean(),
  state: z.enum(["off", "starting", "ready", "failed"]),
  named: z.boolean(),
  hostname: z.string(),
  publicUrl: z.string().optional(),
});
export type TunnelInfo = z.infer<typeof TunnelInfoSchema>;

export const TunnelSettingsSchema = z.object({
  enabled: z.boolean(),
  mode: z.enum(["quick", "named"]),
  hostname: z.string(),
  tokenConfigured: z.boolean(),
  source: z.enum(["web", "config"]),
  state: TunnelInfoSchema.shape.state,
  publicUrl: z.string().optional(),
  error: z.string().optional(),
  canEnable: z.boolean(),
  enableBlockedReason: z.string().optional(),
  storageAvailable: z.boolean(),
  storageError: z.string().optional(),
});
export type TunnelSettings = z.infer<typeof TunnelSettingsSchema>;
export const TunnelSettingsUpdateSchema = z.object({
  mode: z.enum(["quick", "named"]),
  hostname: z.string(),
  token: z.string().optional(),
});
export type TunnelSettingsUpdate = z.infer<typeof TunnelSettingsUpdateSchema>;
const TailscaleEntrySchema = z.object({
  host: z.string(),
  port: z.number().int().min(0).max(65535),
  public: z.boolean(),
});
const TailscaleWSLSchema = z.object({
  networkingMode: z.enum(["mirrored", "nat", "unknown"]),
  windowsDetected: z.boolean(),
  windowsRunning: z.boolean(),
  windowsDnsName: z.string().optional(),
});
export const TailscaleStatusSchema = z.object({
  provider: z.enum(["linux", "wsl-native", "windows", "none"]),
  configuredProvider: z.enum(["auto", "linux", "windows"]),
  backendState: z.enum([
    "running",
    "stopped",
    "needs-login",
    "unavailable",
    "unknown",
  ]),
  dnsName: z.string().optional(),
  httpsReady: z.boolean(),
  funnelAvailable: z.enum(["available", "unavailable", "unknown"]),
  mode: z.enum(["off", "serve", "funnel"]),
  port: z.number().int().min(0).max(65535),
  source: z.enum(["config", "web"]),
  storageAvailable: z.boolean(),
  storageError: z.string().optional(),
  canEnable: z.boolean(),
  enableBlockedReason: z.string().optional(),
  state: TunnelInfoSchema.shape.state,
  publicUrl: z.string().optional(),
  blockedReason: z.string().optional(),
  error: z.string().optional(),
  entries: z.array(TailscaleEntrySchema),
  wsl: TailscaleWSLSchema.optional(),
});
export type TailscaleStatus = z.infer<typeof TailscaleStatusSchema>;
export const TailscaleFunnelPorts = [443, 8443, 10000] as const;
export const TailscaleSettingsUpdateSchema = z
  .object({
    mode: TailscaleStatusSchema.shape.mode,
    port: TailscaleStatusSchema.shape.port,
    provider: TailscaleStatusSchema.shape.configuredProvider,
    confirmFunnel: z.boolean().optional(),
  })
  .strict()
  .refine(
    (draft) =>
      draft.mode !== "funnel" ||
      draft.port === 0 ||
      (TailscaleFunnelPorts as readonly number[]).includes(draft.port),
    { error: () => m.s1_schema_funnel_ports() },
  );
export type TailscaleSettingsUpdate = z.infer<
  typeof TailscaleSettingsUpdateSchema
>;
// The web-editable [voice] recognizer; env values are credentials for the
// recognition service.
export const VoiceSettingsSchema = z.object({
  cmd: z.array(z.string()),
  env: z.record(z.string(), z.string()),
  enabled: z.boolean(),
  source: z.enum(["config", "web"]),
  storageAvailable: z.boolean(),
  storageError: z.string().optional(),
});
export type VoiceSettings = z.infer<typeof VoiceSettingsSchema>;
export const VoiceSettingsUpdateSchema = z
  .object({
    cmd: VoiceSettingsSchema.shape.cmd,
    env: VoiceSettingsSchema.shape.env,
  })
  .strict();
export type VoiceSettingsUpdate = z.infer<typeof VoiceSettingsUpdateSchema>;

const AgentUpdateStatusSchema = z.enum([
  "unchecked",
  "up_to_date",
  "update_available",
  "source_unavailable",
  "check_failed",
  "not_installed",
]);
export type AgentUpdateStatus = z.infer<typeof AgentUpdateStatusSchema>;

const agentVersionFields = {
  installed: z.boolean(),
  path: z.string().optional(),
  installedVersion: z.string().optional(),
  latestVersion: z.string().optional(),
  verifiedVersion: z.string().optional(),
  belowVerified: z.boolean().optional(),
  updateStatus: AgentUpdateStatusSchema,
  checkError: z.string().optional(),
  installCommand: z.string().optional(),
  upgradeCommand: z.string().optional(),
  docsUrl: z.url({ protocol: /^https$/ }).optional(),
};

const AgentConnectorStatusSchema = z.object({
  executable: z.string().min(1),
  title: z.string().min(1),
  ...agentVersionFields,
});
export type AgentConnectorStatus = z.infer<typeof AgentConnectorStatusSchema>;

const AgentToolStatusSchema = z.object({
  id: z.string().min(1),
  title: z.string().min(1),
  icon: z.string().optional(),
  kind: z.enum(["hybrid", "terminal"]),
  executable: z.string().min(1),
  executables: z.array(z.string()),
  ...agentVersionFields,
  connector: AgentConnectorStatusSchema.optional(),
});
export type AgentToolStatus = z.infer<typeof AgentToolStatusSchema>;

export const AgentToolsReportSchema = z.object({
  checkedAt: z.iso.datetime({ offset: true }),
  remoteCheckedAt: z.iso.datetime({ offset: true }).optional(),
  shell: z.string(),
  tools: z.array(AgentToolStatusSchema),
});
export type AgentToolsReport = z.infer<typeof AgentToolsReportSchema>;
// An installed version that takes effect after a restart. An unknown future
// launcher falls back to the manual restart method.
const PendingUpdateSchema = z.object({
  installedVersion: z.string().min(1).max(64),
  launcher: z
    .enum(["systemd", "launchd", "windows-tray", "manual"])
    .catch("manual"),
});
export type PendingUpdate = z.infer<typeof PendingUpdateSchema>;
export const ConfigSchema = z.object({
  defaultCwd: z.string(),
  // Older servers do not send the editor's default startup shell.
  defaultShell: z.enum(["bash", "zsh", ""]).default(""),
  // Older servers do not send this display-only field.
  hostname: z.string().default(""),
  workspace: WorkspaceCapabilitySchema,
  // Older servers do not send the runtime tunnel state.
  tunnel: TunnelInfoSchema.optional(),
  // Present only while a different version awaits a restart; older servers
  // never send it.
  pendingUpdate: PendingUpdateSchema.optional(),
  // Whether sessions report listening ports on the server's platform.
  portDetection: z.boolean(),
  // Whether external speech recognition is configured on the server.
  voice: z.boolean().default(false),
  // Whether opportunistic P2P direct upgrade is enabled on the server.
  p2p: z.boolean().default(false),
});

export const VoiceClientMessageSchema = z.object({
  type: z.literal("stop"),
});

export const VoiceServerEventSchema = z.discriminatedUnion("type", [
  z.object({ type: z.literal("ready") }),
  z.object({ type: z.literal("partial"), text: z.string().default("") }),
  z.object({ type: z.literal("final"), text: z.string().default("") }),
  z.object({ type: z.literal("error"), error: z.string() }),
]);
export type VoiceServerEvent = z.infer<typeof VoiceServerEventSchema>;

export const RecentDirsSchema = z
  .array(z.string())
  .transform((dirs) => dirs.slice(0, 10));
export const FontSizeSchema = z.coerce.number().int().min(8).max(20);
export const CodeFontSizeSchema = z.coerce.number().int().min(10).max(24);
export const ThemeSchema = z.enum(["light", "dark"]);
export type Theme = z.infer<typeof ThemeSchema>;
export const TextSettingSchema = z.string();
export const QuickCommandSchema = z.object({
  id: z.string().min(1).max(64),
  name: z.string().max(40),
  command: z
    .string()
    .max(4096)
    .refine((value) => value.trim() !== "", {
      error: () => m.s1_schema_command_required(),
    }),
  enter: z.boolean(),
});
export type QuickCommand = z.infer<typeof QuickCommandSchema>;
export const QuickCommandsSchema = z
  .array(QuickCommandSchema)
  .max(50)
  .refine(
    (commands) =>
      new Set(commands.map((cmd) => cmd.id)).size === commands.length,
    { error: () => m.s1_schema_duplicate_quick_command_id() },
  );
export const NoContentSchema = z.undefined();
export const PushReasonSchema = z.enum([
  "disabled",
  "storage",
  "quick_tunnel",
  "unsupported_origin",
]);
export type PushReason = z.infer<typeof PushReasonSchema>;
export const PushStatusSchema = z.object({
  enabled: z.boolean(),
  available: z.boolean(),
  reason: PushReasonSchema.optional(),
  // Base64url uncompressed P-256 VAPID key, present only when available.
  publicKey: z.string().optional(),
  subscriptions: z.number().int().min(0),
});
export type PushStatus = z.infer<typeof PushStatusSchema>;
export const PushSubscriptionResultSchema = z.object({ id: z.string() });
export const PushSubscriptionRequestSchema = z.object({
  endpoint: z.string(),
  expirationTime: z.number().nullable().optional(),
  keys: z.object({ p256dh: z.string(), auth: z.string() }),
});

// A test notification goes to every device; failures name only the push
// service host, and removed lists subscriptions the service reported expired.
export const PushTestResultSchema = z.object({
  devices: z.number().int().min(0),
  accepted: z.number().int().min(0),
  removed: z.array(z.string()),
  errors: z.array(z.string()),
});
export type PushTestResult = z.infer<typeof PushTestResultSchema>;

export const SessionPinsSchema = z
  .array(SessionIdSchema)
  .max(50)
  .refine((ids) => new Set(ids).size === ids.length);

function validWorktreeMap(
  value: Record<string, string[]>,
  maxTotal: number,
): boolean {
  const entries = Object.entries(value);
  return (
    entries.length <= 200 &&
    entries.reduce((n, [, ids]) => n + ids.length, 0) <= maxTotal &&
    entries.every(
      ([id, ids]) =>
        id.length <= 128 &&
        ids.every((x) => x.length <= 128) &&
        new Set(ids).size === ids.length,
    )
  );
}

// User preferences shared by every authenticated client of this server.
export const SettingsValuesSchema = z.strictObject({
  preview: PreviewSettingsSchema.nullable(),
  codeReader: z
    .strictObject({
      lineNumbers: z.enum(["show", "hide"]),
      diffOverflow: z.enum(["scroll", "wrap"]),
    })
    .nullable(),
  quickCommands: QuickCommandsSchema.nullable(),
  homeWorktrees: HomeWorktreesSchema.refine((value) =>
    validWorktreeMap(value, 2000),
  ).nullable(),
  worktreePins: WorktreePinsSchema.refine((value) =>
    validWorktreeMap(value, 1000),
  )
    .nullable()
    .optional(),
  sessionPins: SessionPinsSchema.nullable().optional(),
  ignoreHighPorts: z.boolean().nullable(),
});
export type SettingsValues = z.infer<typeof SettingsValuesSchema>;
export const SettingsSnapshotSchema = z.strictObject({
  revision: z.number().int().min(0).max(Number.MAX_SAFE_INTEGER),
  updatedAt: z.union([z.literal(""), z.iso.datetime({ offset: true })]),
  values: SettingsValuesSchema,
});
export type SettingsSnapshot = z.infer<typeof SettingsSnapshotSchema>;
const SettingsChangesSchema = SettingsValuesSchema.partial();
export const SettingsEventSchema = z.discriminatedUnion("type", [
  z.strictObject({
    type: z.literal("settings"),
    settings: SettingsSnapshotSchema,
  }),
  z.strictObject({
    type: z.literal("settings.update"),
    revision: z.number().int().positive().max(Number.MAX_SAFE_INTEGER),
    updatedAt: z.iso.datetime({ offset: true }),
    changes: SettingsChangesSchema,
  }),
]);
