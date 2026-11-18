import { z } from "zod";
import { store, stored } from "../../platform/api";
import type { CodeWorkspaceSnapshot } from "../../contracts/schemas";

// The open panel and its documents belong to the session. The old device
// split size supplies the default until a workspace records its own layout.
const sizeKey = "codePanelSize";
const SizeSchema = z.coerce.number().min(25).max(75);
const snapshots = new Map<string, CodeWorkspaceSnapshot>();

export const codePanelState = {
  wasOpened: (session: string) => stored(`dock.code.${session}`) === "1",
  markOpened(session: string) {
    store(`dock.code.${session}`, "1");
  },
  /** The code pane's share of the horizontal split, in percent. */
  size: () => SizeSchema.safeParse(stored(sizeKey, "x")).data ?? 50,
  /** Open tabs, navigation and reading positions a session left behind. */
  snapshot: (session: string) => snapshots.get(session),
  keep(session: string, snapshot: CodeWorkspaceSnapshot) {
    snapshots.set(session, snapshot);
  },
};
