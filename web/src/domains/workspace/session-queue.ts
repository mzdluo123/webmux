import type { Repository, SessionInfo } from "../../contracts/schemas";
import { hasUrgentAttention } from "../sessions/session-status";
import { worktreeTitle } from "./home-worktrees";

export type QueueGroup = "attention" | "running" | "idle" | "exited";
export type FocusedQueueRow = { id: string; group: QueueGroup };

export function queueGroup(session: SessionInfo): QueueGroup {
  if (session.status === "exited") return "exited";
  if (hasUrgentAttention(session) || session.status === "waiting")
    return "attention";
  return session.agent?.work || session.status === "running"
    ? "running"
    : "idle";
}

export function sessionQueue(
  sessions: readonly SessionInfo[],
  pinned: readonly SessionInfo[],
  held: readonly (FocusedQueueRow | null)[],
): Record<QueueGroup, SessionInfo[]> {
  const groups: Record<QueueGroup, SessionInfo[]> = {
    attention: [],
    running: [],
    idle: [],
    exited: [],
  };
  const pinRank = new Map(pinned.map((session, index) => [session.id, index]));
  const sorted = [...sessions].sort(
    (a, b) =>
      (pinRank.get(a.id) ?? pinned.length) -
        (pinRank.get(b.id) ?? pinned.length) ||
      Date.parse(b.createdAt) - Date.parse(a.createdAt) ||
      a.id.localeCompare(b.id),
  );
  for (const session of sorted) {
    groups[
      held.find((row) => row?.id === session.id)?.group ?? queueGroup(session)
    ].push(session);
  }
  return groups;
}

export function queueLocation(
  session: SessionInfo,
  repositories: readonly Repository[],
): string {
  const repository = repositories.find(
    (item) => item.id === session.workspace?.repositoryId,
  );
  if (!repository) return session.cwd;
  const tree = repository.worktrees.find(
    (item) => item.id === session.workspace?.worktreeId,
  );
  if (tree) return `${repository.name} / ${worktreeTitle(tree)}`;
  return `${repository.name} / ${session.released?.branch ?? session.workspace?.worktreePath ?? session.cwd}`;
}
