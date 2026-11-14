import { pluginEnabled, pluginAvailable } from "../platform/plugin-host.svelte";
import type * as Commands from "./terminal-quick-commands";
import type * as Voice from "../components/shared/VoiceButton.svelte";
import { errorMessage } from "../platform/error";
export interface QuickStartPlugins {
  quickCommands?: typeof Commands;
  voice?: typeof Voice;
  errors: string[];
}
export async function loadQuickStartPlugins(): Promise<QuickStartPlugins> {
  const result: QuickStartPlugins = { errors: [] };
  await Promise.all([
    (async () => {
      if (pluginEnabled("quick-commands")) {
        try {
          result.quickCommands = await import("./terminal-quick-commands");
        } catch (error) {
          result.errors.push(`quick-commands：${errorMessage(error)}`);
        }
      }
    })(),
    (async () => {
      if (pluginEnabled("voice") && pluginAvailable("voice.recognize")) {
        try {
          result.voice =
            await import("../components/shared/VoiceButton.svelte");
        } catch (error) {
          result.errors.push(`voice：${errorMessage(error)}`);
        }
      }
    })(),
  ]);
  return result;
}
