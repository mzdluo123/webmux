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
export { recordTitle } from "../../domains/sessions/archived";
export { connectorMissingText } from "../../domains/sessions/hybrid";
export {
  removeRecord,
  resumeSession,
} from "../../domains/sessions/recoverable";
export const rebuild = () =>
  import("../../components/workspace/RebuildWorktreeDialog.svelte");
export const pages = {
  handoff: () => import("../../pages/Handoff.svelte"),
};
