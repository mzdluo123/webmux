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

export function sessionsByPin(
  sessions: readonly SessionInfo[],
  pinned: readonly SessionInfo[],
): SessionInfo[] {
  const pinRank = new Map(pinned.map((session, index) => [session.id, index]));
  return [...sessions].sort(
    (a, b) =>
      (pinRank.get(a.id) ?? pinned.length) -
        (pinRank.get(b.id) ?? pinned.length) ||
      Date.parse(b.createdAt) - Date.parse(a.createdAt) ||
      a.id.localeCompare(b.id),
  );
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
  for (const session of sessionsByPin(sessions, pinned)) {
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
  const location = queueWorkspaceLocation(session, repository).text;
  return repository ? `${repository.name} / ${location}` : location;
}

export function queueWorkspaceLocation(
  session: SessionInfo,
  repository: Repository | undefined,
): { text: string; branch: boolean } {
  if (!repository) return { text: session.cwd, branch: false };
  const tree = repository.worktrees.find(
    (item) => item.id === session.workspace?.worktreeId,
  );
  if (tree) return { text: worktreeTitle(tree), branch: Boolean(tree.branch) };
  if (session.released?.branch) {
    return { text: session.released.branch, branch: true };
  }
  return {
    text:
      session.released?.branch ??
      session.workspace?.worktreePath ??
      session.cwd,
    branch: false,
  };
}
