import {
  featureEnabled,
  featureAvailable,
} from "../platform/feature-host.svelte";
import type * as Commands from "./terminal-quick-commands";
import type * as Voice from "../components/shared/VoiceButton.svelte";
import { errorMessage } from "../platform/error";
export interface QuickStartFeatures {
  quickCommands?: typeof Commands;
  voice?: typeof Voice;
  errors: string[];
}
export async function loadQuickStartFeatures(): Promise<QuickStartFeatures> {
  const result: QuickStartFeatures = { errors: [] };
  await Promise.all([
    (async () => {
      if (featureEnabled("quick-commands")) {
        try {
          result.quickCommands = await import("./terminal-quick-commands");
        } catch (error) {
          result.errors.push(`quick-commands：${errorMessage(error)}`);
        }
      }
    })(),
    (async () => {
      if (featureEnabled("voice") && featureAvailable("voice.recognize")) {
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
