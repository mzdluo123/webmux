import { sync } from "../../platform/push";
import { pluginAvailable } from "../../platform/plugin-host.svelte";
import {
  observeAttentionSnapshot,
  observeAttentionRemoved,
  observeAttentionSession,
  resetAttentionToasts,
} from "../../domains/notifications/attention-toast.svelte";
import type { SessionEvent } from "../../hooks/sessions/session-feed";
export { default as Toasts } from "../../components/notifications/AttentionToasts.svelte";
export { default as Inbox } from "../../components/notifications/AttentionInbox.svelte";

export function observe(event: SessionEvent) {
  if (event.type === "snapshot") observeAttentionSnapshot(event.sessions);
  else if (event.type === "removed") observeAttentionRemoved(event.id);
  else if (event.type !== "forwards") observeAttentionSession(event.session);
}
export function start(context: { forwardAuth: boolean; codePopout: boolean }) {
  const controller = new AbortController();
  if (
    !context.forwardAuth &&
    !context.codePopout &&
    pluginAvailable("notifications.push")
  )
    void sync(controller.signal).catch(() => undefined);
  return () => {
    controller.abort();
    resetAttentionToasts();
  };
}
