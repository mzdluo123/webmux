import { pluginAvailable, pluginEnabled } from "../platform/plugin-host.svelte";
import { errorMessage } from "../platform/error";
import type { PluginID } from "../contracts/plugin-schemas";
import type * as ChatContribution from "./terminal-chat";
import type * as FilesContribution from "./terminal-files";
import type * as CodeContribution from "./terminal-code";
import type * as QuickContribution from "./terminal-quick-commands";
import type * as VoiceContribution from "./terminal-voice";
import type * as ImmersiveContribution from "./terminal-immersive";
import type * as ForwardContribution from "./terminal-forward";
import type * as WorkspaceContribution from "./terminal-workspace";
import type * as GitContribution from "./terminal-git";

export interface TerminalPlugins {
  chat?: typeof ChatContribution;
  files?: typeof FilesContribution;
  code?: typeof CodeContribution;
  quickCommands?: typeof QuickContribution;
  voice?: typeof VoiceContribution;
  immersive?: typeof ImmersiveContribution;
  forward?: typeof ForwardContribution;
  workspace?: typeof WorkspaceContribution;
  git?: typeof GitContribution;
  errors?: string[];
}

/** Availability may recover without replacing the terminal's selected owners. */
export function availableTerminalPlugins(
  selected: TerminalPlugins,
): TerminalPlugins {
  const files = pluginAvailable("files.upload") ? selected.files : undefined;
  const code = pluginAvailable("code.browse") ? selected.code : undefined;
  const workspace = pluginAvailable("workspace.manage")
    ? selected.workspace
    : undefined;
  return {
    chat: files && pluginAvailable("chat.acp") ? selected.chat : undefined,
    files,
    code,
    quickCommands: pluginAvailable("quick-commands.send")
      ? selected.quickCommands
      : undefined,
    voice: pluginAvailable("voice.recognize") ? selected.voice : undefined,
    immersive: pluginAvailable("immersive.xr") ? selected.immersive : undefined,
    forward: pluginAvailable("forward.ports") ? selected.forward : undefined,
    workspace,
    git:
      workspace && code && pluginAvailable("workspace.git-workbench")
        ? selected.git
        : undefined,
    errors: selected.errors,
  };
}

/** Disabled implementations never load; an optional chunk failure leaves PTY usable. */
export async function loadTerminalPlugins(): Promise<TerminalPlugins> {
  const errors: string[] = [];
  async function load<T>(id: PluginID, implementation: () => Promise<T>) {
    if (!pluginEnabled(id)) return undefined;
    try {
      return await implementation();
    } catch (cause) {
      errors.push(`${id}：${errorMessage(cause)}`);
      return undefined;
    }
  }
  const [
    chat,
    files,
    code,
    quickCommands,
    voice,
    immersive,
    forward,
    workspace,
    git,
  ] = await Promise.all([
    load("chat", () => import("./terminal-chat")),
    load("files", () => import("./terminal-files")),
    load("code", () => import("./terminal-code")),
    load("quick-commands", () => import("./terminal-quick-commands")),
    load("voice", () => import("./terminal-voice")),
    load("immersive", () => import("./terminal-immersive")),
    load("forward", () => import("./terminal-forward")),
    load("workspace", () => import("./terminal-workspace")),
    pluginEnabled("code")
      ? load("workspace", () => import("./terminal-git"))
      : undefined,
  ]);
  return {
    chat: files ? chat : undefined,
    files,
    code,
    quickCommands,
    voice,
    immersive,
    forward,
    workspace,
    git: code ? git : undefined,
    errors,
  };
}
