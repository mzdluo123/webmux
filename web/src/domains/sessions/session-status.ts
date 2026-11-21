import { m } from "$lib/paraglide/messages.js";
import type { SessionInfo } from "../../contracts/schemas";

export type SessionState = SessionInfo["status"];

const urgentAttentionKinds = new Set<
  NonNullable<SessionInfo["attention"]>["kind"]
>(["question", "permission", "error"]);

export function hasUrgentAttention(
  session: Pick<SessionInfo, "attention">,
): boolean {
  return (
    session.attention !== null &&
    urgentAttentionKinds.has(session.attention.kind)
  );
}

export type SessionStatusTone =
  | "destructive"
  | "warning"
  | "success"
  | "secondary";

export function sessionStatusTone(
  session: Pick<SessionInfo, "status" | "exitCode" | "attention">,
): SessionStatusTone {
  if (session.status === "exited") {
    return session.exitCode !== null && session.exitCode !== 0
      ? "destructive"
      : "secondary";
  }
  if (hasUrgentAttention(session)) return "destructive";
  if (session.status === "waiting") return "warning";
  if (session.status === "running") return "success";
  return "secondary";
}

export interface SessionCounts {
  // "Running" in list summaries covers both busy and quiet live sessions.
  active: number;
  waiting: number;
  exited: number;
}

export function sessionRelativeTime(now: number, timestamp: string): string {
  const seconds = Math.max(0, Math.floor((now - Date.parse(timestamp)) / 1000));
  if (seconds < 60) return m.s3_rel_just_now();
  if (seconds < 3600)
    return m.s3_rel_minutes_ago({ count: String(Math.floor(seconds / 60)) });
  return m.s3_rel_hours_ago({ count: String(Math.floor(seconds / 3600)) });
}

export function isActive(status: SessionState) {
  return status === "running" || status === "idle";
}

// "archived" keeps only archived hybrid records, so no live session matches.
export type StatusFilter = "all" | "active" | "waiting" | "exited" | "archived";

export function matchesStatusFilter(
  status: SessionState,
  filter: StatusFilter,
) {
  if (filter === "all") return true;
  if (filter === "archived") return false;
  return filter === "active" ? isActive(status) : status === filter;
}

export function countSessions(
  sessions: readonly Pick<SessionInfo, "status">[],
): SessionCounts {
  const counts: SessionCounts = { active: 0, waiting: 0, exited: 0 };
  for (const { status } of sessions) {
    if (isActive(status)) counts.active++;
    else if (status === "waiting") counts.waiting++;
    else if (status === "exited") counts.exited++;
  }
  return counts;
}

export function statusSummary(
  sessions: readonly Pick<SessionInfo, "status">[],
) {
  const { active, waiting, exited } = countSessions(sessions);
  return m.s3_status_summary({ active, waiting, exited });
}

// Sidebar rows pair each mark with a localized label.
export type RowStatus =
  | "working"
  | "waiting"
  | "idle"
  | "dormant"
  | "ended"
  | "released";
export const rowStatusLabels: Record<RowStatus, string> = {
  get working() {
    return m.s3_row_status_working();
  },
  get waiting() {
    return m.s3_row_status_waiting();
  },
  get idle() {
    return m.s3_row_status_idle();
  },
  get dormant() {
    return m.sessions_resume_on_open();
  },
  get ended() {
    return m.s3_row_status_ended();
  },
  get released() {
    return m.s3_row_status_released();
  },
};
export function rowStatus(
  session: Pick<SessionInfo, "status" | "attention" | "agent">,
): RowStatus {
  if (session.status === "exited") return "ended";
  const kind = session.attention?.kind;
  if (
    kind === "question" ||
    kind === "permission" ||
    session.status === "waiting"
  )
    return "waiting";
  if (session.agent?.work || session.status === "running") return "working";
  return "idle";
}

/** The accessible name of a live session's sidebar button. */
export function sessionRowLabel(session: SessionInfo): string {
  return m.s3_session_row_label({
    title: session.title,
    status: rowStatusLabels[rowStatus(session)],
  });
}
/** The tooltip of a live session's sidebar button. */
export function sessionRowTitle(session: SessionInfo): string {
  return m.s3_session_row_title({
    title: session.title,
    agent: session.agent?.title ?? "",
    status: rowStatusLabels[rowStatus(session)],
  });
}
