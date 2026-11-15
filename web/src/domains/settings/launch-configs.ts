import * as m from "$lib/paraglide/messages.js";
import { errorMessage } from "../../platform/error";
import { joinArgs, parseArgs } from "../../platform/api";
import type {
  AttentionMode,
  AttentionSettings,
  LaunchConfigSummary,
  PreviewSettings,
} from "../../contracts/schemas";

// Client-side mirror of the server limits; the server stays authoritative.
export const LAUNCH_CONFIGS_LIMIT = 100;
const LAUNCH_NAME_MAX = 64;
const LAUNCH_ARGS_MAX = 128;
export const LAUNCH_ENV_MAX = 64;
const LAUNCH_VALUE_BYTES = 8192;
export const LAUNCH_PREVIEW_LINES_MIN = 1;
export const LAUNCH_PREVIEW_LINES_MAX = 50;
export const LAUNCH_PREVIEW_SKIP_MIN = 0;
export const LAUNCH_PREVIEW_SKIP_MAX = 50;
export const LAUNCH_ATTENTION_MIN_TURN_MIN = 0;
export const LAUNCH_ATTENTION_MIN_TURN_MAX = 300;
export const LAUNCH_ATTENTION_DONE_DELAY_MIN = 0;
export const LAUNCH_ATTENTION_DONE_DELAY_MAX = 60;

export const defaultLaunchAttentionSettings: AttentionSettings = {
  mode: "all",
  programStatus: true,
  agentProtocol: true,
  notify: true,
  bell: true,
  minTurnSeconds: 0,
  doneDelaySeconds: 0,
};

export const LAUNCH_ATTENTION_MODE_OPTIONS: readonly {
  value: AttentionMode;
  label: string;
  description: string;
}[] = [
  {
    value: "all",
    get label() {
      return m.s2_launch_attn_mode_all();
    },
    get description() {
      return m.s2_launch_attn_mode_all_desc();
    },
  },
  {
    value: "urgent",
    get label() {
      return m.s2_launch_attn_mode_urgent();
    },
    get description() {
      return m.s2_launch_attn_mode_urgent_desc();
    },
  },
  {
    value: "off",
    get label() {
      return m.s2_launch_attn_mode_off();
    },
    get description() {
      return m.s2_launch_attn_mode_off_desc();
    },
  },
];
const envName = /^[A-Za-z_][A-Za-z0-9_]*$/;
const encoder = new TextEncoder();
const bytes = (value: string) => encoder.encode(value).length;

const STARTUP_SHELLS = ["bash", "zsh"] as const;
export type StartupShell = (typeof STARTUP_SHELLS)[number];
const STARTUP_PREFIXES: Record<StartupShell, readonly string[]> = {
  bash: ["bash", "-ic", 'exec -- "$@"', "webmux"],
  zsh: ["zsh", "-ic", 'exec -- "$@"', "webmux"],
};

export type EnvRow = { key: number; name: string; value: string };
export type LaunchConfigInput = {
  name: string;
  icon?: string;
  cmd: string[];
  env: Record<string, string>;
  preview: PreviewSettings | null;
  attention: AttentionSettings | null;
};
export type LaunchDraftErrors = {
  name?: string;
  command?: string;
  env?: string;
  preview?: string;
  attention?: string;
  rows: Record<number, { name?: string; value?: string }>;
};
/** Summaries as seen by the new-session sheet; values are never loaded. */
export type LaunchConfigsState =
  | { status: "loading" }
  | { status: "ready"; configs: LaunchConfigSummary[] }
  | { status: "unavailable"; error: string };

/** Restore only our exact wrapper; arbitrary user-written shell commands stay intact. */
export function launchCommandDraft(cmd: readonly string[]) {
  const startupShell = wrappedShell(cmd);
  return {
    command: joinArgs(
      startupShell ? cmd.slice(STARTUP_PREFIXES[startupShell].length) : cmd,
    ),
    startupShell,
  };
}

function wrappedShell(cmd: readonly string[]): StartupShell | null {
  for (const shell of STARTUP_SHELLS) {
    const prefix = STARTUP_PREFIXES[shell];
    if (
      cmd.length > prefix.length &&
      prefix.every((arg, index) => cmd[index] === arg)
    )
      return shell;
  }
  return null;
}

export const LAUNCH_ICON_OPTIONS = [
  {
    value: "terminal",
    get label(): string {
      return m.s2_icon_terminal();
    },
  },
  { value: "claudecode", label: "Claude Code" },
  { value: "codex", label: "Codex" },
  { value: "antigravity", label: "Antigravity CLI" },
  {
    value: "geminicli",
    get label(): string {
      return m.s2_icon_gemini_legacy();
    },
  },
  { value: "cursor", label: "Cursor CLI" },
  { value: "githubcopilot", label: "GitHub Copilot CLI" },
  { value: "opencode", label: "OpenCode" },
  { value: "qwen", label: "Qwen Code" },
  { value: "kimi", label: "Kimi Code CLI" },
  { value: "codebuddy", label: "CodeBuddy Code" },
  { value: "pi", label: "Pi / omp" },
  { value: "qoder", label: "Qoder" },
  { value: "devin", label: "Devin CLI" },
] as const;

/** The one-line description of a saved launch config under its name. */
export function launchConfigSummary(config: {
  chat?: boolean;
  envNames: readonly string[];
  preview?: { lines: number; skip: number } | null;
  attention?: AttentionSettings | null;
}) {
  const attentionLabel = !config.attention
    ? ""
    : config.attention.mode === "urgent"
      ? m.s2_launch_attn_mode_urgent()
      : config.attention.mode === "off"
        ? m.s2_launch_summary_attn_off()
        : m.s2_launch_summary_attn_custom();
  return [
    config.chat ? m.s2_launch_summary_hybrid() : m.s2_icon_terminal(),
    config.envNames.length
      ? m.s2_launch_env_count({ count: String(config.envNames.length) })
      : "",
    config.preview
      ? m.s2_launch_summary_preview({ lines: String(config.preview.lines) })
      : "",
    attentionLabel,
  ]
    .filter(Boolean)
    .join(" · ");
}

/** Old configs get a display default without rewriting their saved commands. */
export function launchConfigIcon(config: {
  icon?: string;
  cmd: readonly string[];
}) {
  if (config.icon) return config.icon;
  const shell = wrappedShell(config.cmd);
  const executable = config.cmd[shell ? STARTUP_PREFIXES[shell].length : 0]
    ?.split(/[\\/]/)
    .pop()
    ?.toLowerCase()
    .replace(/\.exe$/, "");
  switch (executable) {
    case "claude":
      return "claudecode";
    case "codex":
      return "codex";
    case "agy":
      return "antigravity";
    case "gemini":
      return "geminicli";
    case "agent":
    case "cursor-agent":
      return "cursor";
    case "copilot":
      return "githubcopilot";
    case "opencode":
      return "opencode";
    case "qwen":
      return "qwen";
    case "kimi":
      return "kimi";
    case "codebuddy":
      return "codebuddy";
    case "omp":
    case "pi":
      return "pi";
    case "qoder":
    case "qodercn":
    case "qoderclicn":
    case "qoder-cn":
      return "qoder";
    case "devin":
      return "devin";
    default:
      return "terminal";
  }
}

/** Error for one env variable name, or "" when valid. */
function envNameError(name: string): string {
  if (!name) return m.s2_launch_err_env_name_empty();
  if (!envName.test(name)) return m.s2_launch_err_env_name_invalid();
  if (name === "WEBMUX_SESSION_ID") return m.s2_launch_err_env_name_reserved();
  return "";
}

/**
 * Validates the editor fields with the server's limits. Rows whose name and
 * value are both empty are ignored so an added-but-unused row never blocks.
 */
export function validateLaunchDraft(
  name: string,
  command: string,
  rows: readonly EnvRow[],
  startupShell: StartupShell | null = null,
  preview: PreviewSettings | null = null,
  attention: AttentionSettings | null = null,
):
  | { ok: true; value: LaunchConfigInput; errors: LaunchDraftErrors }
  | { ok: false; errors: LaunchDraftErrors } {
  const errors: LaunchDraftErrors = { rows: {} };
  if (
    preview &&
    (!Number.isInteger(preview.lines) ||
      preview.lines < LAUNCH_PREVIEW_LINES_MIN ||
      preview.lines > LAUNCH_PREVIEW_LINES_MAX)
  )
    errors.preview = m.s2_launch_err_preview_lines({
      min: String(LAUNCH_PREVIEW_LINES_MIN),
      max: String(LAUNCH_PREVIEW_LINES_MAX),
    });
  else if (
    preview &&
    (!Number.isInteger(preview.skip) ||
      preview.skip < LAUNCH_PREVIEW_SKIP_MIN ||
      preview.skip > LAUNCH_PREVIEW_SKIP_MAX)
  )
    errors.preview = m.s2_launch_err_preview_skip({
      min: String(LAUNCH_PREVIEW_SKIP_MIN),
      max: String(LAUNCH_PREVIEW_SKIP_MAX),
    });

  if (
    attention &&
    attention.mode !== "all" &&
    attention.mode !== "urgent" &&
    attention.mode !== "off"
  )
    errors.attention = m.s2_launch_err_attn_mode();
  else if (
    attention &&
    (!Number.isInteger(attention.minTurnSeconds) ||
      attention.minTurnSeconds < LAUNCH_ATTENTION_MIN_TURN_MIN ||
      attention.minTurnSeconds > LAUNCH_ATTENTION_MIN_TURN_MAX)
  )
    errors.attention = m.s2_launch_err_attn_min_turn({
      min: String(LAUNCH_ATTENTION_MIN_TURN_MIN),
      max: String(LAUNCH_ATTENTION_MIN_TURN_MAX),
    });
  else if (
    attention &&
    (!Number.isInteger(attention.doneDelaySeconds) ||
      attention.doneDelaySeconds < LAUNCH_ATTENTION_DONE_DELAY_MIN ||
      attention.doneDelaySeconds > LAUNCH_ATTENTION_DONE_DELAY_MAX)
  )
    errors.attention = m.s2_launch_err_attn_done_delay({
      min: String(LAUNCH_ATTENTION_DONE_DELAY_MIN),
      max: String(LAUNCH_ATTENTION_DONE_DELAY_MAX),
    });
  const trimmed = name.trim();
  if (!trimmed) errors.name = m.s2_launch_err_name_empty();
  else if (Array.from(trimmed).length > LAUNCH_NAME_MAX)
    errors.name = m.s2_launch_err_name_max({ max: String(LAUNCH_NAME_MAX) });

  let cmd: string[] = [];
  try {
    cmd = parseArgs(command);
    if (!cmd.length || !cmd[0]) errors.command = m.s2_launch_err_cmd_empty();
    else {
      if (startupShell) cmd = [...STARTUP_PREFIXES[startupShell], ...cmd];
      if (cmd.length > LAUNCH_ARGS_MAX)
        errors.command = startupShell
          ? m.s2_launch_err_cmd_max_startup({ max: String(LAUNCH_ARGS_MAX) })
          : m.s2_launch_err_cmd_max({ max: String(LAUNCH_ARGS_MAX) });
      else if (cmd.some((arg) => bytes(arg) > LAUNCH_VALUE_BYTES))
        errors.command = m.s2_launch_err_arg_bytes({
          max: String(LAUNCH_VALUE_BYTES),
        });
    }
  } catch (err) {
    errors.command = errorMessage(err);
  }

  const env: Record<string, string> = {};
  const used = rows.filter((row) => row.name || row.value);
  if (used.length > LAUNCH_ENV_MAX)
    errors.env = m.s2_launch_err_env_max({ max: String(LAUNCH_ENV_MAX) });
  for (const row of used) {
    const rowErrors: { name?: string; value?: string } = {};
    const nameError = envNameError(row.name);
    if (nameError) rowErrors.name = nameError;
    else if (Object.hasOwn(env, row.name))
      rowErrors.name = m.s2_env_err_duplicate();
    if (row.value.includes("\0")) rowErrors.value = m.s2_launch_err_env_nul();
    else if (bytes(row.value) > LAUNCH_VALUE_BYTES)
      rowErrors.value = m.s2_launch_err_env_bytes({
        max: String(LAUNCH_VALUE_BYTES),
      });
    if (rowErrors.name || rowErrors.value) errors.rows[row.key] = rowErrors;
    else env[row.name] = row.value;
  }

  if (
    errors.name ||
    errors.command ||
    errors.env ||
    errors.preview ||
    errors.attention ||
    Object.keys(errors.rows).length
  )
    return { ok: false, errors };
  return {
    ok: true,
    value: { name: trimmed, cmd, env, preview, attention },
    errors,
  };
}
