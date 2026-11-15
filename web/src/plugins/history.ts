import { pluginEnabled } from "../platform/plugin-host.svelte";
import type { SessionView } from "../contracts/schemas";

// Workspace views may show archive projections, but only chat owns archive I/O.
export async function removeRecord(id: string) {
  if (!pluginEnabled("chat")) throw new Error("聊天功能未启用");
  const history = await import("../domains/sessions/recoverable");
  return history.removeRecord(id);
}
export async function resumeSession(id: string, view: SessionView) {
  if (!pluginEnabled("chat")) throw new Error("聊天功能未启用");
  const history = await import("../domains/sessions/recoverable");
  return history.resumeSession(id, view);
}
