import { api } from "../../platform/api";
import { ChatMessageMatchesSchema } from "../../contracts/schemas";

export function search(query: string, signal: AbortSignal) {
  return api(
    `/chat/search?q=${encodeURIComponent(query)}`,
    ChatMessageMatchesSchema,
    "GET",
    undefined,
    signal,
  );
}
export { createArchived } from "../../hooks/sessions/archived.svelte";
export { default as History } from "../../components/chat/ChatHistory.svelte";
export { recordTitle } from "../../domains/sessions/archived";
export { connectorMissingText } from "../../domains/sessions/hybrid";
export {
  removeRecord,
  resumeSession,
} from "../../domains/sessions/recoverable";
export const rebuild = () =>
  import("../../components/workspace/RebuildWorktreeDialog.svelte");
export const pages = {
  "quick-start": () => import("../../pages/QuickStart.svelte"),
  handoff: () => import("../../pages/Handoff.svelte"),
};
export const navigation = [{ id: "chat.new", label: "新聊天", href: "/new" }];
