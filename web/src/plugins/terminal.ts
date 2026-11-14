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

/** Fixed local contributions; a failed optional chunk never blocks the PTY. */
export async function loadTerminalPlugins(): Promise<TerminalPlugins> {
  const errors: string[] = [];
  async function load<T>(
    id: PluginID,
    capability: string,
    implementation: () => Promise<T>,
  ) {
    if (!pluginEnabled(id) || !pluginAvailable(capability)) return undefined;
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
    load("chat", "chat.acp", () => import("./terminal-chat")),
    load("files", "files.upload", () => import("./terminal-files")),
    load("code", "code.browse", () => import("./terminal-code")),
    load(
      "quick-commands",
      "quick-commands.send",
      () => import("./terminal-quick-commands"),
    ),
    load("voice", "voice.recognize", () => import("./terminal-voice")),
    load("immersive", "immersive.xr", () => import("./terminal-immersive")),
    load("forward", "forward.ports", () => import("./terminal-forward")),
    load("workspace", "workspace.manage", () => import("./terminal-workspace")),
    pluginEnabled("code")
      ? load(
          "workspace",
          "workspace.git-workbench",
          () => import("./terminal-git"),
        )
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
