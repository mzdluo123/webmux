import { m } from "$lib/paraglide/messages.js";
import { featureEnabled } from "../platform/feature-host.svelte";
import type { SessionView } from "../contracts/schemas";

// Workspace views may show archive projections, but only chat owns archive I/O.
export async function removeRecord(id: string) {
  if (!featureEnabled("chat")) throw new Error(m.s1_chat_not_enabled());
  const history = await import("../domains/sessions/recoverable");
  return history.removeRecord(id);
}
export async function resumeSession(id: string, view: SessionView) {
  if (!featureEnabled("chat")) throw new Error(m.s1_chat_not_enabled());
  const history = await import("../domains/sessions/recoverable");
  return history.resumeSession(id, view);
}
